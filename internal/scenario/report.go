package scenario

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"text/tabwriter"

	"bancada/internal/sim"
)

var reportMetrics = []struct {
	label string
	name  string
}{
	{"mensagens oferecidas", "offered"},
	{"entregues (únicas)", "delivered"},
	{"perdidas para a aplicação", "missing"},
	{"duplicadas entregues à aplicação", "duplicates"},
	{"entregues em ordem", "inOrder"},
	{"entregues fora de ordem", "outOfOrder"},
	{"segmentos enviados", "sent"},
	{"retransmissões", "retransmissions"},
	{"retransmissões rápidas", "fastRetransmits"},
	{"timeouts", "timeouts"},
	{"perdidos no canal", "lost"},
	{"corrompidos no canal", "corrupted"},
	{"duplicados pelo canal", "duplicated"},
	{"descartados pelo receptor", "discarded"},
	{"bytes no fio", "bytesOnWire"},
	{"tempo de transferência: 1ª oferta → última entrega (ms)", "totalTimeMs"},
	{"tempo total desde t=0, com handshake (ms)", "lastDeliveryMs"},
	{"economia da retransmissão rápida (ms)", "fastRetransmitSavedMs"},
}

func ProtocolLabel(r Result) string {
	var seen []string
	for _, p := range r.Protocols {
		upper := strings.ToUpper(p)
		if !slices.Contains(seen, upper) {
			seen = append(seen, upper)
		}
	}
	return strings.Join(seen, " vs ")
}

func Verdict(passed bool) string {
	if passed {
		return "APROVADO"
	}
	return "REPROVADO"
}

func virtualSpan(r Result) sim.VirtualTime {
	var longest sim.VirtualTime
	for _, run := range r.Runs {
		longest = max(longest, run.End)
	}
	return longest
}

func WriteTable(w io.Writer, results []Result) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tPROTOCOLO\tCENÁRIO\tRESULTADO\tVERIFICAÇÕES\tSEED\tTEMPO VIRTUAL")
	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d/%d\t%d\t%v\n", r.Number, ProtocolLabel(r), r.Title, Verdict(r.Passed), r.PassedChecks(), len(r.Checks), r.Seed, virtualSpan(r))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "\n%d de %d cenários aprovados\n", passed, len(results))
	return err
}

func WriteFailures(w io.Writer, results []Result) {
	for _, r := range results {
		if r.Passed {
			continue
		}
		fmt.Fprintf(w, "\n✘ %s %s\n", r.Number, r.Title)
		if r.Error != "" {
			fmt.Fprintf(w, "    erro: %s\n", r.Error)
		}
		for _, c := range r.Checks {
			if !c.Passed {
				fmt.Fprintf(w, "    %s [%s] %s\n", c.Title, c.Run, c.Detail)
			}
		}
	}
}

func WriteJSON(w io.Writer, results []Result) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(results)
}

func WriteMarkdown(w io.Writer, results []Result) error {
	var b strings.Builder
	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	b.WriteString("# Relatório automático dos cenários\n\n")
	fmt.Fprintf(&b, "Gerado por `go run ./cmd/scenarios`. Todos os tempos são virtuais. **%d de %d cenários aprovados.**\n\n", passed, len(results))
	b.WriteString("| # | Protocolo | Cenário | Resultado | Verificações | Seed | Tempo virtual |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, r := range results {
		fmt.Fprintf(&b, "| %s | %s | %s | **%s** | %d/%d | %d | %v |\n", r.Number, ProtocolLabel(r), r.Title, Verdict(r.Passed), r.PassedChecks(), len(r.Checks), r.Seed, virtualSpan(r))
	}
	for _, r := range results {
		writeScenario(&b, r)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeScenario(b *strings.Builder, r Result) {
	fmt.Fprintf(b, "\n## %s · %s — %s\n\n", r.Number, r.Title, Verdict(r.Passed))
	if r.Expected != "" {
		fmt.Fprintf(b, "**Esperado (enunciado):** %s\n\n", r.Expected)
	}
	if r.Description != "" {
		fmt.Fprintf(b, "%s\n\n", r.Description)
	}
	if r.Error != "" {
		fmt.Fprintf(b, "**Erro ao preparar:** `%s`\n\n", r.Error)
		return
	}
	b.WriteString("| Verificação | Execução | Resultado | Detalhe |\n|---|---|---|---|\n")
	for _, c := range r.Checks {
		mark := "✔"
		if !c.Passed {
			mark = "✘"
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", escape(c.Title), c.Run, mark, escape(c.Detail))
	}
	b.WriteString("\n**Caminhos de estados:**\n\n")
	for _, run := range r.Runs {
		for _, p := range run.Paths {
			label := p.Host + "/" + p.Machine
			if len(r.Runs) > 1 {
				label = run.Name + " · " + label
			}
			fmt.Fprintf(b, "- `%s`: %s\n", label, strings.Join(p.States, " → "))
		}
	}
	b.WriteString("\n| Métrica |")
	for _, run := range r.Runs {
		fmt.Fprintf(b, " %s |", run.Name)
	}
	b.WriteString("\n|---|" + strings.Repeat("---|", len(r.Runs)) + "\n")
	for _, m := range reportMetrics {
		fmt.Fprintf(b, "| %s |", m.label)
		for _, run := range r.Runs {
			value, _ := run.Stats.Metric(m.name)
			fmt.Fprintf(b, " %s |", format(roundTo(value, 3)))
		}
		b.WriteString("\n")
	}
}

func roundTo(v float64, digits int) float64 {
	scale := math.Pow(10, float64(digits))
	return math.Round(v*scale) / scale
}

func escape(text string) string {
	return strings.ReplaceAll(text, "|", "\\|")
}
