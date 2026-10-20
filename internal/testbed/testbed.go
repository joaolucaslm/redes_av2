package testbed

import (
	"errors"
	"fmt"
	"net/netip"

	"bancada/internal/channel"
	"bancada/internal/fsm"
	"bancada/internal/host"
	"bancada/internal/sim"
	"bancada/internal/wire"
)

var (
	AddrA = netip.MustParseAddr("10.0.0.1")
	AddrB = netip.MustParseAddr("10.0.0.2")
)

type CommandKind string

const (
	CommandApp      CommandKind = "app"
	CommandDropNext CommandKind = "drop-next"
	CommandChannel  CommandKind = "channel"
	CommandFault    CommandKind = "fault"
)

type Command struct {
	Kind    CommandKind    `json:"kind"`
	Host    string         `json:"host,omitzero"`
	App     host.Command   `json:"app,omitzero"`
	Channel channel.Params `json:"channel,omitzero"`
	Fault   channel.Rule   `json:"fault,omitzero"`
}

func AppCommand(hostName string, cmd host.Command) Command {
	return Command{Kind: CommandApp, Host: hostName, App: cmd}
}

func DropNextCommand(from string) Command {
	return Command{Kind: CommandDropNext, Host: from}
}

func ChannelCommand(p channel.Params) Command {
	return Command{Kind: CommandChannel, Channel: p}
}

func FaultCommand(rule channel.Rule) Command {
	return Command{Kind: CommandFault, Fault: rule}
}

func (c Command) String() string {
	switch c.Kind {
	case CommandApp:
		text := fmt.Sprintf("%s · app %s", c.Host, c.App.Verb)
		if c.App.Count > 0 {
			text += fmt.Sprintf(" ×%d", c.App.Count)
		}
		return text
	case CommandDropNext:
		if c.Host == "" {
			return "operador · descartar o próximo segmento"
		}
		return "operador · descartar o próximo segmento de " + c.Host
	case CommandChannel:
		return "operador · parâmetros do canal"
	case CommandFault:
		return fmt.Sprintf("operador · regra %s nº%d", c.Fault.Action, c.Fault.Nth)
	default:
		return string(c.Kind)
	}
}

type Trigger struct {
	Host    string `json:"host"`
	State   string `json:"state,omitzero"`
	Drained bool   `json:"drained,omitzero"`
}

type trigger struct {
	cond  Trigger
	after sim.VirtualTime
	cmd   Command
	spent bool
}

type ChannelView struct {
	Params    channel.Params `json:"params"`
	InTransit int            `json:"inTransit"`
	Armed     []string       `json:"armed"`
	Rules     int            `json:"rules"`
}

type Snapshot struct {
	Step     int             `json:"step"`
	Now      sim.VirtualTime `json:"now"`
	Protocol string          `json:"protocol"`
	Family   string          `json:"family"`
	Hosts    []host.View     `json:"hosts"`
	Channel  ChannelView     `json:"channel"`
	Stats    Stats           `json:"stats"`
	Pending  int             `json:"pending"`
	NextAt   sim.VirtualTime `json:"nextAt"`
	Idle     bool            `json:"idle"`
}

type Testbed struct {
	cfg       Config
	file      fsm.File
	sched     *sim.Scheduler
	bus       *sim.Bus
	streams   sim.Streams
	channel   *channel.Channel
	nodes     []*node
	step      int
	stats     Stats
	triggers  []*trigger
	inTransit int
}

func New(cat *fsm.Catalog, cfg Config) (*Testbed, error) {
	file, bp, err := compile(cat, cfg.Protocol)
	if err != nil {
		return nil, err
	}
	sched := sim.NewScheduler()
	streams := sim.NewStreams(cfg.Seed)
	ch, err := channel.New(cfg.Channel, streams)
	if err != nil {
		return nil, err
	}
	tb := &Testbed{cfg: cfg, file: file, sched: sched, bus: sim.NewBus(sched), streams: streams, channel: ch}
	tb.bus.Subscribe(tb.stats.Observe)
	tb.bus.Subscribe(tb.watch)
	tb.nodes = []*node{
		{tb: tb, name: "A", peer: "B", role: host.RoleClient, addr: AddrA, peerAddr: AddrB, timers: make(map[sim.TimerID]host.TimerView)},
		{tb: tb, name: "B", peer: "A", role: host.RoleServer, addr: AddrB, peerAddr: AddrA, timers: make(map[sim.TimerID]host.TimerView)},
	}
	var problems []error
	for _, n := range tb.nodes {
		n.stack, err = bp.build(n, cfg)
		problems = append(problems, err)
	}
	if err := errors.Join(problems...); err != nil {
		return nil, err
	}
	return tb, nil
}

func (tb *Testbed) Config() Config { return tb.cfg }

func (tb *Testbed) Protocol() string { return tb.cfg.Protocol }

func (tb *Testbed) Family() string { return tb.file.Family }

func (tb *Testbed) Now() sim.VirtualTime { return tb.sched.Now() }

func (tb *Testbed) StepIndex() int { return tb.step }

func (tb *Testbed) Peek() (sim.VirtualTime, bool) { return tb.sched.Peek() }

func (tb *Testbed) InTransit() int { return tb.inTransit }

func (tb *Testbed) Stats() Stats { return tb.stats }

func (tb *Testbed) Records() []sim.Record { return tb.bus.Records() }

func (tb *Testbed) RecordsSince(index int) []sim.Record { return tb.bus.Since(index) }

func (tb *Testbed) RecordCount() int { return tb.bus.Len() }

func (tb *Testbed) Step() (sim.Occurrence, bool) {
	occ, ok := tb.sched.Next()
	if !ok {
		return occ, false
	}
	tb.step++
	tb.bus.SetStep(tb.step)
	occ.Fire()
	tb.checkDrained()
	return occ, true
}

func (tb *Testbed) node(name string) (*node, error) {
	for _, n := range tb.nodes {
		if n.name == name {
			return n, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownHost, name)
}

func (tb *Testbed) transmit(from *node, frame wire.Frame, header sim.Header) {
	to, _ := tb.node(from.peer)
	packet := channel.Packet{Src: from.name, Dst: to.name, Frame: frame, Header: header}
	for _, transit := range tb.channel.Transmit(tb.sched.Now(), packet) {
		flight := transit.Flight
		tb.bus.Publish(sim.Record{Kind: sim.KindSend, Host: from.name, Flight: flight})
		tb.inTransit++
		if flight.Fate == sim.FateLost {
			tb.sched.At(flight.EndAt, fmt.Sprintf("perda #%d %s→%s", flight.ID, from.name, to.name), func() {
				tb.inTransit--
				tb.bus.Publish(sim.Record{Kind: sim.KindLoss, Host: from.name, Flight: flight, Reason: "perda " + flight.Cause})
			})
			continue
		}
		tb.sched.At(flight.EndAt, fmt.Sprintf("chegada #%d em %s", flight.ID, to.name), func() {
			tb.inTransit--
			tb.bus.Publish(sim.Record{Kind: sim.KindArrival, Host: to.name, Flight: flight})
			to.stack.Receive(transit.Frame)
		})
	}
}

func (tb *Testbed) Apply(cmd Command) error {
	switch cmd.Kind {
	case CommandApp:
		n, err := tb.node(cmd.Host)
		if err != nil {
			return err
		}
		n.Publish(sim.Record{Kind: sim.KindApp, Event: string(cmd.App.Verb), Detail: cmd.String()})
		err = n.stack.Do(cmd.App)
		var rejection *fsm.RejectionError
		if err != nil && !errors.As(err, &rejection) {
			n.Publish(sim.Record{Kind: sim.KindOperator, Event: string(cmd.App.Verb), Reason: err.Error()})
		}
		tb.checkDrained()
		return err
	case CommandDropNext:
		if cmd.Host != channel.AnySource {
			if _, err := tb.node(cmd.Host); err != nil {
				return err
			}
		}
		tb.channel.ArmDropNext(cmd.Host)
	case CommandChannel:
		if err := tb.channel.SetParams(cmd.Channel); err != nil {
			return err
		}
		tb.cfg.Channel = cmd.Channel
	case CommandFault:
		if err := tb.channel.AddRule(cmd.Fault); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w: %q", ErrUnknownKind, cmd.Kind)
	}
	tb.bus.Publish(sim.Record{Kind: sim.KindOperator, Host: cmd.Host, Event: string(cmd.Kind), Detail: cmd.String()})
	return nil
}

func (tb *Testbed) Schedule(at sim.VirtualTime, cmd Command) {
	tb.sched.At(at, cmd.String(), func() { _ = tb.Apply(cmd) })
}

func (tb *Testbed) When(cond Trigger, after sim.VirtualTime, cmd Command) error {
	if _, err := tb.node(cond.Host); err != nil {
		return err
	}
	tb.triggers = append(tb.triggers, &trigger{cond: cond, after: after, cmd: cmd})
	return nil
}

func (tb *Testbed) watch(r sim.Record) {
	if !r.ChangesState() {
		return
	}
	for _, t := range tb.triggers {
		if !t.spent && t.cond.State != "" && t.cond.Host == r.Host && t.cond.State == r.To {
			t.spent = true
			tb.Schedule(tb.sched.Now()+t.after, t.cmd)
		}
	}
}

func (tb *Testbed) checkDrained() {
	for _, t := range tb.triggers {
		if t.spent || !t.cond.Drained {
			continue
		}
		if n, err := tb.node(t.cond.Host); err == nil && n.stack.Drained() {
			t.spent = true
			tb.Schedule(tb.sched.Now()+t.after, t.cmd)
		}
	}
}

func (tb *Testbed) Snapshot() Snapshot {
	hosts := make([]host.View, len(tb.nodes))
	for i, n := range tb.nodes {
		view := n.stack.View()
		view.Name, view.Addr, view.Role = n.name, n.addr.String(), n.role
		view.Timers = n.timerViews()
		hosts[i] = view
	}
	armed := tb.channel.ArmedDrops()
	if armed == nil {
		armed = []string{}
	}
	next, pending := tb.sched.Peek()
	return Snapshot{
		Step:     tb.step,
		Now:      tb.sched.Now(),
		Protocol: tb.cfg.Protocol,
		Family:   tb.file.Family,
		Hosts:    hosts,
		Channel:  ChannelView{Params: tb.channel.Params(), InTransit: tb.inTransit, Armed: armed, Rules: tb.channel.PendingRules()},
		Stats:    tb.stats,
		Pending:  tb.sched.Pending(),
		NextAt:   next,
		Idle:     !pending,
	}
}
