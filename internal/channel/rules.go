package channel

import (
	"fmt"
	"slices"
	"strings"

	"bancada/internal/sim"
)

type Action string

const (
	ActionDrop      Action = "drop"
	ActionCorrupt   Action = "corrupt"
	ActionDuplicate Action = "duplicate"
)

type Selector struct {
	From           string `json:"from,omitzero"`
	Proto          string `json:"proto,omitzero"`
	Flags          string `json:"flags,omitzero"`
	ExactFlags     string `json:"exactFlags,omitzero"`
	Data           *bool  `json:"data,omitzero"`
	Retransmission *bool  `json:"retransmission,omitzero"`
	Message        int    `json:"message,omitzero"`
}

type Rule struct {
	Action Action   `json:"action"`
	Match  Selector `json:"match"`
	Nth    int      `json:"nth"`
}

func (r Rule) Validate() error {
	if !slices.Contains([]Action{ActionDrop, ActionCorrupt, ActionDuplicate}, r.Action) {
		return fmt.Errorf("%w: ação %q", ErrInvalidRule, r.Action)
	}
	if r.Nth < 1 {
		return fmt.Errorf("%w: nth=%d (deve ser >= 1)", ErrInvalidRule, r.Nth)
	}
	return nil
}

func (s Selector) Matches(src string, h sim.Header) bool {
	switch {
	case s.From != "" && s.From != src:
		return false
	case s.Proto != "" && s.Proto != h.Proto:
		return false
	case s.Flags != "" && !containsFlags(h.Flags, s.Flags):
		return false
	case s.ExactFlags != "" && !sameFlags(h.Flags, s.ExactFlags):
		return false
	case s.Data != nil && *s.Data != (h.Payload > 0):
		return false
	case s.Retransmission != nil && *s.Retransmission != h.Retransmission:
		return false
	case s.Message != 0 && s.Message != h.Message:
		return false
	}
	return true
}

func flagSet(text string) []string {
	var set []string
	for _, f := range strings.Split(text, "|") {
		if f = strings.TrimSpace(strings.ToUpper(f)); f != "" {
			set = append(set, f)
		}
	}
	slices.Sort(set)
	return set
}

func containsFlags(have, want string) bool {
	present := flagSet(have)
	for _, f := range flagSet(want) {
		if !slices.Contains(present, f) {
			return false
		}
	}
	return true
}

func sameFlags(have, want string) bool {
	return slices.Equal(flagSet(have), flagSet(want))
}

func MatchesFlags(have, want string) bool { return containsFlags(have, want) }

type armedRule struct {
	rule  Rule
	seen  int
	spent bool
}
