package sim

import "slices"

type Clock interface {
	Now() VirtualTime
}

type Bus struct {
	clock       Clock
	step        int
	records     []Record
	subscribers []func(Record)
}

func NewBus(clock Clock) *Bus {
	return &Bus{clock: clock}
}

func (b *Bus) SetStep(step int) { b.step = step }

func (b *Bus) Step() int { return b.step }

func (b *Bus) Publish(r Record) Record {
	r.Seq = len(b.records)
	r.Step = b.step
	r.At = b.clock.Now()
	r.Actions = slices.Clone(r.Actions)
	r.Flight.Flipped = slices.Clone(r.Flight.Flipped)
	b.records = append(b.records, r)
	for _, notify := range b.subscribers {
		notify(r)
	}
	return r
}

func (b *Bus) Subscribe(notify func(Record)) {
	b.subscribers = append(b.subscribers, notify)
}

func (b *Bus) Len() int { return len(b.records) }

func (b *Bus) Since(index int) []Record {
	index = min(max(index, 0), len(b.records))
	return slices.Clone(b.records[index:])
}

func (b *Bus) Records() []Record {
	return slices.Clone(b.records)
}
