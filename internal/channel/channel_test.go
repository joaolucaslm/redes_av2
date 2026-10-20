package channel

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"bancada/internal/sim"
	"bancada/internal/wire"
)

func packet(src string, flags string, payload int) Packet {
	return Packet{
		Src:    src,
		Dst:    map[string]string{"A": "B", "B": "A"}[src],
		Frame:  wire.Frame{Proto: wire.ProtoTCP, Bytes: make([]byte, 20+payload)},
		Header: sim.Header{Proto: "tcp", Flags: flags, Payload: payload},
	}
}

func mustChannel(t *testing.T, p Params, seed uint64) *Channel {
	t.Helper()
	c, err := New(p, sim.NewStreams(seed))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSameSeedSameFaults(t *testing.T) {
	p := Params{Loss: 0.3, DelayMin: 10 * sim.Millisecond, DelayMax: 90 * sim.Millisecond, Corrupt: 0.2, CorruptBits: 2, Duplicate: 0.1, Reorder: 0.1, ReorderLag: 50 * sim.Millisecond}
	run := func() [][]Transit {
		c := mustChannel(t, p, 99)
		var out [][]Transit
		for i := range 200 {
			out = append(out, c.Transmit(sim.VirtualTime(i)*sim.Millisecond, packet("A", "ACK", 100)))
		}
		return out
	}
	if !reflect.DeepEqual(run(), run()) {
		t.Fatal("mesma seed deveria produzir exatamente as mesmas falhas")
	}
}

func TestChangingLossDoesNotReshuffleDelays(t *testing.T) {
	base := Params{DelayMin: 10 * sim.Millisecond, DelayMax: 200 * sim.Millisecond, CorruptBits: 1, Reorder: 1, ReorderLag: 0}
	lossy := base
	lossy.Loss = 0.5
	clean, noisy := mustChannel(t, base, 7), mustChannel(t, lossy, 7)
	compared := 0
	for i := range 300 {
		now := sim.VirtualTime(i) * sim.Millisecond
		a := clean.Transmit(now, packet("A", "ACK", 10))[0]
		b := noisy.Transmit(now, packet("A", "ACK", 10))[0]
		if b.Flight.Fate == sim.FateDelivered {
			compared++
			if a.Flight.EndAt != b.Flight.EndAt {
				t.Fatalf("pacote %d: atraso mudou de %v para %v só por alterar a perda", i, a.Flight.EndAt, b.Flight.EndAt)
			}
		}
	}
	if compared < 100 {
		t.Fatalf("poucos pacotes comparados: %d", compared)
	}
}

func TestLossRateConverges(t *testing.T) {
	c := mustChannel(t, Params{Loss: 0.3, CorruptBits: 1}, 2024)
	lost := 0
	const n = 20000
	for i := range n {
		if c.Transmit(sim.VirtualTime(i), packet("A", "", 1))[0].Flight.Fate == sim.FateLost {
			lost++
		}
	}
	if rate := float64(lost) / n; math.Abs(rate-0.3) > 0.02 {
		t.Fatalf("taxa de perda observada %.3f, esperada ~0.30", rate)
	}
}

func TestDropNextIsOneShotAndDirectional(t *testing.T) {
	c := mustChannel(t, DefaultParams(), 1)
	c.ArmDropNext("B")
	if c.Transmit(0, packet("A", "SYN", 0))[0].Flight.Fate != sim.FateDelivered {
		t.Fatal("pacote de A não deveria ser afetado pelo descarte armado para B")
	}
	first := c.Transmit(0, packet("B", "SYN|ACK", 0))[0].Flight
	second := c.Transmit(0, packet("B", "ACK", 0))[0].Flight
	if first.Fate != sim.FateLost || first.Cause != CauseManual || second.Fate != sim.FateDelivered {
		t.Fatalf("descarte manual incorreto: %+v / %+v", first, second)
	}
	if first.EndAt <= first.SentAt || first.EndAt-first.SentAt > DefaultParams().DelayMax/2 {
		t.Fatalf("perda deveria ocorrer no meio do canal: %+v", first)
	}
}

func TestRulesHitTheNthMatch(t *testing.T) {
	data := true
	c := mustChannel(t, DefaultParams(), 1)
	if err := c.AddRule(Rule{Action: ActionDrop, Match: Selector{From: "A", Data: &data}, Nth: 3}); err != nil {
		t.Fatal(err)
	}
	var fates []sim.Fate
	for _, p := range []Packet{packet("A", "SYN", 0), packet("A", "ACK", 100), packet("B", "ACK", 100), packet("A", "ACK", 100), packet("A", "ACK", 100), packet("A", "ACK", 100)} {
		fates = append(fates, c.Transmit(0, p)[0].Flight.Fate)
	}
	want := []sim.Fate{sim.FateDelivered, sim.FateDelivered, sim.FateDelivered, sim.FateDelivered, sim.FateLost, sim.FateDelivered}
	if !reflect.DeepEqual(fates, want) {
		t.Fatalf("destinos = %v, quero %v", fates, want)
	}
}

func TestCorruptionFlipsDistinctBits(t *testing.T) {
	c := mustChannel(t, Params{Corrupt: 1, CorruptBits: 3}, 5)
	p := packet("A", "ACK", 40)
	got := c.Transmit(0, p)[0]
	if !got.Flight.Corrupted || len(got.Flight.Flipped) != 3 {
		t.Fatalf("esperava 3 bits trocados: %+v", got.Flight)
	}
	diff := 0
	for i := range got.Frame.Bytes {
		for b := got.Frame.Bytes[i] ^ p.Frame.Bytes[i]; b != 0; b &= b - 1 {
			diff++
		}
	}
	if diff != 3 {
		t.Fatalf("bits diferentes = %d, quero 3", diff)
	}
	if p.Frame.Bytes[0] != 0 {
		t.Fatal("o quadro original não pode ser alterado")
	}
}

func TestFIFOUnlessReordered(t *testing.T) {
	c := mustChannel(t, Params{DelayMin: 0, DelayMax: 100 * sim.Millisecond, CorruptBits: 1}, 3)
	last := sim.VirtualTime(-1)
	for i := range 100 {
		end := c.Transmit(sim.VirtualTime(i)*sim.Millisecond, packet("A", "ACK", 1))[0].Flight.EndAt
		if end <= last {
			t.Fatalf("pacote %d chegou antes do anterior (%v <= %v)", i, end, last)
		}
		last = end
	}
	r := mustChannel(t, Params{Reorder: 1, ReorderLag: 30 * sim.Millisecond, CorruptBits: 1}, 3)
	if !r.Transmit(0, packet("A", "ACK", 1))[0].Flight.Reordered {
		t.Fatal("reordem com probabilidade 1 deveria marcar o voo")
	}
}

func TestDuplicateProducesPristineCopy(t *testing.T) {
	c := mustChannel(t, Params{Duplicate: 1, Corrupt: 1, CorruptBits: 1, DelayMin: 10 * sim.Millisecond, DelayMax: 10 * sim.Millisecond}, 8)
	out := c.Transmit(0, packet("A", "ACK", 4))
	if len(out) != 2 || !out[1].Flight.Duplicate || out[1].Flight.Corrupted || out[1].Flight.EndAt <= out[0].Flight.EndAt {
		t.Fatalf("duplicação incorreta: %+v", out)
	}
}

func TestParamsValidation(t *testing.T) {
	tests := []struct {
		name string
		p    Params
		ok   bool
	}{
		{"padrão", DefaultParams(), true},
		{"perda acima de 1", Params{Loss: 1.5, CorruptBits: 1}, false},
		{"atraso invertido", Params{DelayMin: 10, DelayMax: 5, CorruptBits: 1}, false},
		{"bits zerados", Params{}, false},
		{"NaN", Params{Duplicate: math.NaN(), CorruptBits: 1}, false},
	}
	for _, tc := range tests {
		err := tc.p.Validate()
		if (err == nil) != tc.ok || (err != nil && !errors.Is(err, ErrInvalidParams)) {
			t.Errorf("%s: Validate = %v", tc.name, err)
		}
	}
	if err := (Rule{Action: "explodir", Nth: 1}).Validate(); !errors.Is(err, ErrInvalidRule) {
		t.Errorf("regra inválida aceita: %v", err)
	}
}
