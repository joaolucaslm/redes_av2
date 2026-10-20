package scenario

import (
	"bancada/internal/fsm"
	"bancada/internal/sim"
	"bancada/internal/testbed"
)

const (
	DefaultUntil    = 120 * sim.Second
	DefaultMaxSteps = 200000
)

type Path struct {
	Host    string   `json:"host"`
	Machine string   `json:"machine"`
	States  []string `json:"states"`
}

type Run struct {
	Name     string          `json:"name"`
	Protocol string          `json:"protocol"`
	Seed     uint64          `json:"seed"`
	Stats    testbed.Stats   `json:"stats"`
	Steps    int             `json:"steps"`
	End      sim.VirtualTime `json:"end"`
	Paths    []Path          `json:"paths"`
	Halted   string          `json:"halted,omitzero"`
	Records  []sim.Record    `json:"-"`
}

type Check struct {
	Title  string `json:"title"`
	Run    string `json:"run,omitzero"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type Result struct {
	ID          string   `json:"id"`
	Number      string   `json:"number"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitzero"`
	Expected    string   `json:"expected,omitzero"`
	Protocols   []string `json:"protocols"`
	Seed        uint64   `json:"seed"`
	Passed      bool     `json:"passed"`
	Checks      []Check  `json:"checks"`
	Runs        []Run    `json:"runs"`
	Error       string   `json:"error,omitzero"`
}

func (r Result) PassedChecks() int {
	n := 0
	for _, c := range r.Checks {
		if c.Passed {
			n++
		}
	}
	return n
}

type Runner struct {
	Catalog *fsm.Catalog
}

func (r Runner) RunAll(specs []Spec) []Result {
	results := make([]Result, len(specs))
	for i, spec := range specs {
		results[i] = r.Run(spec)
	}
	return results
}

func (r Runner) Run(spec Spec) Result {
	result := Result{ID: spec.ID, Number: spec.Number, Title: spec.Title, Description: spec.Description, Expected: spec.Expected}
	for _, runSpec := range spec.RunSpecs() {
		result.Protocols = append(result.Protocols, runSpec.Protocol)
		run, err := r.execute(spec, runSpec)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Seed = run.Seed
		result.Runs = append(result.Runs, run)
	}
	result.Checks = evaluate(spec, result.Runs)
	result.Passed = len(result.Checks) > 0
	for _, c := range result.Checks {
		result.Passed = result.Passed && c.Passed
	}
	return result
}

func (r Runner) Trace(spec Spec, runName string) (Run, error) {
	runSpec, err := spec.Run(runName)
	if err != nil {
		return Run{}, err
	}
	return r.execute(spec, runSpec)
}

func (r Runner) execute(spec Spec, runSpec RunSpec) (Run, error) {
	tb, err := Prepare(r.Catalog, spec, runSpec, Overrides{})
	if err != nil {
		return Run{}, err
	}
	initial := tb.Snapshot()
	until := spec.Until
	if until <= 0 {
		until = DefaultUntil
	}
	maxSteps := spec.MaxSteps
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	run := Run{Name: runSpec.Name, Protocol: runSpec.Protocol, Seed: tb.Config().Seed}
	for {
		next, pending := tb.Peek()
		if !pending {
			break
		}
		if next > until {
			run.Halted = "limite de tempo virtual atingido"
			break
		}
		if run.Steps >= maxSteps {
			run.Halted = "limite de passos atingido"
			break
		}
		tb.Step()
		run.Steps++
	}
	run.Records = tb.Records()
	run.Stats = tb.Stats()
	run.End = tb.Now()
	run.Paths = paths(initial, run.Records)
	return run, nil
}

func paths(initial testbed.Snapshot, records []sim.Record) []Path {
	var out []Path
	for _, h := range initial.Hosts {
		for _, m := range h.Machines {
			p := Path{Host: h.Name, Machine: m.Machine, States: []string{m.State}}
			for _, r := range records {
				if r.ChangesState() && r.Host == h.Name && r.Machine == m.Machine {
					p.States = append(p.States, r.To)
				}
			}
			out = append(out, p)
		}
	}
	return out
}

func (run Run) Path(hostName, machine string) (Path, bool) {
	for _, p := range run.Paths {
		if p.Host == hostName && (machine == "" || p.Machine == machine) {
			return p, true
		}
	}
	return Path{}, false
}
