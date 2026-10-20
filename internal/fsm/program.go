package fsm

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type Program[C any] struct {
	protocol string
	def      Definition
	edges    []Edge
	table    map[State]map[Event][]boundEdge[C]
	alphabet map[Event]bool
	guards   map[string]GuardFunc[C]
	init     []boundAction[C]
}

type boundEdge[C any] struct {
	edge    Edge
	guard   *GuardExpr
	actions []boundAction[C]
}

type boundAction[C any] struct {
	ref ActionRef
	run ActionFunc[C]
}

func Compile[C any](protocol string, def Definition, reg *Registry[C]) (*Program[C], error) {
	if problems := validateDefinition(protocol, def); len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	p := &Program[C]{
		protocol: protocol,
		def:      def,
		edges:    def.Edges(),
		table:    make(map[State]map[Event][]boundEdge[C]),
		alphabet: make(map[Event]bool),
		guards:   make(map[string]GuardFunc[C]),
	}
	var problems []error
	fail := func(where string, err error, detail string) {
		problems = append(problems, &DefinitionError{Protocol: protocol, Machine: def.Name, Where: where, Err: err, Detail: detail})
	}
	bind := func(where, source string) (boundAction[C], bool) {
		ref, err := ParseAction(source)
		if err != nil {
			fail(where, err, "")
			return boundAction[C]{}, false
		}
		entry, ok := reg.actions[ref.Name]
		if !ok {
			fail(where, ErrUnknownAction, ref.Name)
			return boundAction[C]{}, false
		}
		if entry.allowed != nil && !slices.Contains(entry.allowed, ref.Arg) {
			fail(where, ErrBadActionArgument, fmt.Sprintf("%s: permitidos %s", ref, describeArgs(ref.Name, entry.allowed)))
			return boundAction[C]{}, false
		}
		return boundAction[C]{ref: ref, run: entry.run}, true
	}

	for _, source := range def.Init {
		if action, ok := bind("init", source); ok {
			p.init = append(p.init, action)
		}
	}
	for _, edge := range p.edges {
		where := fmt.Sprintf("transição #%d", edge.Rule)
		guard, err := ParseGuard(edge.Guard)
		if err != nil {
			fail(where, err, "")
			continue
		}
		for _, name := range guard.Names() {
			fn, ok := reg.guards[name]
			if !ok {
				fail(where, ErrUnknownGuard, name)
				continue
			}
			p.guards[name] = fn
		}
		bound := boundEdge[C]{edge: edge, guard: guard}
		for _, source := range edge.Actions {
			if action, ok := bind(where, source); ok {
				bound.actions = append(bound.actions, action)
			}
		}
		if p.table[edge.From] == nil {
			p.table[edge.From] = make(map[Event][]boundEdge[C])
		}
		p.table[edge.From][edge.Event] = append(p.table[edge.From][edge.Event], bound)
		p.alphabet[edge.Event] = true
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return p, nil
}

func describeArgs(name string, allowed []string) string {
	forms := make([]string, len(allowed))
	for i, arg := range allowed {
		forms[i] = ActionRef{Name: name, Arg: arg}.String()
	}
	return strings.Join(forms, ", ")
}

func (p *Program[C]) Protocol() string { return p.protocol }

func (p *Program[C]) Name() string { return p.def.Name }

func (p *Program[C]) Definition() Definition { return p.def }

func (p *Program[C]) Edges() []Edge { return slices.Clone(p.edges) }

func (p *Program[C]) Alphabet() []Event {
	return slices.Sorted(maps.Keys(p.alphabet))
}

func (p *Program[C]) truth(ctx C) func(string) bool {
	return func(name string) bool { return p.guards[name](ctx) }
}
