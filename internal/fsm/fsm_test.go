package fsm

import (
	"errors"
	"strings"
	"testing"
)

type door struct {
	locked  bool
	opened  int
	journal []string
}

func doorRegistry() *Registry[*door] {
	return NewRegistry[*door]().
		Guard("locked", func(d *door) bool { return d.locked }).
		Guard("unlocked", func(d *door) bool { return !d.locked }).
		Effect("open", func(d *door) { d.opened++ }).
		Action("say", func(d *door, arg string) error {
			d.journal = append(d.journal, arg)
			return nil
		}, "oi", "tchau").
		Action("explode", func(*door, string) error { return errors.New("bum") })
}

const doorJSON = `{
  "protocol": "porta",
  "family": "teste",
  "machines": [{
    "name": "porta",
    "initial": "FECHADA",
    "init": ["say(oi)"],
    "states": [{"id": "FECHADA"}, {"id": "ABERTA"}],
    "transitions": [
      {"from": "FECHADA", "event": "empurrar", "guard": "unlocked", "actions": ["open", "say(oi)"], "to": "ABERTA"},
      {"from": "FECHADA", "event": "empurrar", "guard": "locked", "actions": [], "to": "FECHADA"},
      {"from": "ABERTA", "event": ["puxar", "vento"], "actions": ["say(tchau)"], "to": "FECHADA"},
      {"from": ["ABERTA", "FECHADA"], "event": "chutar", "actions": ["explode"], "to": "FECHADA"}
    ]
  }]
}`

func compileDoor(t *testing.T) *Program[*door] {
	t.Helper()
	file, err := ParseFile([]byte(doorJSON))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	def, err := file.Machine("porta")
	if err != nil {
		t.Fatalf("Machine: %v", err)
	}
	program, err := Compile("porta", def, doorRegistry())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return program
}

func TestMachineFiresBookStyleTransitions(t *testing.T) {
	program := compileDoor(t)
	ctx := &door{}
	var observed []Fired
	m, err := program.Start(ctx, func(f Fired) { observed = append(observed, f) })
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := strings.Join(ctx.journal, ","); got != "oi" {
		t.Fatalf("ações iniciais = %q, quero %q", got, "oi")
	}

	steps := []struct {
		event  Event
		locked bool
		want   State
	}{
		{"empurrar", true, "FECHADA"},
		{"empurrar", false, "ABERTA"},
		{"vento", false, "FECHADA"},
	}
	for _, step := range steps {
		ctx.locked = step.locked
		if _, err := m.Fire(ctx, step.event); err != nil {
			t.Fatalf("Fire(%s): %v", step.event, err)
		}
		if m.State() != step.want {
			t.Fatalf("após %s estado = %s, quero %s", step.event, m.State(), step.want)
		}
	}
	if ctx.opened != 1 || len(observed) != 3 {
		t.Fatalf("opened=%d observed=%d", ctx.opened, len(observed))
	}
	last, ok := m.Last()
	if !ok || last.Event != "vento" || last.Rule != 2 {
		t.Fatalf("última transição = %+v", last)
	}
}

func TestMachineRejectsWithTypedErrors(t *testing.T) {
	program := compileDoor(t)
	tests := []struct {
		name   string
		setup  []Event
		locked bool
		event  Event
		want   error
	}{
		{"evento fora do alfabeto", nil, false, "voar", ErrUnknownEvent},
		{"evento sem transição no estado", nil, false, "puxar", ErrNotAccepted},
		{"ação que falha", nil, false, "chutar", ErrActionFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := &door{locked: tc.locked}
			m, err := program.Start(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range tc.setup {
				if _, err := m.Fire(ctx, e); err != nil {
					t.Fatal(err)
				}
			}
			before := m.State()
			_, err = m.Fire(ctx, tc.event)
			if !errors.Is(err, tc.want) {
				t.Fatalf("erro = %v, quero %v", err, tc.want)
			}
			if m.State() != before {
				t.Fatalf("estado mudou de %s para %s após rejeição", before, m.State())
			}
		})
	}
}

func TestRejectionCarriesReason(t *testing.T) {
	file, err := ParseFile([]byte(`{"protocol":"p","family":"f","machines":[{"name":"m","initial":"A",
		"states":[{"id":"A"}],
		"transitions":[{"from":"A","event":"e","guard":"locked && unlocked","to":"A"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	program, err := Compile("p", file.Machines[0], doorRegistry())
	if err != nil {
		t.Fatal(err)
	}
	m, err := program.Start(&door{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.Fire(&door{}, "e")
	var rejection *RejectionError
	if !errors.As(err, &rejection) {
		t.Fatalf("erro %T não é *RejectionError", err)
	}
	if !errors.Is(err, ErrGuardsFailed) || rejection.State != "A" || !strings.Contains(rejection.Reason(), "locked && unlocked") {
		t.Fatalf("rejeição inesperada: %v", rejection)
	}
}

func TestDefinitionValidation(t *testing.T) {
	base := func(transitions string) string {
		return `{"protocol":"p","family":"f","machines":[{"name":"m","initial":"A",
			"states":[{"id":"A"},{"id":"B"}],"transitions":[` + transitions + `]}]}`
	}
	tests := []struct {
		name string
		json string
		want error
	}{
		{"estado de destino inexistente", base(`{"from":"A","event":"e","to":"Z"}`), ErrUnknownState},
		{"estado de origem inexistente", base(`{"from":"Z","event":"e","to":"A"}`), ErrUnknownState},
		{"transição duplicada", base(`{"from":"A","event":"e","guard":"locked","to":"B"},{"from":"A","event":"e","guard":"locked","to":"A"}`), ErrDuplicateTransition},
		{"transição inalcançável", base(`{"from":"A","event":"e","to":"B"},{"from":"A","event":"e","guard":"locked","to":"A"}`), ErrUnreachableTransition},
		{"guarda malformada", base(`{"from":"A","event":"e","guard":"locked &&","to":"B"}`), ErrMalformedGuard},
		{"ação malformada", base(`{"from":"A","event":"e","actions":["say(oi"],"to":"B"}`), ErrMalformedAction},
		{"inicial ausente", `{"protocol":"p","family":"f","machines":[{"name":"m","states":[{"id":"A"}],"transitions":[]}]}`, ErrMissingField},
		{"campo desconhecido", `{"protocol":"p","family":"f","machinez":[]}`, ErrInvalidDefinition},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFile([]byte(tc.json))
			if !errors.Is(err, tc.want) {
				t.Fatalf("erro = %v, quero %v", err, tc.want)
			}
			if !errors.Is(err, ErrInvalidDefinition) {
				t.Fatalf("erro %v deveria também ser ErrInvalidDefinition", err)
			}
		})
	}
}

func TestCompileChecksRegistry(t *testing.T) {
	tests := []struct {
		name       string
		transition string
		want       error
	}{
		{"guarda não registrada", `{"from":"A","event":"e","guard":"aberta","to":"A"}`, ErrUnknownGuard},
		{"ação não registrada", `{"from":"A","event":"e","actions":["voar"],"to":"A"}`, ErrUnknownAction},
		{"argumento não permitido", `{"from":"A","event":"e","actions":["say(olá)"],"to":"A"}`, ErrBadActionArgument},
		{"efeito não aceita argumento", `{"from":"A","event":"e","actions":["open(2)"],"to":"A"}`, ErrBadActionArgument},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file, err := ParseFile([]byte(`{"protocol":"p","family":"f","machines":[{"name":"m","initial":"A","states":[{"id":"A"}],"transitions":[` + tc.transition + `]}]}`))
			if err != nil {
				t.Fatal(err)
			}
			_, err = Compile("p", file.Machines[0], doorRegistry())
			if !errors.Is(err, tc.want) {
				t.Fatalf("erro = %v, quero %v", err, tc.want)
			}
			var defErr *DefinitionError
			if !errors.As(err, &defErr) {
				t.Fatalf("erro %T sem *DefinitionError", err)
			}
		})
	}
}

func TestGuardExpressions(t *testing.T) {
	truth := map[string]bool{"a": true, "b": false, "c": true, "x.y": true}
	tests := []struct {
		source     string
		want       bool
		normalized string
	}{
		{"a", true, "a"},
		{"!b", true, "!b"},
		{"a && b", false, "a && b"},
		{"a || b", true, "a || b"},
		{"b || a && c", true, "b || a && c"},
		{"(b || a) && !c", false, "(b || a) && !c"},
		{"!(a && b)", true, "!(a && b)"},
		{"x.y && a", true, "x.y && a"},
	}
	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			expr, err := ParseGuard(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if got := expr.Eval(func(n string) bool { return truth[n] }); got != tc.want {
				t.Fatalf("Eval = %v, quero %v", got, tc.want)
			}
			if expr.String() != tc.normalized {
				t.Fatalf("String = %q, quero %q", expr.String(), tc.normalized)
			}
		})
	}
	for _, bad := range []string{"a &", "a && ", "(a", "a b", "&& a", "a ||| b", "a $ b"} {
		if _, err := ParseGuard(bad); !errors.Is(err, ErrMalformedGuard) {
			t.Errorf("ParseGuard(%q) = %v, quero ErrMalformedGuard", bad, err)
		}
	}
}

func TestActionReferences(t *testing.T) {
	tests := []struct {
		source string
		want   ActionRef
		bad    bool
	}{
		{"send(SYN|ACK)", ActionRef{"send", "SYN|ACK"}, false},
		{"tcb.release", ActionRef{"tcb.release", ""}, false},
		{" timer.start( 2msl ) ", ActionRef{"timer.start", "2msl"}, false},
		{"send(", ActionRef{}, true},
		{"(x)", ActionRef{}, true},
		{"a b", ActionRef{}, true},
	}
	for _, tc := range tests {
		got, err := ParseAction(tc.source)
		if tc.bad {
			if !errors.Is(err, ErrMalformedAction) {
				t.Errorf("ParseAction(%q) = %v, quero erro", tc.source, err)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("ParseAction(%q) = %+v, %v", tc.source, got, err)
		}
	}
}

func TestEdgesExpandListsAndKeepRule(t *testing.T) {
	program := compileDoor(t)
	edges := program.Edges()
	if len(edges) != 6 {
		t.Fatalf("len(edges) = %d, quero 6", len(edges))
	}
	if edges[2].Event != "puxar" || edges[3].Event != "vento" || edges[2].Rule != 2 || edges[3].Rule != 2 {
		t.Fatalf("expansão de eventos incorreta: %+v %+v", edges[2], edges[3])
	}
	if edges[3].Label.Action != "say(tchau)" || edges[1].Label.Action != "Λ" {
		t.Fatalf("rótulos padrão incorretos: %q %q", edges[3].Label.Action, edges[1].Label.Action)
	}
}

func TestReentrantFireIsRejected(t *testing.T) {
	file, err := ParseFile([]byte(`{"protocol":"p","family":"f","machines":[{"name":"m","initial":"A",
		"states":[{"id":"A"}],"transitions":[{"from":"A","event":"e","actions":["again"],"to":"A"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var m *Machine[*door]
	var inner error
	reg := NewRegistry[*door]().Action("again", func(d *door, _ string) error {
		_, inner = m.Fire(d, "e")
		return nil
	}, "")
	program, err := Compile("p", file.Machines[0], reg)
	if err != nil {
		t.Fatal(err)
	}
	m, err = program.Start(&door{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Fire(&door{}, "e"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(inner, ErrReentrant) {
		t.Fatalf("disparo interno = %v, quero ErrReentrant", inner)
	}
}
