package sim

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestSchedulerOrdersByTimeThenInsertion(t *testing.T) {
	s := NewScheduler()
	var fired []string
	record := func(name string) func() { return func() { fired = append(fired, name) } }
	s.At(30*Millisecond, "c", record("c"))
	s.At(10*Millisecond, "a", record("a"))
	s.At(10*Millisecond, "b", record("b"))
	cancelled := s.At(20*Millisecond, "x", record("x"))
	if !s.Cancel(cancelled) || s.Cancel(cancelled) {
		t.Fatal("Cancel deveria funcionar exatamente uma vez")
	}
	for {
		occ, ok := s.Next()
		if !ok {
			break
		}
		occ.Fire()
	}
	if !slices.Equal(fired, []string{"a", "b", "c"}) {
		t.Fatalf("ordem = %v", fired)
	}
	if s.Now() != 30*Millisecond {
		t.Fatalf("Now = %v", s.Now())
	}
}

func TestSchedulerNeverGoesBackInTime(t *testing.T) {
	s := NewScheduler()
	s.At(50*Millisecond, "", nil)
	s.Next()
	id := s.At(10*Millisecond, "", nil)
	at, ok := s.Deadline(id)
	if !ok || at != 50*Millisecond {
		t.Fatalf("prazo = %v, quero 50ms", at)
	}
}

func TestVirtualTimeJSON(t *testing.T) {
	tests := []struct {
		raw  string
		want VirtualTime
	}{
		{`"40ms"`, 40 * Millisecond},
		{`"1.5s"`, 1500 * Millisecond},
		{`"250us"`, 250 * Microsecond},
		{`1200`, 1200 * Microsecond},
	}
	for _, tc := range tests {
		var got VirtualTime
		if err := json.Unmarshal([]byte(tc.raw), &got); err != nil || got != tc.want {
			t.Errorf("Unmarshal(%s) = %v, %v; quero %v", tc.raw, got, err, tc.want)
		}
	}
	var bad VirtualTime
	if err := json.Unmarshal([]byte(`"rápido"`), &bad); err == nil {
		t.Error("esperava erro para duração inválida")
	}
}

func TestStreamsAreIndependentAndReproducible(t *testing.T) {
	a := NewStreams(42)
	b := NewStreams(42)
	if a.Stream("loss").Uint64() != b.Stream("loss").Uint64() {
		t.Fatal("mesma seed e mesmo nome devem produzir a mesma sequência")
	}
	if a.Stream("loss").Uint64() == a.Stream("delay").Uint64() {
		t.Fatal("streams com nomes diferentes deveriam divergir")
	}
	if NewStreams(1).Stream("loss").Uint64() == NewStreams(2).Stream("loss").Uint64() {
		t.Fatal("seeds diferentes deveriam divergir")
	}
}

func TestBusStampsStepAndTime(t *testing.T) {
	s := NewScheduler()
	bus := NewBus(s)
	var seen []Kind
	bus.Subscribe(func(r Record) { seen = append(seen, r.Kind) })
	s.At(7*Millisecond, "", nil)
	s.Next()
	bus.SetStep(3)
	actions := []string{"x"}
	r := bus.Publish(Record{Kind: KindTransition, Actions: actions})
	actions[0] = "mutado"
	if r.Step != 3 || r.At != 7*Millisecond || r.Seq != 0 {
		t.Fatalf("registro mal carimbado: %+v", r)
	}
	if bus.Records()[0].Actions[0] != "x" {
		t.Fatal("o barramento deveria guardar uma cópia das ações")
	}
	if len(seen) != 1 || len(bus.Since(1)) != 0 {
		t.Fatal("inscrição ou Since incorretos")
	}
}
