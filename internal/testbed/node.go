package testbed

import (
	"cmp"
	"maps"
	"math/rand/v2"
	"net/netip"
	"slices"

	"bancada/internal/host"
	"bancada/internal/sim"
	"bancada/internal/wire"
)

type node struct {
	tb       *Testbed
	name     string
	peer     string
	role     host.Role
	addr     netip.Addr
	peerAddr netip.Addr
	stack    host.Stack
	timers   map[sim.TimerID]host.TimerView
}

func (n *node) Name() string { return n.name }

func (n *node) Peer() string { return n.peer }

func (n *node) Role() host.Role { return n.role }

func (n *node) Addr() netip.Addr { return n.addr }

func (n *node) PeerAddr() netip.Addr { return n.peerAddr }

func (n *node) Now() sim.VirtualTime { return n.tb.sched.Now() }

func (n *node) Transmit(frame wire.Frame, header sim.Header) {
	n.tb.transmit(n, frame, header)
}

func (n *node) Arm(timer string, after sim.VirtualTime, fire func()) sim.TimerID {
	var id sim.TimerID
	id = n.tb.sched.After(after, n.name+" · timeout "+timer, func() {
		delete(n.timers, id)
		n.Publish(sim.Record{Kind: sim.KindTimeout, Timer: timer})
		fire()
	})
	deadline, _ := n.tb.sched.Deadline(id)
	n.timers[id] = host.TimerView{Name: timer, Deadline: deadline}
	n.Publish(sim.Record{Kind: sim.KindTimerArm, Timer: timer, Deadline: deadline})
	return id
}

func (n *node) Disarm(id sim.TimerID) bool {
	timer, ok := n.timers[id]
	if !ok {
		return false
	}
	n.tb.sched.Cancel(id)
	delete(n.timers, id)
	n.Publish(sim.Record{Kind: sim.KindTimerStop, Timer: timer.Name, Deadline: timer.Deadline})
	return true
}

func (n *node) Deadline(id sim.TimerID) (sim.VirtualTime, bool) {
	return n.tb.sched.Deadline(id)
}

func (n *node) Defer(label string, after sim.VirtualTime, fire func()) {
	n.tb.sched.After(after, n.name+" · "+label, fire)
}

func (n *node) Publish(r sim.Record) {
	if r.Host == "" {
		r.Host = n.name
	}
	n.tb.bus.Publish(r)
}

func (n *node) Stream(name string) *rand.Rand {
	return n.tb.streams.Stream(n.name + "/" + name)
}

func (n *node) timerViews() []host.TimerView {
	views := slices.Collect(maps.Values(n.timers))
	slices.SortFunc(views, func(a, b host.TimerView) int {
		return cmp.Or(cmp.Compare(a.Deadline, b.Deadline), cmp.Compare(a.Name, b.Name))
	})
	if views == nil {
		views = []host.TimerView{}
	}
	return views
}
