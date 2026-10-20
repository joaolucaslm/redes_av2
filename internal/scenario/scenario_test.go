package scenario

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	fsmfiles "bancada/fsm"
	"bancada/internal/fsm"
	"bancada/internal/testbed"
	scenariofiles "bancada/scenarios"
)

func fixtures(t *testing.T) (*fsm.Catalog, []Spec) {
	t.Helper()
	cat, err := testbed.LoadCatalog(fsmfiles.Files)
	if err != nil {
		t.Fatalf("catálogo: %v", err)
	}
	specs, err := LoadSpecs(scenariofiles.Files)
	if err != nil {
		t.Fatalf("cenários: %v", err)
	}
	return cat, specs
}

func TestEveryScenarioPasses(t *testing.T) {
	cat, specs := fixtures(t)
	runner := Runner{Catalog: cat}
	for _, spec := range specs {
		t.Run(spec.ID, func(t *testing.T) {
			t.Parallel()
			result := runner.Run(spec)
			if result.Error != "" {
				t.Fatalf("erro ao preparar: %s", result.Error)
			}
			for _, c := range result.Checks {
				if !c.Passed {
					t.Errorf("%s [%s]: %s", c.Title, c.Run, c.Detail)
				}
			}
			if len(result.Checks) == 0 {
				t.Error("cenário sem verificações")
			}
		})
	}
}

func TestAssignmentScenariosArePresent(t *testing.T) {
	_, specs := fixtures(t)
	var numbers []string
	for _, s := range specs {
		numbers = append(numbers, s.Number)
	}
	for _, required := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"} {
		if !slices.Contains(numbers, required) {
			t.Errorf("cenário %s do enunciado ausente", required)
		}
	}
}

func TestScenariosAreDeterministic(t *testing.T) {
	cat, specs := fixtures(t)
	runner := Runner{Catalog: cat}
	fingerprint := func(spec Spec) string {
		var b strings.Builder
		for _, run := range runner.Run(spec).Runs {
			data, err := json.Marshal(run.Records)
			if err != nil {
				t.Fatal(err)
			}
			b.Write(data)
		}
		return b.String()
	}
	for _, spec := range specs {
		if fingerprint(spec) != fingerprint(spec) {
			t.Errorf("%s: duas execuções com a mesma seed divergiram", spec.ID)
		}
	}
}

func TestSeedChangesTheNoisyComparison(t *testing.T) {
	cat, specs := fixtures(t)
	spec, err := Find(specs, "10-udp-vs-tcp")
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{Catalog: cat}
	first := runner.Run(spec).Runs[0].Stats
	spec.Seed++
	second := runner.Run(spec).Runs[0].Stats
	if first == second {
		t.Fatal("seeds diferentes deveriam mudar as falhas do canal")
	}
}

func TestSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"sem protocolo", `{"id":"x","title":"x"}`},
		{"protocolo e runs juntos", `{"id":"x","title":"x","protocol":"tcp","runs":[{"name":"a","protocol":"tcp"}]}`},
		{"passo sem at nem on", `{"id":"x","title":"x","protocol":"tcp","script":[{"host":"A","do":"connect"}]}`},
		{"gatilho malformado", `{"id":"x","title":"x","protocol":"tcp","script":[{"on":"ESTABLISHED","host":"A","do":"send"}]}`},
		{"verificação ambígua", `{"id":"x","title":"x","protocol":"tcp","expect":[{"metric":{"name":"sent"},"visits":{"host":"A"}}]}`},
		{"count sem limite", `{"id":"x","title":"x","protocol":"tcp","expect":[{"count":{"kind":"send"}}]}`},
		{"execução desconhecida", `{"id":"x","title":"x","protocol":"tcp","expect":[{"run":"nada","metric":{"name":"sent","eq":1}}]}`},
		{"regra inválida", `{"id":"x","title":"x","protocol":"tcp","faults":[{"action":"drop","nth":0}]}`},
		{"campo desconhecido", `{"id":"x","title":"x","protocol":"tcp","sed":3}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseSpec([]byte(tc.json)); !errors.Is(err, ErrInvalidSpec) {
				t.Fatalf("esperava ErrInvalidSpec, veio %v", err)
			}
		})
	}
}

func TestFailingExpectationIsReported(t *testing.T) {
	cat, _ := fixtures(t)
	spec, err := ParseSpec([]byte(`{"id":"neg","title":"negativo","protocol":"tcp","seed":1,
		"script":[{"at":"0ms","host":"B","do":"listen"},{"at":"1ms","host":"A","do":"connect"}],
		"expect":[{"visits":{"host":"A","states":["CLOSED","LISTEN"]}},{"metric":{"name":"sent","eq":99}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	result := Runner{Catalog: cat}.Run(spec)
	if result.Passed || result.PassedChecks() != 0 {
		t.Fatalf("expectativas falsas deveriam reprovar: %+v", result.Checks)
	}
}
