package udp

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

const SocketMachine = "socket"

const (
	EventSend    fsm.Event = "send"
	EventReceive fsm.Event = "rcv"
)

var ErrInvalidConfig = errors.New("configuração UDP inválida")

type Config struct {
	ClientPort uint16          `json:"clientPort"`
	ServerPort uint16          `json:"serverPort"`
	Size       int             `json:"size"`
	Interval   sim.VirtualTime `json:"interval"`
}

func DefaultConfig() Config {
	return Config{ClientPort: 40000, ServerPort: 9000, Size: 200, Interval: 10 * sim.Millisecond}
}

func (c Config) Validate() error {
	if c.ClientPort == 0 || c.ServerPort == 0 || c.Size <= workload.HeaderLen || c.Interval < 0 {
		return fmt.Errorf("%w: %+v", ErrInvalidConfig, c)
	}
	return nil
}

type Socket struct {
	stack   *Stack
	out     []byte
	message int
	frame   wire.Frame
	header  sim.Header
	in      wire.Datagram
	inErr   error
}

func Vocabulary() *fsm.Registry[*Socket] {
	return fsm.NewRegistry[*Socket]().
		Guard("intact", func(s *Socket) bool { return s.inErr == nil }).
		Guard("bound", func(s *Socket) bool { return s.in.DstPort == s.stack.local }).
		Effect("make_datagram", (*Socket).makeDatagram).
		Effect("udt_send", func(s *Socket) {
			s.stack.sent++
			s.stack.env.Transmit(s.frame, s.header)
		}).
		Effect("deliver_data", func(s *Socket) {
			s.stack.received++
			if id, _, ok := workload.Parse(s.in.Payload); ok {
				host.Deliver(s.stack.env, s.stack.sink, id)
			}
		}).
		Action("discard", func(s *Socket, reason string) error {
			s.stack.discarded++
			text := fmt.Sprintf("porta %d sem socket", s.in.DstPort)
			if reason == "checksum" {
				text = s.inErr.Error()
			}
			host.Discard(s.stack.env, text+" (sem aviso ao remetente)")
			return nil
		}, "checksum", "port")
}

func (s *Socket) makeDatagram() {
	env := s.stack.env
	bytes := wire.EncodeUDP(env.Addr(), env.PeerAddr(), s.stack.local, s.stack.remote, s.out)
	s.frame = wire.Frame{Src: env.Addr(), Dst: env.PeerAddr(), Proto: wire.ProtoUDP, Bytes: bytes}
	checksum := uint16(bytes[6])<<8 | uint16(bytes[7])
	s.stack.lastChecksum = checksum
	s.header = sim.Header{
		Proto:    "udp",
		Label:    fmt.Sprintf("#%d ck=%04X", s.message, checksum),
		SrcPort:  s.stack.local,
		DstPort:  s.stack.remote,
		Length:   len(bytes),
		Payload:  len(s.out),
		Checksum: checksum,
		Message:  s.message,
	}
}

func Compile(cat *fsm.Catalog, protocol string) (*fsm.Program[*Socket], error) {
	file, err := cat.File(protocol)
	if err != nil {
		return nil, err
	}
	def, err := file.Machine(SocketMachine)
	if err != nil {
		return nil, err
	}
	return fsm.Compile(protocol, def, Vocabulary())
}

type Stack struct {
	env          host.Env
	cfg          Config
	machine      *fsm.Machine[*Socket]
	socket       *Socket
	local        uint16
	remote       uint16
	source       workload.Source
	sink         *workload.Sink
	pending      int
	sent         int
	received     int
	discarded    int
	lastChecksum uint16
}

func New(env host.Env, cfg Config, program *fsm.Program[*Socket]) (*Stack, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	s := &Stack{env: env, cfg: cfg, sink: workload.NewSink(), local: cfg.ClientPort, remote: cfg.ServerPort}
	if env.Role() == host.RoleServer {
		s.local, s.remote = cfg.ServerPort, cfg.ClientPort
	}
	s.socket = &Socket{stack: s}
	machine, err := program.Start(s.socket, host.Observer(env, program.Protocol()))
	if err != nil {
		return nil, err
	}
	s.machine = machine
	return s, nil
}

func (s *Stack) dispatch(event fsm.Event) error {
	_, err := s.machine.Fire(s.socket, event)
	if err != nil {
		host.Reject(s.env, s.machine.Protocol(), err)
	}
	return err
}

func (s *Stack) Receive(frame wire.Frame) {
	s.socket.in, s.socket.inErr = wire.DecodeUDP(frame.Src, frame.Dst, frame.Bytes)
	s.dispatch(EventReceive)
}

func (s *Stack) Do(cmd host.Command) error {
	if cmd.Verb != host.VerbSend {
		return fmt.Errorf("%w: UDP não tem %s (não há conexão)", host.ErrUnsupportedVerb, cmd.Verb)
	}
	size := cmd.Size
	if size <= 0 {
		size = s.cfg.Size
	}
	interval := cmd.Interval
	if interval <= 0 {
		interval = s.cfg.Interval
	}
	count := max(cmd.Count, 1)
	s.pending += count
	for i := range count {
		s.env.Defer("app: send", sim.VirtualTime(i)*interval, func() {
			s.pending--
			id, payload := s.source.Next(size)
			host.Offer(s.env, id)
			s.socket.out, s.socket.message = payload, id
			s.dispatch(EventSend)
		})
	}
	return nil
}

func (s *Stack) Drained() bool {
	return s.source.Offered() > 0 && s.pending == 0
}

func (s *Stack) View() host.View {
	tally := s.sink.Tally()
	tally.Offered = s.source.Offered()
	itoa := strconv.Itoa
	return host.View{
		Machines: []host.MachineView{host.ViewOf(s.machine)},
		Registers: []host.Register{
			{Group: "socket", Name: "portas", Value: fmt.Sprintf("%d → %d", s.local, s.remote)},
			{Group: "socket", Name: "conexão", Value: "nenhuma (sem handshake)"},
			{Group: "datagramas", Name: "enviados", Value: itoa(s.sent)},
			{Group: "datagramas", Name: "agendados", Value: itoa(s.pending)},
			{Group: "datagramas", Name: "recebidos", Value: itoa(s.received)},
			{Group: "datagramas", Name: "descartados", Value: itoa(s.discarded)},
			{Group: "checksum", Name: "último calculado", Value: fmt.Sprintf("0x%04X", s.lastChecksum)},
		},
		App: tally,
	}
}
