package rdt

import (
	"errors"
	"fmt"
	"strconv"

	"bancada/internal/fsm"
	"bancada/internal/host"
	"bancada/internal/sim"
	"bancada/internal/wire"
	"bancada/internal/workload"
)

const (
	SenderMachine   = "sender"
	ReceiverMachine = "receiver"
)

const (
	EventSend    fsm.Event = "rdt_send"
	EventReceive fsm.Event = "rdt_rcv"
	EventTimeout fsm.Event = "timeout"
)

var ErrInvalidConfig = errors.New("configuração rdt inválida")

type Config struct {
	Timeout sim.VirtualTime `json:"timeout"`
	Gap     sim.VirtualTime `json:"gap"`
	Size    int             `json:"size"`
}

func DefaultConfig() Config {
	return Config{Timeout: 250 * sim.Millisecond, Gap: 2 * sim.Millisecond, Size: 64}
}

func (c Config) Validate() error {
	if c.Timeout <= 0 || c.Gap < 0 || c.Size <= workload.HeaderLen {
		return fmt.Errorf("%w: %+v", ErrInvalidConfig, c)
	}
	return nil
}

type Agent struct {
	stack     *Stack
	data      []byte
	message   int
	in        wire.RDTPacket
	intact    bool
	sndpkt    wire.RDTPacket
	label     string
	pktMsg    int
	sentOnce  bool
	extracted []byte
	timer     sim.TimerID
}

var packetShapes = map[string]struct {
	kind  wire.RDTKind
	seq   uint8
	label string
}{
	"0":    {wire.RDTData, 0, "pkt0"},
	"1":    {wire.RDTData, 1, "pkt1"},
	"ACK":  {wire.RDTAck, 0, "ACK"},
	"NAK":  {wire.RDTNak, 0, "NAK"},
	"ACK0": {wire.RDTAck, 0, "ACK0"},
	"ACK1": {wire.RDTAck, 1, "ACK1"},
}

func Vocabulary() *fsm.Registry[*Agent] {
	is := func(kind wire.RDTKind) fsm.GuardFunc[*Agent] {
		return func(a *Agent) bool { return a.in.Kind == kind }
	}
	seq := func(kind wire.RDTKind, n uint8) fsm.GuardFunc[*Agent] {
		return func(a *Agent) bool { return a.in.Kind == kind && a.in.Seq == n }
	}
	return fsm.NewRegistry[*Agent]().
		Guard("corrupt", func(a *Agent) bool { return !a.intact }).
		Guard("notcorrupt", func(a *Agent) bool { return a.intact }).
		Guard("isACK", is(wire.RDTAck)).
		Guard("isNAK", is(wire.RDTNak)).
		Guard("isACK0", seq(wire.RDTAck, 0)).
		Guard("isACK1", seq(wire.RDTAck, 1)).
		Guard("has_seq0", seq(wire.RDTData, 0)).
		Guard("has_seq1", seq(wire.RDTData, 1)).
		Action("make_pkt", func(a *Agent, shape string) error {
			s := packetShapes[shape]
			a.sndpkt = wire.RDTPacket{Kind: s.kind, Seq: s.seq}
			a.pktMsg = 0
			if s.kind == wire.RDTData {
				a.sndpkt.Payload = a.data
				a.pktMsg = a.message
			}
			a.label = s.label
			a.sentOnce = false
			return nil
		}, "0", "1", "ACK", "NAK", "ACK0", "ACK1").
		Effect("udt_send", (*Agent).send).
		Effect("extract", func(a *Agent) { a.extracted = a.in.Payload }).
		Effect("deliver_data", func(a *Agent) {
			if id, _, ok := workload.Parse(a.extracted); ok {
				a.stack.delivered++
				host.Deliver(a.stack.env, a.stack.sink, id)
			}
		}).
		Effect("start_timer", func(a *Agent) {
			a.stopTimer()
			a.timer = a.stack.env.Arm("rdt", a.stack.cfg.Timeout, func() {
				a.timer = 0
				a.stack.dispatch(EventTimeout)
			})
		}).
		Effect("stop_timer", (*Agent).stopTimer)
}

func (a *Agent) stopTimer() {
	if a.timer != 0 {
		a.stack.env.Disarm(a.timer)
		a.timer = 0
	}
}

func (a *Agent) send() {
	env := a.stack.env
	bytes := wire.EncodeRDT(a.sndpkt)
	label := a.label
	if a.pktMsg != 0 {
		label = fmt.Sprintf("%s #%d", label, a.pktMsg)
	}
	retransmission := a.sentOnce
	a.sentOnce = true
	a.stack.sent++
	if retransmission {
		a.stack.retransmissions++
	}
	env.Transmit(wire.Frame{Src: env.Addr(), Dst: env.PeerAddr(), Proto: wire.ProtoRDT, Bytes: bytes}, sim.Header{
		Proto:          "rdt",
		Label:          label,
		Flags:          a.sndpkt.Kind.String(),
		Seq:            uint32(a.sndpkt.Seq),
		RelSeq:         uint32(a.sndpkt.Seq),
		Length:         len(bytes),
		Payload:        len(a.sndpkt.Payload),
		Checksum:       uint16(bytes[2])<<8 | uint16(bytes[3]),
		Message:        a.pktMsg,
		Retransmission: retransmission,
		Reason:         map[bool]string{true: "reenvio"}[retransmission],
	})
}

type Programs struct {
	Sender   *fsm.Program[*Agent]
	Receiver *fsm.Program[*Agent]
}

func Compile(cat *fsm.Catalog, protocol string) (Programs, error) {
	file, err := cat.File(protocol)
	if err != nil {
		return Programs{}, err
	}
	senderDef, senderErr := file.Machine(SenderMachine)
	receiverDef, receiverErr := file.Machine(ReceiverMachine)
	if err := errors.Join(senderErr, receiverErr); err != nil {
		return Programs{}, err
	}
	vocabulary := Vocabulary()
	sender, senderErr := fsm.Compile(protocol, senderDef, vocabulary)
	receiver, receiverErr := fsm.Compile(protocol, receiverDef, vocabulary)
	if err := errors.Join(senderErr, receiverErr); err != nil {
		return Programs{}, err
	}
	return Programs{Sender: sender, Receiver: receiver}, nil
}

type Stack struct {
	env             host.Env
	cfg             Config
	machine         *fsm.Machine[*Agent]
	agent           *Agent
	source          workload.Source
	sink            *workload.Sink
	pending         int
	pumping         bool
	sent            int
	retransmissions int
	delivered       int
}

func New(env host.Env, cfg Config, programs Programs) (*Stack, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	s := &Stack{env: env, cfg: cfg, sink: workload.NewSink()}
	s.agent = &Agent{stack: s}
	program := programs.Sender
	if env.Role() == host.RoleServer {
		program = programs.Receiver
	}
	machine, err := program.Start(s.agent, host.Observer(env, program.Protocol()))
	if err != nil {
		return nil, err
	}
	s.machine = machine
	return s, nil
}

func (s *Stack) dispatch(event fsm.Event) {
	if _, err := s.machine.Fire(s.agent, event); err != nil {
		host.Reject(s.env, s.machine.Protocol(), err)
	}
	s.pump()
}

func (s *Stack) Receive(frame wire.Frame) {
	s.agent.in, s.agent.intact = wire.DecodeRDT(frame.Bytes)
	s.dispatch(EventReceive)
}

func (s *Stack) Do(cmd host.Command) error {
	if cmd.Verb != host.VerbSend || s.machine.Name() != SenderMachine {
		return fmt.Errorf("%w: %s em %s", host.ErrUnsupportedVerb, cmd.Verb, s.machine.Name())
	}
	if cmd.Size > 0 {
		s.cfg.Size = cmd.Size
	}
	s.pending += max(cmd.Count, 1)
	s.pump()
	return nil
}

func (s *Stack) pump() {
	if s.pumping || s.pending == 0 || !s.machine.Accepts(s.agent, EventSend) {
		return
	}
	s.pumping = true
	s.env.Defer("app: rdt_send", s.cfg.Gap, func() {
		s.pumping = false
		if s.pending == 0 || !s.machine.Accepts(s.agent, EventSend) {
			return
		}
		s.pending--
		id, payload := s.source.Next(s.cfg.Size)
		host.Offer(s.env, id)
		s.agent.data, s.agent.message = payload, id
		s.dispatch(EventSend)
	})
}

func (s *Stack) Drained() bool {
	return s.source.Offered() > 0 && s.pending == 0 && s.machine.Accepts(s.agent, EventSend)
}

func (s *Stack) View() host.View {
	tally := s.sink.Tally()
	tally.Offered = s.source.Offered()
	itoa := strconv.Itoa
	last := s.agent.label
	if last == "" {
		last = "—"
	}
	return host.View{
		Machines: []host.MachineView{host.ViewOf(s.machine)},
		Registers: []host.Register{
			{Group: "rdt", Name: "papel", Value: s.machine.Name()},
			{Group: "rdt", Name: "sndpkt", Value: last},
			{Group: "rdt", Name: "pendentes na app", Value: itoa(s.pending)},
			{Group: "pacotes", Name: "enviados", Value: itoa(s.sent)},
			{Group: "pacotes", Name: "reenvios", Value: itoa(s.retransmissions)},
			{Group: "pacotes", Name: "entregues", Value: itoa(s.delivered)},
		},
		App: tally,
	}
}
