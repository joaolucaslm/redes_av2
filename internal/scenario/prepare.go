package scenario

import (
	"encoding/json"
	"fmt"

	"bancada/internal/channel"
	"bancada/internal/fsm"
	"bancada/internal/host"
	"bancada/internal/sim"
	"bancada/internal/testbed"
)

type Overrides struct {
	Seed              *uint64
	Channel           *channel.Params
	MSL               *sim.VirtualTime
	FastRetransmit    *bool
	CongestionControl *bool
}

func overlay(target any, layers ...json.RawMessage) error {
	for _, layer := range layers {
		if len(layer) == 0 {
			continue
		}
		if err := json.Unmarshal(layer, target); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidSpec, err)
		}
	}
	return nil
}

func (s Spec) Config(run RunSpec, o Overrides) (testbed.Config, error) {
	cfg := testbed.DefaultConfig(run.Protocol)
	cfg.Seed = max(s.Seed, 1)
	if err := overlay(&cfg.Channel, s.Channel); err != nil {
		return cfg, err
	}
	if err := overlay(&cfg.TCP, s.TCP, run.TCP); err != nil {
		return cfg, err
	}
	if err := overlay(&cfg.UDP, s.UDP, run.UDP); err != nil {
		return cfg, err
	}
	if err := overlay(&cfg.RDT, s.RDT, run.RDT); err != nil {
		return cfg, err
	}
	if o.Seed != nil {
		cfg.Seed = *o.Seed
	}
	if o.Channel != nil {
		cfg.Channel = *o.Channel
	}
	if o.MSL != nil {
		cfg.TCP.MSL = *o.MSL
	}
	if o.FastRetransmit != nil {
		cfg.TCP.FastRetransmit = *o.FastRetransmit
	}
	if o.CongestionControl != nil {
		cfg.TCP.CongestionControl = *o.CongestionControl
	}
	return cfg, nil
}

func Prepare(cat *fsm.Catalog, s Spec, run RunSpec, o Overrides) (*testbed.Testbed, error) {
	cfg, err := s.Config(run, o)
	if err != nil {
		return nil, err
	}
	tb, err := testbed.New(cat, cfg)
	if err != nil {
		return nil, err
	}
	for _, rule := range s.Faults {
		if err := tb.Apply(testbed.FaultCommand(rule)); err != nil {
			return nil, err
		}
	}
	steps := s.Script
	if s.Workload != nil {
		steps = append(WorkloadSteps(tb.Family(), *s.Workload), steps...)
	}
	if err := Install(tb, steps); err != nil {
		return nil, err
	}
	return tb, nil
}

func Install(tb *testbed.Testbed, steps []Step) error {
	for _, step := range steps {
		cmd := testbed.AppCommand(step.Host, host.Command{
			Verb:     step.Do,
			Count:    step.Count,
			Size:     step.Size,
			Interval: step.Interval,
			Port:     step.Port,
		})
		if step.At != nil {
			tb.Schedule(*step.At+step.After, cmd)
			continue
		}
		trigger, err := ParseTrigger(step.On)
		if err != nil {
			return err
		}
		if err := tb.When(trigger, step.After, cmd); err != nil {
			return err
		}
	}
	return nil
}

func at(t sim.VirtualTime) *sim.VirtualTime { return &t }

func WorkloadSteps(family string, w Workload) []Step {
	messages := max(w.Messages, 1)
	switch family {
	case "tcp":
		steps := []Step{
			{At: at(0), Host: "B", Do: host.VerbListen},
			{At: at(w.Start + sim.Millisecond), Host: "A", Do: host.VerbConnect},
			{On: "A:ESTABLISHED", Host: "A", Do: host.VerbSend, Count: messages, Size: w.Size},
		}
		if w.Close == nil || *w.Close {
			steps = append(steps,
				Step{On: "A:drained", Host: "A", Do: host.VerbClose},
				Step{On: "B:CLOSE_WAIT", After: 10 * sim.Millisecond, Host: "B", Do: host.VerbClose},
			)
		}
		return steps
	case "udp":
		return []Step{{At: at(w.Start), Host: "A", Do: host.VerbSend, Count: messages, Size: w.Size, Interval: w.Interval}}
	default:
		return []Step{{At: at(w.Start), Host: "A", Do: host.VerbSend, Count: messages, Size: w.Size}}
	}
}
