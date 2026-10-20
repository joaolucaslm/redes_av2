package session

import (
	"slices"

	"bancada/internal/sim"
	"bancada/internal/testbed"
)

const MaxFrames = 20000

type Frame struct {
	Index    int              `json:"index"`
	At       sim.VirtualTime  `json:"at"`
	Cause    string           `json:"cause"`
	Snapshot testbed.Snapshot `json:"snapshot"`
	Records  []sim.Record     `json:"records"`
}

type entry struct {
	step  int
	apply func(*testbed.Testbed) error
}

type lane struct {
	info    LaneInfo
	build   func() (*testbed.Testbed, error)
	tb      *testbed.Testbed
	frames  []Frame
	cursor  int
	seen    int
	journal []entry
	dirty   int
}

func newLane(info LaneInfo, build func() (*testbed.Testbed, error)) (*lane, error) {
	tb, err := build()
	if err != nil {
		return nil, err
	}
	info.Family = tb.Family()
	l := &lane{info: info, build: build, tb: tb}
	l.frames = []Frame{l.capture("estado inicial")}
	return l, nil
}

func (l *lane) capture(cause string) Frame {
	frame := Frame{
		Index:    len(l.frames),
		At:       l.tb.Now(),
		Cause:    cause,
		Snapshot: l.tb.Snapshot(),
		Records:  l.tb.RecordsSince(l.seen),
	}
	l.seen = l.tb.RecordCount()
	return frame
}

func (l *lane) markDirty(index int) {
	if l.dirty < 0 || index < l.dirty {
		l.dirty = index
	}
}

func (l *lane) current() Frame { return l.frames[l.cursor] }

func (l *lane) atFrontier() bool { return l.cursor == len(l.frames)-1 }

func (l *lane) nextAt() (sim.VirtualTime, bool) {
	if !l.atFrontier() {
		return l.frames[l.cursor+1].At, true
	}
	if len(l.frames) >= MaxFrames {
		return 0, false
	}
	return l.tb.Peek()
}

func (l *lane) forward() bool {
	if !l.atFrontier() {
		l.cursor++
		return true
	}
	if len(l.frames) >= MaxFrames {
		return false
	}
	occ, ok := l.tb.Step()
	if !ok {
		return false
	}
	l.frames = append(l.frames, l.capture(occ.Label))
	l.markDirty(len(l.frames) - 1)
	l.cursor++
	return true
}

func (l *lane) back() bool {
	if l.cursor == 0 {
		return false
	}
	l.cursor--
	return true
}

func (l *lane) syncTo(t sim.VirtualTime, budget int) {
	for l.cursor > 0 && l.current().At > t {
		l.cursor--
	}
	for range budget {
		next, ok := l.nextAt()
		if !ok || next > t || !l.forward() {
			return
		}
	}
}

func (l *lane) fork() error {
	if l.atFrontier() {
		return nil
	}
	tb, err := l.build()
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(l.journal), func(e entry) bool { return e.step > l.cursor })
	next := 0
	for step := 0; step <= l.cursor; step++ {
		for next < len(kept) && kept[next].step == step {
			_ = kept[next].apply(tb)
			next++
		}
		if step < l.cursor {
			tb.Step()
		}
	}
	l.tb = tb
	l.journal = kept
	l.frames = l.frames[:l.cursor+1]
	l.seen = tb.RecordCount()
	l.markDirty(l.cursor + 1)
	return nil
}

func (l *lane) apply(fn func(*testbed.Testbed) error) error {
	if err := l.fork(); err != nil {
		return err
	}
	err := fn(l.tb)
	l.journal = append(l.journal, entry{step: l.cursor, apply: fn})
	frame := l.frames[l.cursor]
	frame.Snapshot = l.tb.Snapshot()
	frame.Records = append(slices.Clone(frame.Records), l.tb.RecordsSince(l.seen)...)
	l.frames[l.cursor] = frame
	l.seen = l.tb.RecordCount()
	l.markDirty(l.cursor)
	return err
}
