package tcp

import (
	"math"

	"bancada/internal/sim"
)

const maxBackoff = 16

type RTTEstimator struct {
	alpha     float64
	beta      float64
	estimated float64
	deviation float64
	samples   int
	last      sim.VirtualTime
	initial   sim.VirtualTime
	floor     sim.VirtualTime
	ceiling   sim.VirtualTime
	backoff   int
}

func NewRTTEstimator(cfg Config) RTTEstimator {
	return RTTEstimator{
		alpha:   cfg.Alpha,
		beta:    cfg.Beta,
		initial: cfg.InitialRTO,
		floor:   cfg.MinRTO,
		ceiling: cfg.MaxRTO,
	}
}

func (r *RTTEstimator) Observe(sample sim.VirtualTime) {
	s := float64(sample)
	if r.samples == 0 {
		r.estimated = s
		r.deviation = s / 2
	} else {
		r.deviation = (1-r.beta)*r.deviation + r.beta*math.Abs(s-r.estimated)
		r.estimated = (1-r.alpha)*r.estimated + r.alpha*s
	}
	r.samples++
	r.last = sample
	r.backoff = 0
}

func (r *RTTEstimator) TimeoutInterval() sim.VirtualTime {
	interval := r.initial
	if r.samples > 0 {
		interval = sim.VirtualTime(math.Round(r.estimated + 4*r.deviation))
	}
	interval = min(max(interval, r.floor), r.ceiling)
	for range r.backoff {
		interval = min(interval*2, r.ceiling)
	}
	return interval
}

func (r *RTTEstimator) Backoff() {
	r.backoff = min(r.backoff+1, maxBackoff)
}

func (r *RTTEstimator) ResetBackoff() { r.backoff = 0 }

func (r *RTTEstimator) Samples() int { return r.samples }

func (r *RTTEstimator) LastSample() sim.VirtualTime { return r.last }

func (r *RTTEstimator) Estimated() sim.VirtualTime {
	return sim.VirtualTime(math.Round(r.estimated))
}

func (r *RTTEstimator) Deviation() sim.VirtualTime {
	return sim.VirtualTime(math.Round(r.deviation))
}
