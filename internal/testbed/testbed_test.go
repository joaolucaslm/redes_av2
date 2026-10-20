package testbed

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	fsmfiles "bancada/fsm"
	"bancada/internal/fsm"
	"bancada/internal/host"
	"bancada/internal/sim"
)

func catalog(t *testing.T) *fsm.Catalog {
	t.Helper()
	cat, err := LoadCatalog(fsmfiles.Files)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	return cat
}

func run(t *testing.T, tb *Testbed, limit int) {
	t.Helper()
	for range limit {
		if _, ok := tb.Step(); !ok {
			return
		}
	}
	t.Fatalf("simulação não terminou em %d passos", limit)
}

func dump(records []sim.Record) string {
	var b strings.Builder
	for _, r := range records {
		switch r.Kind {
		case sim.KindTransition:
			fmt.Fprintf(&b, "%8v %s %-10s %s --%s--> %s %v\n", r.At, r.Host, r.Machine, r.From, r.Event, r.To, r.Actions)
		case sim.KindSend, sim.KindLoss, sim.KindArrival:
			fmt.Fprintf(&b, "%8v %s %-10s %s %s\n", r.At, r.Host, r.Kind, r.Flight.Header.Label, r.Flight.Fate)
		default:
			fmt.Fprintf(&b, "%8v %s %-10s %s %s %s %s\n", r.At, r.Host, r.Kind, r.Event, r.Timer, r.Reason, r.Detail)
		}
	}
	return b.String()
}

func path(records []sim.Record, hostName, machine string) []string {
	var states []string
	for _, r := range records {
		if r.ChangesState() && r.Host == hostName && r.Machine == machine {
			if len(states) == 0 {
				states = append(states, r.From)
			}
			states = append(states, r.To)
		}
	}
	return states
}

func TestCatalogExposesEveryRunnableProtocol(t *testing.T) {
	cat := catalog(t)
	var names []string
	for _, f := range Runnable(cat) {
		names = append(names, f.Protocol)
	}
	for _, want := range []string{"udp", "tcp", "rdt-2.1", "rdt-3.0", "tcp-sem-time-wait"} {
		if !slices.Contains(names, want) {
			t.Errorf("protocolo %s ausente em %v", want, names)
		}
	}
}

func TestCatalogRejectsUnregisteredVocabulary(t *testing.T) {
	broken := fstest.MapFS{"x.json": {Data: []byte(`{"protocol":"x","family":"udp","machines":[{"name":"socket","initial":"S",
		"states":[{"id":"S"}],"transitions":[{"from":"S","event":"send","actions":["teleportar"],"to":"S"}]}]}`)}}
	if _, err := LoadCatalog(broken); !errors.Is(err, fsm.ErrUnknownAction) {
		t.Fatalf("esperava ErrUnknownAction, veio %v", err)
	}
}

func TestTCPFullConversation(t *testing.T) {
	cfg := DefaultConfig("tcp")
	cfg.Seed = 3
	tb, err := New(catalog(t), cfg)
	if err != nil {
		t.Fatal(err)
	}
	tb.Schedule(0, AppCommand("B", host.Command{Verb: host.VerbListen}))
	tb.Schedule(sim.Millisecond, AppCommand("A", host.Command{Verb: host.VerbConnect}))
	must(t, tb.When(Trigger{Host: "A", State: "ESTABLISHED"}, 0, AppCommand("A", host.Command{Verb: host.VerbSend, Count: 12})))
	must(t, tb.When(Trigger{Host: "A", Drained: true}, 0, AppCommand("A", host.Command{Verb: host.VerbClose})))
	must(t, tb.When(Trigger{Host: "B", State: "CLOSE_WAIT"}, 10*sim.Millisecond, AppCommand("B", host.Command{Verb: host.VerbClose})))
	run(t, tb, 10000)

	records := tb.Records()
	stats := tb.Stats()
	if stats.Delivered != 12 || stats.Duplicates != 0 || stats.OutOfOrder != 0 || stats.Rejected != 0 {
		t.Fatalf("estatísticas inesperadas %+v\n%s", stats, dump(records))
	}
	wantA := []string{"CLOSED", "SYN_SENT", "ESTABLISHED", "FIN_WAIT_1", "FIN_WAIT_2", "TIME_WAIT", "CLOSED"}
	wantB := []string{"CLOSED", "LISTEN", "SYN_RCVD", "ESTABLISHED", "CLOSE_WAIT", "LAST_ACK", "CLOSED"}
	if got := path(records, "A", "connection"); !slices.Equal(got, wantA) {
		t.Fatalf("caminho de A = %v\n%s", got, dump(records))
	}
	if got := path(records, "B", "connection"); !slices.Equal(got, wantB) {
		t.Fatalf("caminho de B = %v", got)
	}
	if stats.RTTSamples == 0 {
		t.Fatal("nenhuma amostra de RTT foi colhida")
	}
}

func TestSnapshotIsDetachedFromLiveState(t *testing.T) {
	tb, err := New(catalog(t), DefaultConfig("tcp"))
	if err != nil {
		t.Fatal(err)
	}
	tb.Schedule(0, AppCommand("B", host.Command{Verb: host.VerbListen}))
	tb.Schedule(0, AppCommand("A", host.Command{Verb: host.VerbConnect}))
	tb.Step()
	before := tb.Snapshot()
	frozen, _ := json.Marshal(before)
	run(t, tb, 1000)
	again, _ := json.Marshal(before)
	if string(frozen) != string(again) {
		t.Fatal("o snapshot mudou depois que a simulação avançou")
	}
}

func TestSameSeedSameHistory(t *testing.T) {
	history := func(seed uint64) string {
		cfg := DefaultConfig("tcp")
		cfg.Seed = seed
		cfg.Channel.Loss = 0.2
		cfg.Channel.Duplicate = 0.1
		cfg.Channel.Corrupt = 0.1
		tb, err := New(catalog(t), cfg)
		if err != nil {
			t.Fatal(err)
		}
		tb.Schedule(0, AppCommand("B", host.Command{Verb: host.VerbListen}))
		tb.Schedule(0, AppCommand("A", host.Command{Verb: host.VerbConnect}))
		must(t, tb.When(Trigger{Host: "A", State: "ESTABLISHED"}, 0, AppCommand("A", host.Command{Verb: host.VerbSend, Count: 20})))
		run(t, tb, 100000)
		out, _ := json.Marshal(tb.Records())
		return string(out)
	}
	if history(11) != history(11) {
		t.Fatal("mesma seed produziu históricos diferentes")
	}
	if history(11) == history(12) {
		t.Fatal("seeds diferentes deveriam produzir históricos diferentes")
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
