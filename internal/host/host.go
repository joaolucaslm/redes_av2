package host

import (
	"errors"
	"math/rand/v2"
	"net/netip"

	"bancada/internal/fsm"
	"bancada/internal/sim"
	"bancada/internal/wire"
	"bancada/internal/workload"
)

var (
	ErrUnsupportedVerb = errors.New("comando não suportado por esta pilha")
	ErrNoConnection    = errors.New("não há conexão aberta neste host")
)

type Role string

const (
	RoleClient Role = "cliente"
	RoleServer Role = "servidor"
)

type Env interface {
	Name() string
	Peer() string
	Role() Role
	Addr() netip.Addr
	PeerAddr() netip.Addr
	Now() sim.VirtualTime
	Transmit(frame wire.Frame, header sim.Header)
	Arm(timer string, after sim.VirtualTime, fire func()) sim.TimerID
	Disarm(id sim.TimerID) bool
	Deadline(id sim.TimerID) (sim.VirtualTime, bool)
	Defer(label string, after sim.VirtualTime, fire func())
	Publish(r sim.Record)
	Stream(name string) *rand.Rand
}

type Stack interface {
	Receive(frame wire.Frame)
	Do(cmd Command) error
	View() View
	Drained() bool
}

type Verb string

const (
	VerbListen  Verb = "listen"
	VerbConnect Verb = "connect"
	VerbSend    Verb = "send"
	VerbClose   Verb = "close"
)

type Command struct {
	Verb     Verb            `json:"verb"`
	Count    int             `json:"count,omitzero"`
	Size     int             `json:"size,omitzero"`
	Interval sim.VirtualTime `json:"interval,omitzero"`
	Port     uint16          `json:"port,omitzero"`
}

type MachineView struct {
	Protocol string    `json:"protocol"`
	Machine  string    `json:"machine"`
	State    string    `json:"state"`
	Last     fsm.Fired `json:"last"`
	HasLast  bool      `json:"hasLast"`
}

type Register struct {
	Group string `json:"group"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type TimerView struct {
	Name     string          `json:"name"`
	Deadline sim.VirtualTime `json:"deadline"`
}

type View struct {
	Name      string         `json:"name"`
	Addr      string         `json:"addr"`
	Role      Role           `json:"role"`
	Machines  []MachineView  `json:"machines"`
	Registers []Register     `json:"registers"`
	Timers    []TimerView    `json:"timers"`
	App       workload.Tally `json:"app"`
}

type Observed interface {
	State() fsm.State
	Last() (fsm.Fired, bool)
	Name() string
	Protocol() string
}

func ViewOf(m Observed) MachineView {
	last, ok := m.Last()
	return MachineView{Protocol: m.Protocol(), Machine: m.Name(), State: string(m.State()), Last: last, HasLast: ok}
}

func Observer(env Env, protocol string) fsm.Observer {
	return func(f fsm.Fired) {
		env.Publish(sim.Record{
			Kind:     sim.KindTransition,
			Protocol: protocol,
			Machine:  f.Machine,
			From:     string(f.From),
			Event:    string(f.Event),
			Guard:    f.Guard,
			Actions:  f.Actions,
			To:       string(f.To),
			Rule:     f.Rule,
			Edge:     f.Edge,
		})
	}
}

func Reject(env Env, protocol string, err error) {
	var rejection *fsm.RejectionError
	if errors.As(err, &rejection) {
		env.Publish(sim.Record{
			Kind:     sim.KindRejection,
			Protocol: protocol,
			Machine:  rejection.Machine,
			From:     string(rejection.State),
			Event:    string(rejection.Event),
			Reason:   rejection.Reason(),
		})
		return
	}
	env.Publish(sim.Record{Kind: sim.KindRejection, Protocol: protocol, Reason: err.Error()})
}

func Offer(env Env, id int) {
	env.Publish(sim.Record{Kind: sim.KindApp, Event: "offer", Message: id})
}

func Deliver(env Env, sink *workload.Sink, id int) workload.Verdict {
	verdict := sink.Accept(id)
	env.Publish(sim.Record{Kind: sim.KindDelivery, Message: id, Verdict: string(verdict)})
	return verdict
}

func Notify(env Env, event, detail string) {
	env.Publish(sim.Record{Kind: sim.KindApp, Event: event, Detail: detail})
}

func Discard(env Env, reason string) {
	env.Publish(sim.Record{Kind: sim.KindDiscard, Reason: reason})
}
