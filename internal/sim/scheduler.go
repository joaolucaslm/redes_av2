package sim

import "container/heap"

type TimerID uint64

type Occurrence struct {
	ID    TimerID
	At    VirtualTime
	Label string
	fire  func()
}

func (o Occurrence) Fire() {
	if o.fire != nil {
		o.fire()
	}
}

type agendaItem struct {
	at    VirtualTime
	order uint64
	id    TimerID
	label string
	fire  func()
	slot  int
}

type agenda []*agendaItem

func (a agenda) Len() int { return len(a) }

func (a agenda) Less(i, j int) bool {
	if a[i].at != a[j].at {
		return a[i].at < a[j].at
	}
	return a[i].order < a[j].order
}

func (a agenda) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
	a[i].slot = i
	a[j].slot = j
}

func (a *agenda) Push(x any) {
	item := x.(*agendaItem)
	item.slot = len(*a)
	*a = append(*a, item)
}

func (a *agenda) Pop() any {
	old := *a
	last := old[len(old)-1]
	old[len(old)-1] = nil
	last.slot = -1
	*a = old[:len(old)-1]
	return last
}

type Scheduler struct {
	now     VirtualTime
	queue   agenda
	counter uint64
	live    map[TimerID]*agendaItem
}

func NewScheduler() *Scheduler {
	return &Scheduler{live: make(map[TimerID]*agendaItem)}
}

func (s *Scheduler) Now() VirtualTime { return s.now }

func (s *Scheduler) At(at VirtualTime, label string, fire func()) TimerID {
	at = max(at, s.now)
	s.counter++
	item := &agendaItem{at: at, order: s.counter, id: TimerID(s.counter), label: label, fire: fire}
	heap.Push(&s.queue, item)
	s.live[item.id] = item
	return item.id
}

func (s *Scheduler) After(delay VirtualTime, label string, fire func()) TimerID {
	return s.At(s.now+max(delay, 0), label, fire)
}

func (s *Scheduler) Cancel(id TimerID) bool {
	item, ok := s.live[id]
	if !ok {
		return false
	}
	heap.Remove(&s.queue, item.slot)
	delete(s.live, id)
	return true
}

func (s *Scheduler) Deadline(id TimerID) (VirtualTime, bool) {
	item, ok := s.live[id]
	if !ok {
		return 0, false
	}
	return item.at, true
}

func (s *Scheduler) Peek() (VirtualTime, bool) {
	if len(s.queue) == 0 {
		return 0, false
	}
	return s.queue[0].at, true
}

func (s *Scheduler) Pending() int { return len(s.queue) }

func (s *Scheduler) Next() (Occurrence, bool) {
	if len(s.queue) == 0 {
		return Occurrence{}, false
	}
	item := heap.Pop(&s.queue).(*agendaItem)
	delete(s.live, item.id)
	s.now = item.at
	return Occurrence{ID: item.id, At: item.at, Label: item.label, fire: item.fire}, true
}
