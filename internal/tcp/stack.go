package tcp

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"

	"bancada/internal/fsm"
	"bancada/internal/host"
	"bancada/internal/wire"
	"bancada/internal/workload"
)

const (
	ConnectionMachine = "connection"
	CongestionMachine = "congestion"
)

var ErrMissingCongestion = errors.New("nenhum arquivo em \"uses\" define a máquina congestion")

type Programs struct {
	Connection *fsm.Program[*Endpoint]
	Congestion *fsm.Program[*Endpoint]
}

func Compile(cat *fsm.Catalog, protocol string) (Programs, error) {
	file, err := cat.File(protocol)
	if err != nil {
		return Programs{}, err
	}
	connDef, err := file.Machine(ConnectionMachine)
	if err != nil {
		return Programs{}, err
	}
	var ccProtocol string
	var ccDef fsm.Definition
	for _, used := range file.Uses {
		dep, err := cat.File(used)
		if err != nil {
			return Programs{}, err
		}
		if def, err := dep.Machine(CongestionMachine); err == nil {
			ccProtocol, ccDef = used, def
			break
		}
	}
	if ccProtocol == "" {
		return Programs{}, fmt.Errorf("%s: %w", protocol, ErrMissingCongestion)
	}
	vocabulary := Vocabulary()
	conn, connErr := fsm.Compile(protocol, connDef, vocabulary)
	cc, ccErr := fsm.Compile(ccProtocol, ccDef, vocabulary)
	if err := errors.Join(connErr, ccErr); err != nil {
		return Programs{}, err
	}
	return Programs{Connection: conn, Congestion: cc}, nil
}

type Stack struct {
	env        host.Env
	cfg        Config
	programs   Programs
	isn        *rand.Rand
	endpoints  map[uint16]*Endpoint
	primary    *Endpoint
	closedPort *Endpoint
	idle       *Endpoint
	source     workload.Source
	sink       *workload.Sink
}

func New(env host.Env, cfg Config, programs Programs) (*Stack, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	s := &Stack{
		env:       env,
		cfg:       cfg,
		programs:  programs,
		isn:       env.Stream("tcp.isn"),
		endpoints: make(map[uint16]*Endpoint),
		sink:      workload.NewSink(),
	}
	idle, err := s.newEndpoint(0, 0)
	if err != nil {
		return nil, err
	}
	s.idle = idle
	return s, nil
}

func (s *Stack) Receive(frame wire.Frame) {
	seg, err := wire.DecodeTCP(frame.Src, frame.Dst, frame.Bytes)
	if err != nil {
		host.Discard(s.env, err.Error())
		return
	}
	ep, bound := s.endpoints[seg.DstPort]
	if !bound {
		ep, err = s.newEndpoint(seg.DstPort, seg.SrcPort)
		if err != nil {
			host.Reject(s.env, s.programs.Connection.Protocol(), err)
			return
		}
		ep.transient = true
		s.closedPort = ep
	}
	ep.in = seg
	ep.dispatch(Classify(seg))
}

func (s *Stack) Do(cmd host.Command) error {
	switch cmd.Verb {
	case host.VerbListen:
		port := orDefault(cmd.Port, s.cfg.ServerPort)
		ep, err := s.bind(port, 0)
		if err != nil {
			return err
		}
		return ep.dispatch(EventListen)
	case host.VerbConnect:
		ep, err := s.bind(s.cfg.ClientPort, orDefault(cmd.Port, s.cfg.ServerPort))
		if err != nil {
			return err
		}
		return ep.dispatch(EventConnect)
	case host.VerbSend:
		if s.primary == nil {
			return host.ErrNoConnection
		}
		size := cmd.Size
		if size <= 0 {
			size = s.cfg.MSS
		}
		var data []byte
		for range max(cmd.Count, 1) {
			id, payload := s.source.Next(size)
			host.Offer(s.env, id)
			data = append(data, payload...)
		}
		s.primary.write = data
		return s.primary.dispatch(EventSend)
	case host.VerbClose:
		if s.primary == nil {
			return host.ErrNoConnection
		}
		return s.primary.dispatch(EventClose)
	default:
		return fmt.Errorf("%w: %s", host.ErrUnsupportedVerb, cmd.Verb)
	}
}

func orDefault(value, fallback uint16) uint16 {
	if value != 0 {
		return value
	}
	return fallback
}

func (s *Stack) bind(local, remote uint16) (*Endpoint, error) {
	ep, err := s.newEndpoint(local, remote)
	if err != nil {
		return nil, err
	}
	if old, ok := s.endpoints[local]; ok {
		old.release()
	}
	s.endpoints[local] = ep
	s.primary = ep
	return ep, nil
}

func (s *Stack) Drained() bool {
	return s.primary != nil && s.source.Offered() > 0 && s.primary.drained()
}

func (s *Stack) shown() *Endpoint {
	switch {
	case s.primary != nil:
		return s.primary
	case s.closedPort != nil:
		return s.closedPort
	default:
		return s.idle
	}
}

func (s *Stack) View() host.View {
	ep := s.shown()
	tally := s.sink.Tally()
	tally.Offered = s.source.Offered()
	return host.View{
		Machines:  []host.MachineView{host.ViewOf(ep.conn), host.ViewOf(ep.cc)},
		Registers: ep.registers(),
		App:       tally,
	}
}

func (ep *Endpoint) registers() []host.Register {
	mss := ep.cfg().MSS
	itoa := strconv.Itoa
	rel := func(s, base Seq) string { return itoa(s.Minus(base)) }
	var regs []host.Register
	add := func(group, name, value string) {
		regs = append(regs, host.Register{Group: group, Name: name, Value: value})
	}
	add("conexão", "portas", fmt.Sprintf("%d ⇄ %d", ep.local, ep.remote))
	add("conexão", "ISN local", strconv.FormatUint(uint64(ep.iss), 10))
	irs := "—"
	if ep.synced {
		irs = strconv.FormatUint(uint64(ep.irs), 10)
	}
	add("conexão", "ISN remoto", irs)
	add("envio", "SendBase", rel(ep.sndUna, ep.iss))
	add("envio", "NextSeqNum", rel(ep.sndNxt, ep.iss))
	add("envio", "em voo", itoa(ep.inFlight())+" B")
	add("envio", "fila da app", itoa(len(ep.unsent))+" B")
	ack := "—"
	if ep.synced {
		ack = rel(ep.rcvNxt, ep.irs)
	}
	add("recepção", "rcv_nxt (ACK)", ack)
	add("recepção", "fora de ordem", itoa(len(ep.ooo))+" seg")
	add("recepção", "rwnd anunciada", itoa(int(ep.advertisedWindow())))
	add("recepção", "rwnd do par", itoa(ep.peerWindow))
	sample, estimated, deviation := "—", "—", "—"
	if ep.rtt.Samples() > 0 {
		sample, estimated, deviation = ep.rtt.LastSample().String(), ep.rtt.Estimated().String(), ep.rtt.Deviation().String()
	}
	add("rtt", "SampleRTT", sample)
	add("rtt", "EstimatedRTT", estimated)
	add("rtt", "DevRTT", deviation)
	add("rtt", "TimeoutInterval", ep.rtt.TimeoutInterval().String())
	add("congestionamento", "cwnd", fmt.Sprintf("%d B (%.1f MSS)", ep.cwnd, float64(ep.cwnd)/float64(mss)))
	add("congestionamento", "ssthresh", itoa(ep.ssthresh)+" B")
	add("congestionamento", "dupACKcount", itoa(ep.dupAcks))
	add("congestionamento", "janela efetiva", itoa(ep.sendWindow())+" B")
	return regs
}
