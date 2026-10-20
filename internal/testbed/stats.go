package testbed

import (
	"maps"
	"slices"

	"bancada/internal/sim"
	"bancada/internal/workload"
)

type Stats struct {
	Offered             int             `json:"offered"`
	Delivered           int             `json:"delivered"`
	InOrder             int             `json:"inOrder"`
	OutOfOrder          int             `json:"outOfOrder"`
	Duplicates          int             `json:"duplicates"`
	Missing             int             `json:"missing"`
	Sent                int             `json:"sent"`
	BytesOnWire         int             `json:"bytesOnWire"`
	Retransmissions     int             `json:"retransmissions"`
	FastRetransmits     int             `json:"fastRetransmits"`
	Timeouts            int             `json:"timeouts"`
	Lost                int             `json:"lost"`
	Corrupted           int             `json:"corrupted"`
	Duplicated          int             `json:"duplicated"`
	Reordered           int             `json:"reordered"`
	Discarded           int             `json:"discarded"`
	Rejected            int             `json:"rejected"`
	Transitions         int             `json:"transitions"`
	RTTSamples          int             `json:"rttSamples"`
	FirstOfferAt        sim.VirtualTime `json:"firstOfferAt"`
	LastDeliveryAt      sim.VirtualTime `json:"lastDeliveryAt"`
	TotalTime           sim.VirtualTime `json:"totalTime"`
	FastRetransmitSaved sim.VirtualTime `json:"fastRetransmitSaved"`
}

func (s *Stats) Observe(r sim.Record) {
	switch r.Kind {
	case sim.KindApp:
		if r.Event == "offer" {
			if s.Offered == 0 {
				s.FirstOfferAt = r.At
			}
			s.Offered++
		}
	case sim.KindDelivery:
		s.LastDeliveryAt = r.At
		switch workload.Verdict(r.Verdict) {
		case workload.VerdictDuplicate:
			s.Duplicates++
		case workload.VerdictLate:
			s.Delivered++
			s.OutOfOrder++
		default:
			s.Delivered++
			s.InOrder++
		}
	case sim.KindSend:
		s.observeFlight(r.Flight)
	case sim.KindLoss:
		s.Lost++
	case sim.KindDiscard:
		s.Discarded++
	case sim.KindTimeout:
		if r.Timer != "2msl" {
			s.Timeouts++
		}
	case sim.KindRejection:
		s.Rejected++
	case sim.KindTransition:
		s.Transitions++
	case sim.KindMetric:
		switch r.Metric {
		case "fast_retransmit_saved":
			s.FastRetransmitSaved += sim.VirtualTime(r.Value)
		case "rtt_sample":
			s.RTTSamples++
		}
	}
	s.Missing = s.Offered - s.Delivered
	if s.Delivered > 0 {
		s.TotalTime = s.LastDeliveryAt - s.FirstOfferAt
	}
}

func (s *Stats) observeFlight(f sim.Flight) {
	if f.Duplicate {
		s.Duplicated++
		return
	}
	s.Sent++
	s.BytesOnWire += f.Header.Length
	if f.Header.Retransmission {
		s.Retransmissions++
	}
	if f.Header.Reason == "fast" {
		s.FastRetransmits++
	}
	if f.Corrupted {
		s.Corrupted++
	}
	if f.Reordered {
		s.Reordered++
	}
}

var metrics = map[string]func(Stats) float64{
	"offered":               func(s Stats) float64 { return float64(s.Offered) },
	"delivered":             func(s Stats) float64 { return float64(s.Delivered) },
	"inOrder":               func(s Stats) float64 { return float64(s.InOrder) },
	"outOfOrder":            func(s Stats) float64 { return float64(s.OutOfOrder) },
	"duplicates":            func(s Stats) float64 { return float64(s.Duplicates) },
	"missing":               func(s Stats) float64 { return float64(s.Missing) },
	"sent":                  func(s Stats) float64 { return float64(s.Sent) },
	"bytesOnWire":           func(s Stats) float64 { return float64(s.BytesOnWire) },
	"retransmissions":       func(s Stats) float64 { return float64(s.Retransmissions) },
	"fastRetransmits":       func(s Stats) float64 { return float64(s.FastRetransmits) },
	"timeouts":              func(s Stats) float64 { return float64(s.Timeouts) },
	"lost":                  func(s Stats) float64 { return float64(s.Lost) },
	"corrupted":             func(s Stats) float64 { return float64(s.Corrupted) },
	"duplicated":            func(s Stats) float64 { return float64(s.Duplicated) },
	"reordered":             func(s Stats) float64 { return float64(s.Reordered) },
	"discarded":             func(s Stats) float64 { return float64(s.Discarded) },
	"rejected":              func(s Stats) float64 { return float64(s.Rejected) },
	"transitions":           func(s Stats) float64 { return float64(s.Transitions) },
	"rttSamples":            func(s Stats) float64 { return float64(s.RTTSamples) },
	"totalTimeMs":           func(s Stats) float64 { return s.TotalTime.Milliseconds() },
	"lastDeliveryMs":        func(s Stats) float64 { return s.LastDeliveryAt.Milliseconds() },
	"fastRetransmitSavedMs": func(s Stats) float64 { return s.FastRetransmitSaved.Milliseconds() },
}

func (s Stats) Metric(name string) (float64, bool) {
	get, ok := metrics[name]
	if !ok {
		return 0, false
	}
	return get(s), true
}

func MetricNames() []string {
	return slices.Sorted(maps.Keys(metrics))
}
