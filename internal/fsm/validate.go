package fsm

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

func (f File) Validate() error {
	var problems []error
	fail := func(machine string, err error, detail string) {
		problems = append(problems, &DefinitionError{Protocol: f.Protocol, Machine: machine, Err: err, Detail: detail})
	}
	if strings.TrimSpace(f.Protocol) == "" {
		fail("", ErrMissingField, "protocol")
	}
	if strings.TrimSpace(f.Family) == "" {
		fail("", ErrMissingField, "family")
	}
	if len(f.Machines) == 0 {
		fail("", ErrMissingField, "machines")
	}
	seen := make(map[string]bool)
	for _, def := range f.Machines {
		if def.Name == "" {
			fail("", ErrMissingField, "machines[].name")
			continue
		}
		if seen[def.Name] {
			fail(def.Name, ErrDuplicateMachine, def.Name)
		}
		seen[def.Name] = true
		problems = append(problems, validateDefinition(f.Protocol, def)...)
	}
	return errors.Join(problems...)
}

type transitionKey struct {
	from  State
	event Event
}

func validateDefinition(protocol string, def Definition) []error {
	var problems []error
	fail := func(where string, err error, detail string) {
		problems = append(problems, &DefinitionError{Protocol: protocol, Machine: def.Name, Where: where, Err: err, Detail: detail})
	}
	if len(def.States) == 0 {
		fail("", ErrMissingField, "states")
	}
	known := make(map[State]bool)
	for _, s := range def.States {
		switch {
		case s.ID == "":
			fail("states", ErrMissingField, "id")
		case known[s.ID]:
			fail("states", ErrDuplicateState, string(s.ID))
		}
		known[s.ID] = true
	}
	switch {
	case def.Initial == "":
		fail("", ErrMissingField, "initial")
	case !known[def.Initial]:
		fail("initial", ErrUnknownState, string(def.Initial))
	}
	for _, source := range def.Init {
		if _, err := ParseAction(source); err != nil {
			fail("init", err, "")
		}
	}

	guardsSeen := make(map[transitionKey][]string)
	unguardedAt := make(map[transitionKey]int)
	for i, t := range def.Transitions {
		where := fmt.Sprintf("transição #%d", i)
		if len(t.From) == 0 {
			fail(where, ErrMissingField, "from")
		}
		if len(t.Event) == 0 {
			fail(where, ErrMissingField, "event")
		}
		switch {
		case t.To == "":
			fail(where, ErrMissingField, "to")
		case !known[t.To]:
			fail(where, ErrUnknownState, "to: "+string(t.To))
		}
		for _, from := range t.From {
			if !known[from] {
				fail(where, ErrUnknownState, "from: "+string(from))
			}
		}
		guard, err := ParseGuard(t.Guard)
		if err != nil {
			fail(where, err, "")
		}
		normalized := ""
		if guard != nil {
			normalized = guard.String()
		}
		for _, source := range t.Actions {
			if _, err := ParseAction(source); err != nil {
				fail(where, err, "")
			}
		}
		for _, from := range t.From {
			for _, event := range t.Event {
				if event == "" {
					fail(where, ErrMissingField, "event")
					continue
				}
				key := transitionKey{from: from, event: event}
				if earlier, ok := unguardedAt[key]; ok {
					fail(where, ErrUnreachableTransition, fmt.Sprintf("%s --%s--> já é coberta sem guarda pela transição #%d", from, event, earlier))
					continue
				}
				if slices.Contains(guardsSeen[key], normalized) {
					fail(where, ErrDuplicateTransition, fmt.Sprintf("%s --%s [%s]-->", from, event, normalized))
					continue
				}
				guardsSeen[key] = append(guardsSeen[key], normalized)
				if normalized == "" {
					unguardedAt[key] = i
				}
			}
		}
	}
	return problems
}
