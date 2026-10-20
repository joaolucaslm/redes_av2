package scenario

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"bancada/internal/channel"
	"bancada/internal/sim"
)

func (m RecordMatch) Matches(r sim.Record) bool {
	header := r.Flight.Header
	switch {
	case m.Kind != "" && m.Kind != r.Kind,
		m.Host != "" && m.Host != r.Host,
		m.Machine != "" && m.Machine != r.Machine,
		m.From != "" && m.From != r.From,
		m.To != "" && m.To != r.To,
		m.Event != "" && m.Event != r.Event,
		m.Action != "" && !slices.Contains(r.Actions, m.Action),
		m.Timer != "" && m.Timer != r.Timer,
		m.Metric != "" && m.Metric != r.Metric,
		m.Flags != "" && !channel.MatchesFlags(header.Flags, m.Flags),
		m.ExactFlags != "" && !(channel.MatchesFlags(header.Flags, m.ExactFlags) && channel.MatchesFlags(m.ExactFlags, header.Flags)),
		m.Data != nil && *m.Data != (header.Payload > 0),
		m.Retransmission != nil && *m.Retransmission != header.Retransmission,
		m.Reason != "" && !strings.Contains(strings.Join([]string{r.Reason, header.Reason, r.Flight.Cause}, " "), m.Reason),
		m.Verdict != "" && m.Verdict != r.Verdict,
		m.Changes != nil && *m.Changes != r.ChangesState():
		return false
	}
	return true
}

func (m RecordMatch) String() string {
	var parts []string
	add := func(key, value string) {
		if value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	add("host", m.Host)
	add("máquina", m.Machine)
	add("de", m.From)
	add("evento", m.Event)
	add("para", m.To)
	add("ação", m.Action)
	add("timer", m.Timer)
	add("métrica", m.Metric)
	add("flags", m.Flags)
	add("flags exatas", m.ExactFlags)
	add("motivo", m.Reason)
	add("veredito", m.Verdict)
	if m.Data != nil {
		add("dados", strconv.FormatBool(*m.Data))
	}
	if m.Retransmission != nil {
		add("retransmissão", strconv.FormatBool(*m.Retransmission))
	}
	kind := string(m.Kind)
	if kind == "" {
		kind = "registro"
	}
	return kind + "[" + strings.Join(parts, " ") + "]"
}

func evaluate(spec Spec, runs []Run) []Check {
	byName := make(map[string]Run)
	for _, r := range runs {
		byName[r.Name] = r
	}
	checks := make([]Check, 0, len(spec.Expect))
	for _, e := range spec.Expect {
		run := runs[0]
		if e.Run != "" {
			run = byName[e.Run]
		}
		check := judge(e, run, byName)
		if len(runs) > 1 {
			check.Run = run.Name
		}
		if e.Title != "" {
			check.Title = e.Title
		}
		if run.Halted != "" && !check.Passed {
			check.Detail += " · " + run.Halted
		}
		checks = append(checks, check)
	}
	return checks
}

func judge(e Expectation, run Run, runs map[string]Run) Check {
	switch {
	case e.Visits != nil:
		return judgeVisits(*e.Visits, run)
	case e.Count != nil:
		return judgeCount(*e.Count, e, run)
	case e.Metric != nil:
		return judgeMetric(*e.Metric, run, runs)
	case e.Order != nil:
		return judgeOrder(*e.Order, run)
	case e.Dwell != nil:
		return judgeDwell(*e.Dwell, run)
	default:
		return Check{Title: "verificação vazia", Detail: ErrUnknownCheck.Error()}
	}
}

func judgeVisits(v VisitCheck, run Run) Check {
	title := fmt.Sprintf("%s visita %s", v.Host, strings.Join(v.States, "→"))
	if v.Exact {
		title = fmt.Sprintf("%s percorre exatamente %s", v.Host, strings.Join(v.States, "→"))
	}
	p, ok := run.Path(v.Host, v.Machine)
	if !ok {
		return Check{Title: title, Detail: "máquina não encontrada"}
	}
	observed := strings.Join(p.States, "→")
	passed := slices.Equal(p.States, v.States)
	if !v.Exact {
		passed = containsRun(p.States, v.States)
	}
	return Check{Title: title, Passed: passed, Detail: "observado: " + observed}
}

func containsRun(haystack, needle []string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if slices.Equal(haystack[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

func judgeCount(m RecordMatch, e Expectation, run Run) Check {
	n := 0
	for _, r := range run.Records {
		if m.Matches(r) {
			n++
		}
	}
	var bounds []string
	passed := true
	if e.Eq != nil {
		bounds = append(bounds, fmt.Sprintf("= %d", *e.Eq))
		passed = passed && n == *e.Eq
	}
	if e.AtLeast != nil {
		bounds = append(bounds, fmt.Sprintf("≥ %d", *e.AtLeast))
		passed = passed && n >= *e.AtLeast
	}
	if e.AtMost != nil {
		bounds = append(bounds, fmt.Sprintf("≤ %d", *e.AtMost))
		passed = passed && n <= *e.AtMost
	}
	return Check{Title: fmt.Sprintf("#%s %s", m, strings.Join(bounds, " e ")), Passed: passed, Detail: fmt.Sprintf("observado: %d", n)}
}

func resolveMetric(ref string, run Run, runs map[string]Run) (float64, bool) {
	if runName, name, qualified := strings.Cut(ref, ":"); qualified {
		other, ok := runs[runName]
		if !ok {
			return 0, false
		}
		return other.Stats.Metric(name)
	}
	return run.Stats.Metric(ref)
}

func judgeMetric(m MetricCheck, run Run, runs map[string]Run) Check {
	value, ok := resolveMetric(m.Name, run, runs)
	if !ok {
		return Check{Title: m.Name, Detail: "métrica desconhecida"}
	}
	var conditions []string
	passed := true
	compare := func(symbol string, target float64, holds bool) {
		conditions = append(conditions, symbol+" "+format(target))
		passed = passed && holds
	}
	if m.Eq != nil {
		compare("=", *m.Eq, math.Abs(value-*m.Eq) < 1e-9)
	}
	if m.Min != nil {
		compare("≥", *m.Min, value >= *m.Min)
	}
	if m.Max != nil {
		compare("≤", *m.Max, value <= *m.Max)
	}
	if m.Gt != nil {
		compare(">", *m.Gt, value > *m.Gt)
	}
	if m.Lt != nil {
		compare("<", *m.Lt, value < *m.Lt)
	}
	relate := func(symbol, ref string, holds func(a, b float64) bool) {
		other, ok := resolveMetric(ref, run, runs)
		conditions = append(conditions, symbol+" "+ref+" ("+format(other)+")")
		passed = passed && ok && holds(value, other)
	}
	if m.SameAs != "" {
		relate("=", m.SameAs, func(a, b float64) bool { return math.Abs(a-b) < 1e-9 })
	}
	if m.Above != "" {
		relate(">", m.Above, func(a, b float64) bool { return a > b })
	}
	if m.Below != "" {
		relate("<", m.Below, func(a, b float64) bool { return a < b })
	}
	return Check{Title: m.Name + " " + strings.Join(conditions, " e "), Passed: passed, Detail: "observado: " + format(value)}
}

func format(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func judgeOrder(o OrderCheck, run Run) Check {
	title := fmt.Sprintf("%s antes de %s", o.First, o.Then)
	first, then := -1, -1
	for i, r := range run.Records {
		if first < 0 && o.First.Matches(r) {
			first = i
		}
		if then < 0 && o.Then.Matches(r) {
			then = i
		}
	}
	switch {
	case first < 0:
		return Check{Title: title, Detail: "o primeiro evento nunca ocorreu"}
	case then < 0:
		return Check{Title: title, Passed: true, Detail: fmt.Sprintf("primeiro em %v; o segundo nunca ocorreu", run.Records[first].At)}
	default:
		return Check{Title: title, Passed: first < then, Detail: fmt.Sprintf("primeiro em %v, segundo em %v", run.Records[first].At, run.Records[then].At)}
	}
}

func judgeDwell(d DwellCheck, run Run) Check {
	title := fmt.Sprintf("%s permanece em %s", d.Host, d.State)
	var entered, left sim.VirtualTime = -1, -1
	for _, r := range run.Records {
		if !r.ChangesState() || r.Host != d.Host || (d.Machine != "" && r.Machine != d.Machine) {
			continue
		}
		if entered < 0 && r.To == d.State {
			entered = r.At
			continue
		}
		if entered >= 0 && r.From == d.State {
			left = r.At
			break
		}
	}
	if entered < 0 {
		return Check{Title: title, Detail: "o estado nunca foi visitado"}
	}
	if left < 0 {
		left = run.End
	}
	dwell := left - entered
	passed := true
	var conditions []string
	if d.Eq != nil {
		conditions = append(conditions, "= "+d.Eq.String())
		passed = passed && dwell == *d.Eq
	}
	if d.Min != nil {
		conditions = append(conditions, "≥ "+d.Min.String())
		passed = passed && dwell >= *d.Min
	}
	if d.Max != nil {
		conditions = append(conditions, "≤ "+d.Max.String())
		passed = passed && dwell <= *d.Max
	}
	return Check{Title: title + " " + strings.Join(conditions, " e "), Passed: passed, Detail: fmt.Sprintf("observado: %v (de %v a %v)", dwell, entered, left)}
}
