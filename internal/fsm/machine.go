package fsm

import "slices"

type Fired struct {
	Machine string   `json:"machine"`
	From    State    `json:"from"`
	Event   Event    `json:"event"`
	Guard   string   `json:"guard,omitzero"`
	Actions []string `json:"actions"`
	To      State    `json:"to"`
	Rule    int      `json:"rule"`
	Edge    int      `json:"edge"`
}

type Observer func(Fired)

type Machine[C any] struct {
	program  *Program[C]
	state    State
	last     Fired
	hasLast  bool
	busy     bool
	observer Observer
}

func (p *Program[C]) Start(ctx C, observer Observer) (*Machine[C], error) {
	m := &Machine[C]{program: p, state: p.def.Initial, observer: observer}
	for _, action := range p.init {
		if err := action.run(ctx, action.ref.Arg); err != nil {
			return nil, &ActionError{Machine: p.def.Name, Action: action.ref.String(), Err: err}
		}
	}
	return m, nil
}

func (m *Machine[C]) Name() string { return m.program.def.Name }

func (m *Machine[C]) Protocol() string { return m.program.protocol }

func (m *Machine[C]) State() State { return m.state }

func (m *Machine[C]) Last() (Fired, bool) {
	last := m.last
	last.Actions = slices.Clone(last.Actions)
	return last, m.hasLast
}

func (m *Machine[C]) Accepts(ctx C, event Event) bool {
	truth := m.program.truth(ctx)
	for _, candidate := range m.program.table[m.state][event] {
		if candidate.guard == nil || candidate.guard.Eval(truth) {
			return true
		}
	}
	return false
}

func (m *Machine[C]) Fire(ctx C, event Event) (Fired, error) {
	reject := func(err error, guards []string) (Fired, error) {
		return Fired{}, &RejectionError{Machine: m.Name(), State: m.state, Event: event, Guards: guards, Err: err}
	}
	if m.busy {
		return reject(ErrReentrant, nil)
	}
	if !m.program.alphabet[event] {
		return reject(ErrUnknownEvent, nil)
	}
	candidates := m.program.table[m.state][event]
	if len(candidates) == 0 {
		return reject(ErrNotAccepted, nil)
	}
	truth := m.program.truth(ctx)
	var chosen *boundEdge[C]
	var falsified []string
	for i := range candidates {
		candidate := &candidates[i]
		if candidate.guard == nil || candidate.guard.Eval(truth) {
			chosen = candidate
			break
		}
		falsified = append(falsified, candidate.guard.String())
	}
	if chosen == nil {
		return reject(ErrGuardsFailed, falsified)
	}

	fired := Fired{
		Machine: m.Name(),
		From:    m.state,
		Event:   event,
		Guard:   chosen.edge.Guard,
		Actions: slices.Clone(chosen.edge.Actions),
		To:      chosen.edge.To,
		Rule:    chosen.edge.Rule,
		Edge:    chosen.edge.Index,
	}
	m.busy = true
	defer func() { m.busy = false }()
	if m.observer != nil {
		m.observer(fired)
	}
	for _, action := range chosen.actions {
		if err := action.run(ctx, action.ref.Arg); err != nil {
			return fired, &ActionError{Machine: m.Name(), Action: action.ref.String(), Err: err}
		}
	}
	m.state = fired.To
	m.last = fired
	m.hasLast = true
	return fired, nil
}
