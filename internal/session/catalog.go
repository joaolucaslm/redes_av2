package session

import (
	"bancada/internal/channel"
	"bancada/internal/fsm"
	"bancada/internal/scenario"
	"bancada/internal/sim"
	"bancada/internal/tcp"
	"bancada/internal/testbed"
)

type MachineDTO struct {
	Protocol string          `json:"protocol"`
	Machine  string          `json:"machine"`
	Title    string          `json:"title"`
	Initial  string          `json:"initial"`
	Width    float64         `json:"width"`
	Height   float64         `json:"height"`
	States   []fsm.StateSpec `json:"states"`
	Edges    []fsm.Edge      `json:"edges"`
}

type ProtocolDTO struct {
	ID       string       `json:"id"`
	Family   string       `json:"family"`
	Title    string       `json:"title"`
	Source   string       `json:"source"`
	Summary  string       `json:"summary"`
	Machines []MachineDTO `json:"machines"`
}

type ScenarioDTO struct {
	ID          string     `json:"id"`
	Number      string     `json:"number"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Expected    string     `json:"expected"`
	Runs        []LaneInfo `json:"runs"`
	Seed        uint64     `json:"seed"`
	Screenshot  bool       `json:"screenshot"`
}

type Defaults struct {
	Seed           uint64          `json:"seed"`
	Channel        channel.Params  `json:"channel"`
	MSL            sim.VirtualTime `json:"msl"`
	FastRetransmit bool            `json:"fastRetransmit"`
	SlowMotion     float64         `json:"slowMotion"`
	Speeds         []float64       `json:"speeds"`
}

type Catalog struct {
	Protocols []ProtocolDTO `json:"protocols"`
	Scenarios []ScenarioDTO `json:"scenarios"`
	Defaults  Defaults      `json:"defaults"`
	Warnings  []string      `json:"warnings"`
}

func machineDTO(protocol string, def fsm.Definition) MachineDTO {
	return MachineDTO{
		Protocol: protocol,
		Machine:  def.Name,
		Title:    def.Title,
		Initial:  string(def.Initial),
		Width:    def.Width,
		Height:   def.Height,
		States:   def.States,
		Edges:    def.Edges(),
	}
}

func BuildCatalog(cat *fsm.Catalog, specs []scenario.Spec, warnings []string) Catalog {
	out := Catalog{Warnings: append([]string{}, warnings...)}
	for _, file := range testbed.Runnable(cat) {
		dto := ProtocolDTO{ID: file.Protocol, Family: file.Family, Title: file.Title, Source: file.Source, Summary: file.Summary}
		for _, def := range file.Machines {
			dto.Machines = append(dto.Machines, machineDTO(file.Protocol, def))
		}
		for _, used := range file.Uses {
			if dep, err := cat.File(used); err == nil {
				for _, def := range dep.Machines {
					dto.Machines = append(dto.Machines, machineDTO(dep.Protocol, def))
				}
			}
		}
		out.Protocols = append(out.Protocols, dto)
	}
	for _, spec := range specs {
		dto := ScenarioDTO{ID: spec.ID, Number: spec.Number, Title: spec.Title, Description: spec.Description, Expected: spec.Expected, Seed: spec.Seed, Screenshot: spec.Screenshot}
		for _, run := range spec.RunSpecs() {
			dto.Runs = append(dto.Runs, LaneInfo{Name: run.Name, Protocol: run.Protocol})
		}
		out.Scenarios = append(out.Scenarios, dto)
	}
	defaults := tcp.DefaultConfig()
	out.Defaults = Defaults{
		Seed:           1,
		Channel:        channel.DefaultParams(),
		MSL:            defaults.MSL,
		FastRetransmit: defaults.FastRetransmit,
		SlowMotion:     SlowMotion,
		Speeds:         []float64{0.25, 0.5, 1, 2, 4, 8, 16, 32, 64},
	}
	return out
}
