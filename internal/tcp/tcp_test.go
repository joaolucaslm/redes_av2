package tcp

import (
	"errors"
	"testing"

	"bancada/internal/sim"
	"bancada/internal/wire"
)

func TestSeqArithmeticWrapsAround(t *testing.T) {
	tests := []struct {
		a, b   Seq
		before bool
		minus  int
	}{
		{1, 2, true, -1},
		{0xfffffff0, 0x10, true, -0x20},
		{0x10, 0xfffffff0, false, 0x20},
		{7, 7, false, 0},
	}
	for _, tc := range tests {
		if got := tc.a.Before(tc.b); got != tc.before {
			t.Errorf("%d.Before(%d) = %v", tc.a, tc.b, got)
		}
		if got := tc.a.Minus(tc.b); got != tc.minus {
			t.Errorf("%d.Minus(%d) = %d, quero %d", tc.a, tc.b, got, tc.minus)
		}
	}
	if Seq(0xffffffff).Add(2) != 1 {
		t.Error("Add deveria dar a volta no espaço de 32 bits")
	}
}

func TestRTTEstimatorMatchesSection353(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinRTO = sim.Millisecond
	r := NewRTTEstimator(cfg)
	if r.TimeoutInterval() != sim.Second {
		t.Fatalf("antes de qualquer amostra o TimeoutInterval deve ser 1 s, veio %v", r.TimeoutInterval())
	}
	steps := []struct {
		sample            sim.VirtualTime
		estimated, devRTT sim.VirtualTime
		timeout           sim.VirtualTime
	}{
		{100 * sim.Millisecond, 100 * sim.Millisecond, 50 * sim.Millisecond, 300 * sim.Millisecond},
		{140 * sim.Millisecond, 105 * sim.Millisecond, 47500 * sim.Microsecond, 295 * sim.Millisecond},
		{60 * sim.Millisecond, 99375 * sim.Microsecond, 46875 * sim.Microsecond, 286875 * sim.Microsecond},
	}
	for i, s := range steps {
		r.Observe(s.sample)
		if r.Estimated() != s.estimated || r.Deviation() != s.devRTT || r.TimeoutInterval() != s.timeout {
			t.Fatalf("amostra %d: Estimated=%v Dev=%v Timeout=%v; quero %v %v %v", i, r.Estimated(), r.Deviation(), r.TimeoutInterval(), s.estimated, s.devRTT, s.timeout)
		}
	}
	r.Backoff()
	r.Backoff()
	if r.TimeoutInterval() != 4*286875*sim.Microsecond {
		t.Fatalf("duas expirações deveriam quadruplicar o intervalo, veio %v", r.TimeoutInterval())
	}
	r.ResetBackoff()
	if r.TimeoutInterval() != 286875*sim.Microsecond {
		t.Fatal("ResetBackoff deveria voltar ao intervalo derivado das estimativas")
	}
}

func TestRTOIsClampedToBounds(t *testing.T) {
	cfg := DefaultConfig()
	r := NewRTTEstimator(cfg)
	r.Observe(sim.Millisecond)
	if r.TimeoutInterval() != cfg.MinRTO {
		t.Fatalf("RTO abaixo do mínimo: %v", r.TimeoutInterval())
	}
	for range 30 {
		r.Backoff()
	}
	if r.TimeoutInterval() != cfg.MaxRTO {
		t.Fatalf("RTO acima do máximo: %v", r.TimeoutInterval())
	}
}

func TestClassifySegments(t *testing.T) {
	tests := []struct {
		flags   wire.Flags
		payload int
		want    string
	}{
		{wire.FlagSYN, 0, "rcv.SYN"},
		{wire.FlagSYN | wire.FlagACK, 0, "rcv.SYNACK"},
		{wire.FlagACK, 0, "rcv.ACK"},
		{wire.FlagACK, 10, "rcv.data"},
		{wire.FlagFIN | wire.FlagACK, 0, "rcv.FIN"},
		{wire.FlagRST | wire.FlagACK, 0, "rcv.RST"},
		{0, 0, "rcv.?"},
	}
	for _, tc := range tests {
		got := Classify(wire.Segment{Flags: tc.flags, Payload: make([]byte, tc.payload)})
		if string(got) != tc.want {
			t.Errorf("Classify(%s, %d B) = %s, quero %s", tc.flags, tc.payload, got, tc.want)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	bad := DefaultConfig()
	bad.MSS = 0
	bad.Alpha = 2
	if err := bad.Validate(); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("esperava ErrInvalidConfig, veio %v", err)
	}
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}
