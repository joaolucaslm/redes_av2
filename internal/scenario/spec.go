package scenario

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"bancada/internal/channel"
	"bancada/internal/host"
	"bancada/internal/sim"
	"bancada/internal/testbed"
)

var (
	ErrInvalidSpec  = errors.New("cenário inválido")
	ErrDuplicateID  = errors.New("id de cenário repetido")
	ErrUnknownSpec  = errors.New("cenário inexistente")
	ErrUnknownRun   = errors.New("execução inexistente no cenário")
	ErrUnknownCheck = errors.New("verificação sem tipo")
)

type Spec struct {
	ID          string          `json:"id"`
	Number      string          `json:"number"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Expected    string          `json:"expected"`
	Protocol    string          `json:"protocol"`
	Runs        []RunSpec       `json:"runs"`
	Seed        uint64          `json:"seed"`
	Channel     json.RawMessage `json:"channel"`
	TCP         json.RawMessage `json:"tcp"`
	UDP         json.RawMessage `json:"udp"`
	RDT         json.RawMessage `json:"rdt"`
	Workload    *Workload       `json:"workload"`
	Script      []Step          `json:"script"`
	Faults      []channel.Rule  `json:"faults"`
	Until       sim.VirtualTime `json:"until"`
	MaxSteps    int             `json:"maxSteps"`
	Expect      []Expectation   `json:"expect"`
	Screenshot  bool            `json:"screenshot"`
}

type RunSpec struct {
	Name     string          `json:"name"`
	Protocol string          `json:"protocol"`
	TCP      json.RawMessage `json:"tcp"`
	UDP      json.RawMessage `json:"udp"`
	RDT      json.RawMessage `json:"rdt"`
}

type Workload struct {
	Messages int             `json:"messages"`
	Size     int             `json:"size"`
	Interval sim.VirtualTime `json:"interval"`
	Start    sim.VirtualTime `json:"start"`
	Close    *bool           `json:"close"`
}

type Step struct {
	At       *sim.VirtualTime `json:"at"`
	On       string           `json:"on"`
	After    sim.VirtualTime  `json:"after"`
	Host     string           `json:"host"`
	Do       host.Verb        `json:"do"`
	Count    int              `json:"count"`
	Size     int              `json:"size"`
	Interval sim.VirtualTime  `json:"interval"`
	Port     uint16           `json:"port"`
}

type RecordMatch struct {
	Kind           sim.Kind `json:"kind"`
	Host           string   `json:"host"`
	Machine        string   `json:"machine"`
	From           string   `json:"from"`
	To             string   `json:"to"`
	Event          string   `json:"event"`
	Action         string   `json:"action"`
	Timer          string   `json:"timer"`
	Metric         string   `json:"metric"`
	Flags          string   `json:"flags"`
	ExactFlags     string   `json:"exactFlags"`
	Data           *bool    `json:"data"`
	Retransmission *bool    `json:"retransmission"`
	Reason         string   `json:"reason"`
	Verdict        string   `json:"verdict"`
	Changes        *bool    `json:"changes"`
}

type VisitCheck struct {
	Host    string   `json:"host"`
	Machine string   `json:"machine"`
	States  []string `json:"states"`
	Exact   bool     `json:"exact"`
}

type MetricCheck struct {
	Name   string   `json:"name"`
	Eq     *float64 `json:"eq"`
	Min    *float64 `json:"min"`
	Max    *float64 `json:"max"`
	Gt     *float64 `json:"gt"`
	Lt     *float64 `json:"lt"`
	SameAs string   `json:"sameAs"`
	Above  string   `json:"above"`
	Below  string   `json:"below"`
}

type OrderCheck struct {
	First RecordMatch `json:"first"`
	Then  RecordMatch `json:"then"`
}

type DwellCheck struct {
	Host    string           `json:"host"`
	Machine string           `json:"machine"`
	State   string           `json:"state"`
	Eq      *sim.VirtualTime `json:"eq"`
	Min     *sim.VirtualTime `json:"min"`
	Max     *sim.VirtualTime `json:"max"`
}

type Expectation struct {
	Title   string       `json:"title"`
	Run     string       `json:"run"`
	Visits  *VisitCheck  `json:"visits"`
	Count   *RecordMatch `json:"count"`
	Metric  *MetricCheck `json:"metric"`
	Order   *OrderCheck  `json:"order"`
	Dwell   *DwellCheck  `json:"dwell"`
	Eq      *int         `json:"eq"`
	AtLeast *int         `json:"atLeast"`
	AtMost  *int         `json:"atMost"`
}

func ParseSpec(data []byte) (Spec, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var spec Spec
	if err := decoder.Decode(&spec); err != nil {
		return Spec{}, fmt.Errorf("%w: %v", ErrInvalidSpec, err)
	}
	return spec, spec.Validate()
}

func (s Spec) RunSpecs() []RunSpec {
	if len(s.Runs) > 0 {
		return s.Runs
	}
	return []RunSpec{{Name: s.Protocol, Protocol: s.Protocol}}
}

func (s Spec) Run(name string) (RunSpec, error) {
	runs := s.RunSpecs()
	if name == "" {
		return runs[0], nil
	}
	for _, r := range runs {
		if r.Name == name {
			return r, nil
		}
	}
	return RunSpec{}, fmt.Errorf("%w: %s/%s", ErrUnknownRun, s.ID, name)
}

func (s Spec) Validate() error {
	var problems []error
	fail := func(format string, args ...any) {
		problems = append(problems, fmt.Errorf("%w: %s: "+format, append([]any{ErrInvalidSpec, s.ID}, args...)...))
	}
	if s.ID == "" || s.Title == "" {
		fail("id e title são obrigatórios")
	}
	if (s.Protocol == "") == (len(s.Runs) == 0) {
		fail("use protocol ou runs, exatamente um dos dois")
	}
	names := make(map[string]bool)
	for _, r := range s.Runs {
		if r.Name == "" || r.Protocol == "" || names[r.Name] {
			fail("execução sem nome, sem protocolo ou repetida: %+v", r.Name)
		}
		names[r.Name] = true
	}
	for i, step := range s.Script {
		if (step.At == nil) == (step.On == "") {
			fail("passo #%d precisa de exatamente um entre at e on", i)
		}
		if step.On != "" {
			if _, err := ParseTrigger(step.On); err != nil {
				fail("passo #%d: %v", i, err)
			}
		}
		if step.Host == "" || step.Do == "" {
			fail("passo #%d sem host ou do", i)
		}
	}
	for i, rule := range s.Faults {
		if err := rule.Validate(); err != nil {
			fail("falha #%d: %v", i, err)
		}
	}
	for i, e := range s.Expect {
		kinds := 0
		for _, set := range []bool{e.Visits != nil, e.Count != nil, e.Metric != nil, e.Order != nil, e.Dwell != nil} {
			if set {
				kinds++
			}
		}
		if kinds != 1 {
			fail("verificação #%d deve ter exatamente um entre visits, count, metric, order e dwell", i)
		}
		if e.Count != nil && e.Eq == nil && e.AtLeast == nil && e.AtMost == nil {
			fail("verificação #%d (count) sem eq, atLeast ou atMost", i)
		}
		if e.Run != "" {
			if _, err := s.Run(e.Run); err != nil {
				fail("verificação #%d: %v", i, err)
			}
		}
	}
	return errors.Join(problems...)
}

func ParseTrigger(text string) (testbed.Trigger, error) {
	hostName, condition, ok := strings.Cut(text, ":")
	if !ok || hostName == "" || condition == "" {
		return testbed.Trigger{}, fmt.Errorf("%w: gatilho %q (use HOST:ESTADO ou HOST:drained)", ErrInvalidSpec, text)
	}
	if condition == "drained" {
		return testbed.Trigger{Host: hostName, Drained: true}, nil
	}
	return testbed.Trigger{Host: hostName, State: condition}, nil
}

func LoadSpecs(fsys fs.FS) ([]Spec, error) {
	names, err := fs.Glob(fsys, "*.json")
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	var specs []Spec
	var problems []error
	seen := make(map[string]bool)
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, err))
			continue
		}
		spec, err := ParseSpec(data)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if seen[spec.ID] {
			problems = append(problems, fmt.Errorf("%s: %w: %s", name, ErrDuplicateID, spec.ID))
			continue
		}
		seen[spec.ID] = true
		specs = append(specs, spec)
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return specs, nil
}

func Find(specs []Spec, id string) (Spec, error) {
	for _, s := range specs {
		if s.ID == id {
			return s, nil
		}
	}
	return Spec{}, fmt.Errorf("%w: %s", ErrUnknownSpec, id)
}
