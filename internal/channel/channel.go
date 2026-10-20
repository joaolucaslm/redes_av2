package channel

import (
	"math/rand/v2"
	"slices"

	"bancada/internal/sim"
	"bancada/internal/wire"
)

const (
	CauseRandom = "aleatória"
	CauseManual = "manual"
	CauseRule   = "regra"
	AnySource   = ""
)

type Packet struct {
	Src    string
	Dst    string
	Frame  wire.Frame
	Header sim.Header
}

type Transit struct {
	Flight sim.Flight
	Frame  wire.Frame
}

type draws struct {
	loss    float64
	delay   float64
	corrupt float64
	bits    [MaxCorruptBits]uint64
	dup     float64
	dupLag  float64
	reorder float64
}

type Channel struct {
	params   Params
	loss     *rand.Rand
	delay    *rand.Rand
	corrupt  *rand.Rand
	dup      *rand.Rand
	reorder  *rand.Rand
	nextID   int
	dropNext []string
	rules    []*armedRule
	horizon  map[string]sim.VirtualTime
}

func New(params Params, streams sim.Streams) (*Channel, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}
	return &Channel{
		params:  params,
		loss:    streams.Stream("channel.loss"),
		delay:   streams.Stream("channel.delay"),
		corrupt: streams.Stream("channel.corrupt"),
		dup:     streams.Stream("channel.duplicate"),
		reorder: streams.Stream("channel.reorder"),
		horizon: make(map[string]sim.VirtualTime),
	}, nil
}

func (c *Channel) Params() Params { return c.params }

func (c *Channel) SetParams(p Params) error {
	if err := p.Validate(); err != nil {
		return err
	}
	c.params = p
	return nil
}

func (c *Channel) ArmDropNext(src string) {
	c.dropNext = append(c.dropNext, src)
}

func (c *Channel) ArmedDrops() []string { return slices.Clone(c.dropNext) }

func (c *Channel) AddRule(r Rule) error {
	if err := r.Validate(); err != nil {
		return err
	}
	c.rules = append(c.rules, &armedRule{rule: r})
	return nil
}

func (c *Channel) PendingRules() int {
	n := 0
	for _, r := range c.rules {
		if !r.spent {
			n++
		}
	}
	return n
}

func (c *Channel) draw() draws {
	d := draws{
		loss:    c.loss.Float64(),
		delay:   c.delay.Float64(),
		corrupt: c.corrupt.Float64(),
	}
	for i := range d.bits {
		d.bits[i] = c.corrupt.Uint64()
	}
	d.dup = c.dup.Float64()
	d.dupLag = c.dup.Float64()
	d.reorder = c.reorder.Float64()
	return d
}

func (c *Channel) consumeDropNext(src string) bool {
	for i, armed := range c.dropNext {
		if armed == AnySource || armed == src {
			c.dropNext = slices.Delete(c.dropNext, i, i+1)
			return true
		}
	}
	return false
}

func (c *Channel) fireRules(p Packet) map[Action]bool {
	fired := make(map[Action]bool)
	for _, armed := range c.rules {
		if armed.spent || !armed.rule.Match.Matches(p.Src, p.Header) {
			continue
		}
		armed.seen++
		if armed.seen == armed.rule.Nth {
			armed.spent = true
			fired[armed.rule.Action] = true
		}
	}
	return fired
}

func (c *Channel) Transmit(now sim.VirtualTime, p Packet) []Transit {
	d := c.draw()
	forced := c.fireRules(p)
	span := c.params.DelayMax - c.params.DelayMin
	delay := c.params.DelayMin + sim.VirtualTime(d.delay*float64(span))

	c.nextID++
	flight := sim.Flight{ID: c.nextID, Src: p.Src, Dst: p.Dst, SentAt: now, Header: p.Header}

	switch {
	case c.consumeDropNext(p.Src):
		flight.Cause = CauseManual
	case forced[ActionDrop]:
		flight.Cause = CauseRule
	case d.loss < c.params.Loss:
		flight.Cause = CauseRandom
	}
	if flight.Cause != "" {
		flight.Fate = sim.FateLost
		flight.EndAt = now + delay/2
		return []Transit{{Flight: flight}}
	}

	frame := p.Frame.Clone()
	if forced[ActionCorrupt] || d.corrupt < c.params.Corrupt {
		flight.Corrupted = true
		flight.Flipped = pickBits(d.bits[:c.params.CorruptBits], len(frame.Bytes)*8)
		for _, bit := range flight.Flipped {
			frame = frame.FlipBit(bit)
		}
	}

	direction := p.Src + ">" + p.Dst
	arrival := now + delay
	if d.reorder < c.params.Reorder {
		flight.Reordered = true
		arrival += c.params.ReorderLag
	} else {
		arrival = max(arrival, c.horizon[direction]+sim.Microsecond)
		c.horizon[direction] = arrival
	}
	flight.Fate = sim.FateDelivered
	flight.EndAt = arrival
	transits := []Transit{{Flight: flight, Frame: frame}}

	if forced[ActionDuplicate] || d.dup < c.params.Duplicate {
		c.nextID++
		copyFlight := flight
		copyFlight.ID = c.nextID
		copyFlight.Duplicate = true
		copyFlight.Corrupted = false
		copyFlight.Flipped = nil
		copyFlight.Reordered = false
		copyFlight.EndAt = max(arrival+sim.Millisecond+sim.VirtualTime(d.dupLag*float64(span)), c.horizon[direction]+sim.Microsecond)
		c.horizon[direction] = max(c.horizon[direction], copyFlight.EndAt)
		transits = append(transits, Transit{Flight: copyFlight, Frame: p.Frame.Clone()})
	}
	return transits
}

func pickBits(draws []uint64, totalBits int) []int {
	if totalBits == 0 {
		return nil
	}
	var picked []int
	for _, raw := range draws {
		bit := int(raw % uint64(totalBits))
		for slices.Contains(picked, bit) {
			bit = (bit + 1) % totalBits
		}
		picked = append(picked, bit)
		if len(picked) == totalBits {
			break
		}
	}
	slices.Sort(picked)
	return picked
}
