package scenario

import (
	"fmt"
	"io"
	"strings"

	"bancada/internal/sim"
)

func FormatRecord(r sim.Record) string {
	head := fmt.Sprintf("%10v  %-2s %-11s", r.At, r.Host, r.Kind)
	switch r.Kind {
	case sim.KindTransition:
		return fmt.Sprintf("%s %s: %s --%s--> %s  / %s", head, r.Machine, r.From, eventWithGuard(r), r.To, strings.Join(r.Actions, "; "))
	case sim.KindRejection:
		return fmt.Sprintf("%s %s: %s em %s rejeitado: %s", head, r.Machine, r.Event, r.From, r.Reason)
	case sim.KindSend, sim.KindArrival, sim.KindLoss:
		f := r.Flight
		var tags []string
		if f.Header.Retransmission {
			tags = append(tags, "retransmissão "+f.Header.Reason)
		}
		if f.Corrupted {
			tags = append(tags, fmt.Sprintf("corrompido bits %v", f.Flipped))
		}
		if f.Duplicate {
			tags = append(tags, "cópia do canal")
		}
		if f.Reordered {
			tags = append(tags, "reordenado")
		}
		if f.Fate == sim.FateLost {
			tags = append(tags, "perdido ("+f.Cause+")")
		}
		return fmt.Sprintf("%s #%d %s→%s %s  %s", head, f.ID, f.Src, f.Dst, f.Header.Label, strings.Join(tags, ", "))
	case sim.KindTimerArm, sim.KindTimerStop:
		return fmt.Sprintf("%s %s até %v", head, r.Timer, r.Deadline)
	case sim.KindTimeout:
		return fmt.Sprintf("%s %s", head, r.Timer)
	case sim.KindDelivery:
		return fmt.Sprintf("%s mensagem #%d (%s)", head, r.Message, r.Verdict)
	case sim.KindMetric:
		return fmt.Sprintf("%s %s", head, r.Detail)
	default:
		return fmt.Sprintf("%s %s %s %s", head, r.Event, r.Reason, r.Detail)
	}
}

func eventWithGuard(r sim.Record) string {
	if r.Guard == "" {
		return r.Event
	}
	return r.Event + " [" + r.Guard + "]"
}

func WriteTrace(w io.Writer, run Run) {
	fmt.Fprintf(w, "execução %s · protocolo %s · seed %d · %d passos\n", run.Name, run.Protocol, run.Seed, run.Steps)
	for _, r := range run.Records {
		fmt.Fprintln(w, FormatRecord(r))
	}
}
