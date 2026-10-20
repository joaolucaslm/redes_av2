package fsm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type State string

type Event string

type File struct {
	Protocol  string       `json:"protocol"`
	Family    string       `json:"family"`
	Title     string       `json:"title"`
	Source    string       `json:"source"`
	Summary   string       `json:"summary"`
	Component bool         `json:"component"`
	Uses      []string     `json:"uses"`
	Machines  []Definition `json:"machines"`
}

type Definition struct {
	Name        string           `json:"name"`
	Title       string           `json:"title"`
	Initial     State            `json:"initial"`
	Init        []string         `json:"init"`
	Width       float64          `json:"width"`
	Height      float64          `json:"height"`
	States      []StateSpec      `json:"states"`
	Transitions []TransitionSpec `json:"transitions"`
}

type StateSpec struct {
	ID    State   `json:"id"`
	Label string  `json:"label"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Loop  string  `json:"loop"`
	Note  string  `json:"note"`
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Label struct {
	Event  string `json:"event"`
	Action string `json:"action"`
}

type TransitionSpec struct {
	From    States   `json:"from"`
	Event   Events   `json:"event"`
	Guard   string   `json:"guard"`
	Actions []string `json:"actions"`
	To      State    `json:"to"`
	Label   Label    `json:"label"`
	Bend    float64  `json:"bend"`
	Via     []Point  `json:"via"`
	LabelAt *Point   `json:"labelAt"`
	Loop    string   `json:"loop"`
	Quiet   bool     `json:"quiet"`
}

type Edge struct {
	Index   int      `json:"index"`
	Rule    int      `json:"rule"`
	From    State    `json:"from"`
	Event   Event    `json:"event"`
	Guard   string   `json:"guard,omitzero"`
	Actions []string `json:"actions"`
	To      State    `json:"to"`
	Label   Label    `json:"label"`
	Bend    float64  `json:"bend,omitzero"`
	Via     []Point  `json:"via,omitzero"`
	LabelAt *Point   `json:"labelAt,omitzero"`
	Loop    string   `json:"loop,omitzero"`
	Quiet   bool     `json:"quiet,omitzero"`
}

type States []State

func (s *States) UnmarshalJSON(data []byte) error {
	return unmarshalOneOrMany(data, (*[]State)(s))
}

type Events []Event

func (e *Events) UnmarshalJSON(data []byte) error {
	return unmarshalOneOrMany(data, (*[]Event)(e))
}

func unmarshalOneOrMany[T ~string](data []byte, target *[]T) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return json.Unmarshal(trimmed, target)
	}
	var single T
	if err := json.Unmarshal(trimmed, &single); err != nil {
		return err
	}
	*target = []T{single}
	return nil
}

func ParseFile(data []byte) (File, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var file File
	if err := decoder.Decode(&file); err != nil {
		return File{}, fmt.Errorf("%w: %v", ErrInvalidDefinition, err)
	}
	if err := file.Validate(); err != nil {
		return File{}, err
	}
	return file, nil
}

func (f File) Machine(name string) (Definition, error) {
	for _, def := range f.Machines {
		if def.Name == name {
			return def, nil
		}
	}
	return Definition{}, fmt.Errorf("%w: %s/%s", ErrUnknownMachine, f.Protocol, name)
}

func (d Definition) State(id State) (StateSpec, bool) {
	for _, s := range d.States {
		if s.ID == id {
			return s, true
		}
	}
	return StateSpec{}, false
}

func (d Definition) Edges() []Edge {
	var edges []Edge
	for rule, t := range d.Transitions {
		for _, from := range t.From {
			for _, event := range t.Event {
				edges = append(edges, Edge{
					Index:   len(edges),
					Rule:    rule,
					From:    from,
					Event:   event,
					Guard:   strings.TrimSpace(t.Guard),
					Actions: append([]string(nil), t.Actions...),
					To:      t.To,
					Label:   t.displayLabel(),
					Bend:    t.Bend,
					Via:     append([]Point(nil), t.Via...),
					LabelAt: t.LabelAt,
					Loop:    t.Loop,
					Quiet:   t.Quiet,
				})
			}
		}
	}
	return edges
}

func (t TransitionSpec) displayLabel() Label {
	label := t.Label
	if label.Event == "" {
		names := make([]string, len(t.Event))
		for i, e := range t.Event {
			names[i] = string(e)
		}
		label.Event = strings.Join(names, ", ")
		if guard := strings.TrimSpace(t.Guard); guard != "" {
			label.Event += " [" + guard + "]"
		}
	}
	if label.Action == "" {
		label.Action = "Λ"
		if len(t.Actions) > 0 {
			label.Action = strings.Join(t.Actions, "; ")
		}
	}
	return label
}
