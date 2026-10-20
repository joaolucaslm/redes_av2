package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"bancada/internal/channel"
	"bancada/internal/fsm"
	"bancada/internal/host"
	"bancada/internal/scenario"
	"bancada/internal/sim"
	"bancada/internal/testbed"
)

const (
	TopicSession = "bench:session"
	TopicPatch   = "bench:patch"
	TopicClock   = "bench:clock"
	TopicFault   = "bench:fault"

	SlowMotion      = 20.0
	MinSpeed        = 0.25
	MaxSpeed        = 64.0
	tickInterval    = 33 * time.Millisecond
	maxTickGap      = 250 * time.Millisecond
	stepsPerTick    = 4000
	seekBudget      = MaxFrames
	freeScenarioID  = "bancada-livre"
	freeScenarioTxt = "Bancada livre"
)

var (
	ErrNoSession   = errors.New("nenhuma sessão carregada")
	ErrStopped     = errors.New("controlador encerrado")
	ErrBadSpeed    = errors.New("velocidade fora do intervalo de 0,25× a 64×")
	ErrBadLane     = errors.New("faixa inexistente")
	ErrBadProtocol = errors.New("escolha um ou dois protocolos executáveis")
)

type Emitter interface {
	Emit(topic string, payload any)
}

type LoadRequest struct {
	Scenario       string           `json:"scenario"`
	Protocols      []string         `json:"protocols"`
	SideBySide     bool             `json:"sideBySide"`
	Seed           *uint64          `json:"seed"`
	Channel        *channel.Params  `json:"channel"`
	MSL            *sim.VirtualTime `json:"msl"`
	FastRetransmit *bool            `json:"fastRetransmit"`
}

type LaneInfo struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Family   string `json:"family"`
}

type Info struct {
	Scenario       string          `json:"scenario"`
	ScenarioTitle  string          `json:"scenarioTitle"`
	Lanes          []LaneInfo      `json:"lanes"`
	Seed           uint64          `json:"seed"`
	Channel        channel.Params  `json:"channel"`
	MSL            sim.VirtualTime `json:"msl"`
	FastRetransmit bool            `json:"fastRetransmit"`
	Revision       int             `json:"revision"`
}

type Clock struct {
	Time     sim.VirtualTime `json:"time"`
	Cursors  []int           `json:"cursors"`
	Frontier []int           `json:"frontier"`
	Playing  bool            `json:"playing"`
	Speed    float64         `json:"speed"`
	Done     bool            `json:"done"`
	End      sim.VirtualTime `json:"end"`
	Revision int             `json:"revision"`
}

type Patch struct {
	Lane     int     `json:"lane"`
	Base     int     `json:"base"`
	Frames   []Frame `json:"frames"`
	Revision int     `json:"revision"`
}

type Timeline struct {
	Info  Info      `json:"info"`
	Lanes [][]Frame `json:"lanes"`
	Clock Clock     `json:"clock"`
}

type Fault struct {
	Message string `json:"message"`
}

type state struct {
	request  LoadRequest
	info     Info
	lanes    []*lane
	display  sim.VirtualTime
	playing  bool
	speed    float64
	lastTick time.Time
	revision int
}

type reply struct {
	value any
	err   error
}

type request struct {
	run   func(*state) (any, error)
	reply chan reply
}

type Controller struct {
	catalog  *fsm.Catalog
	specs    []scenario.Spec
	emit     Emitter
	requests chan request
	done     chan struct{}
	now      func() time.Time
}

func New(cat *fsm.Catalog, specs []scenario.Spec, emit Emitter) *Controller {
	return &Controller{
		catalog:  cat,
		specs:    specs,
		emit:     emit,
		requests: make(chan request),
		done:     make(chan struct{}),
		now:      time.Now,
	}
}

func (c *Controller) Run(ctx context.Context) {
	defer close(c.done)
	st := &state{speed: 1}
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-c.requests:
			value, err := req.run(st)
			req.reply <- reply{value: value, err: err}
			c.flush(st)
		case <-ticker.C:
			if st.playing {
				st.advance(c.now())
				c.flush(st)
			}
		}
	}
}

func (c *Controller) call(run func(*state) (any, error)) (any, error) {
	req := request{run: run, reply: make(chan reply, 1)}
	select {
	case c.requests <- req:
	case <-c.done:
		return nil, ErrStopped
	}
	select {
	case r := <-req.reply:
		return r.value, r.err
	case <-c.done:
		return nil, ErrStopped
	}
}

func (c *Controller) do(run func(*state) error) error {
	_, err := c.call(func(st *state) (any, error) {
		if len(st.lanes) == 0 {
			return nil, ErrNoSession
		}
		return nil, run(st)
	})
	return err
}

func (c *Controller) flush(st *state) {
	for i, l := range st.lanes {
		if l.dirty < 0 {
			continue
		}
		base := min(l.dirty, len(l.frames))
		c.emit.Emit(TopicPatch, Patch{Lane: i, Base: base, Frames: slices.Clone(l.frames[base:]), Revision: st.revision})
		l.dirty = -1
	}
	if len(st.lanes) > 0 {
		c.emit.Emit(TopicClock, st.clock())
	}
}

func (c *Controller) fault(err error) {
	if err != nil {
		c.emit.Emit(TopicFault, Fault{Message: err.Error()})
	}
}

func (c *Controller) resolve(req LoadRequest) (scenario.Spec, []scenario.RunSpec, error) {
	if req.Scenario != "" {
		spec, err := scenario.Find(c.specs, req.Scenario)
		if err != nil {
			return spec, nil, err
		}
		runs := spec.RunSpecs()
		if !req.SideBySide {
			runs = runs[:1]
		}
		return spec, runs, nil
	}
	if len(req.Protocols) == 0 || len(req.Protocols) > 2 {
		return scenario.Spec{}, nil, ErrBadProtocol
	}
	spec := scenario.Spec{ID: freeScenarioID, Title: freeScenarioTxt, Seed: 1}
	var runs []scenario.RunSpec
	for i, protocol := range req.Protocols {
		if _, err := c.catalog.File(protocol); err != nil {
			return spec, nil, err
		}
		name := protocol
		if slices.Contains(req.Protocols[:i], protocol) {
			name = fmt.Sprintf("%s-%d", protocol, i+1)
		}
		runs = append(runs, scenario.RunSpec{Name: name, Protocol: protocol})
	}
	spec.Runs = runs
	return spec, runs, nil
}

func (st *state) overrides() scenario.Overrides {
	return scenario.Overrides{
		Seed:           st.request.Seed,
		Channel:        st.request.Channel,
		MSL:            st.request.MSL,
		FastRetransmit: st.request.FastRetransmit,
	}
}

func (c *Controller) load(st *state, req LoadRequest) (Info, error) {
	spec, runs, err := c.resolve(req)
	if err != nil {
		return Info{}, err
	}
	previous := st.request
	st.request = req
	overrides := st.overrides()
	lanes := make([]*lane, 0, len(runs))
	for _, run := range runs {
		l, err := newLane(LaneInfo{Name: run.Name, Protocol: run.Protocol}, func() (*testbed.Testbed, error) {
			return scenario.Prepare(c.catalog, spec, run, overrides)
		})
		if err != nil {
			st.request = previous
			return Info{}, err
		}
		lanes = append(lanes, l)
	}
	st.lanes = lanes
	st.display = 0
	st.playing = false
	st.revision++
	cfg := lanes[0].tb.Config()
	st.info = Info{
		Scenario:       spec.ID,
		ScenarioTitle:  spec.Title,
		Seed:           cfg.Seed,
		Channel:        cfg.Channel,
		MSL:            cfg.TCP.MSL,
		FastRetransmit: cfg.TCP.FastRetransmit,
		Revision:       st.revision,
	}
	for _, l := range lanes {
		st.info.Lanes = append(st.info.Lanes, l.info)
	}
	c.emit.Emit(TopicSession, st.info)
	return st.info, nil
}

func (st *state) clock() Clock {
	clk := Clock{Time: st.display, Playing: st.playing, Speed: st.speed, Done: st.exhausted(), Revision: st.revision}
	for _, l := range st.lanes {
		clk.Cursors = append(clk.Cursors, l.cursor)
		clk.Frontier = append(clk.Frontier, len(l.frames))
		clk.End = max(clk.End, l.frames[len(l.frames)-1].At)
	}
	return clk
}

func (st *state) exhausted() bool {
	for _, l := range st.lanes {
		if _, ok := l.nextAt(); ok {
			return false
		}
	}
	return true
}

func (st *state) quiet() bool {
	for _, l := range st.lanes {
		if l.current().Snapshot.Channel.InTransit > 0 {
			return false
		}
	}
	return true
}

func (st *state) earliestNext() (int, sim.VirtualTime, bool) {
	best, at := -1, sim.VirtualTime(0)
	for i, l := range st.lanes {
		if next, ok := l.nextAt(); ok && (best < 0 || next < at) {
			best, at = i, next
		}
	}
	return best, at, best >= 0
}

func (st *state) advance(now time.Time) {
	elapsed := min(now.Sub(st.lastTick), maxTickGap)
	st.lastTick = now
	delta := sim.VirtualTime(float64(elapsed.Microseconds()) * st.speed / SlowMotion)
	target := st.display + max(delta, sim.Microsecond)
	if _, next, ok := st.earliestNext(); ok && st.quiet() && next > target {
		target = next
	}
	moved := false
	for _, l := range st.lanes {
		for range stepsPerTick {
			next, ok := l.nextAt()
			if !ok || next > target || !l.forward() {
				break
			}
			moved = true
		}
	}
	st.display = target
	if st.exhausted() {
		st.playing = false
		st.display = st.endOfCursors()
		return
	}
	if _, _, ok := st.earliestNext(); !ok && !moved {
		st.playing = false
	}
}

func (st *state) endOfCursors() sim.VirtualTime {
	var end sim.VirtualTime
	for _, l := range st.lanes {
		end = max(end, l.current().At)
	}
	return end
}

func (st *state) stepForward() bool {
	best, at, ok := st.earliestNext()
	if !ok || !st.lanes[best].forward() {
		return false
	}
	st.display = at
	return true
}

func (st *state) stepBack() bool {
	best := -1
	var at sim.VirtualTime
	for i, l := range st.lanes {
		if l.cursor == 0 {
			continue
		}
		if current := l.current().At; best < 0 || current >= at {
			best, at = i, current
		}
	}
	if best < 0 {
		return false
	}
	st.lanes[best].back()
	st.display = st.endOfCursors()
	return true
}

func (st *state) seekTime(t sim.VirtualTime) {
	t = max(t, 0)
	for _, l := range st.lanes {
		l.syncTo(t, seekBudget)
	}
	st.display = t
}

func (st *state) applyAll(fn func(*testbed.Testbed) error) error {
	var problems []error
	for _, l := range st.lanes {
		problems = append(problems, l.apply(fn))
	}
	return errors.Join(problems...)
}

func (c *Controller) Load(req LoadRequest) (Info, error) {
	value, err := c.call(func(st *state) (any, error) { return c.load(st, req) })
	info, _ := value.(Info)
	return info, err
}

func (c *Controller) Reset() (Info, error) {
	value, err := c.call(func(st *state) (any, error) {
		if len(st.lanes) == 0 {
			return Info{}, ErrNoSession
		}
		req := st.request
		params := st.info.Channel
		req.Channel = &params
		return c.load(st, req)
	})
	info, _ := value.(Info)
	return info, err
}

func (c *Controller) Play() error {
	return c.do(func(st *state) error {
		st.playing = !st.exhausted()
		st.lastTick = c.now()
		return nil
	})
}

func (c *Controller) Pause() error {
	return c.do(func(st *state) error {
		st.playing = false
		return nil
	})
}

func (c *Controller) StepForward() error {
	return c.do(func(st *state) error {
		st.playing = false
		st.stepForward()
		return nil
	})
}

func (c *Controller) StepBack() error {
	return c.do(func(st *state) error {
		st.playing = false
		st.stepBack()
		return nil
	})
}

func (c *Controller) SeekFrame(laneIndex, frame int) error {
	return c.do(func(st *state) error {
		if laneIndex < 0 || laneIndex >= len(st.lanes) {
			return ErrBadLane
		}
		l := st.lanes[laneIndex]
		frame = min(max(frame, 0), len(l.frames)-1)
		st.playing = false
		st.seekTime(l.frames[frame].At)
		l.cursor = frame
		return nil
	})
}

func (c *Controller) SeekTime(t sim.VirtualTime) error {
	return c.do(func(st *state) error {
		st.playing = false
		st.seekTime(t)
		return nil
	})
}

func (c *Controller) SetSpeed(speed float64) error {
	if speed < MinSpeed || speed > MaxSpeed {
		return fmt.Errorf("%w: %v", ErrBadSpeed, speed)
	}
	return c.do(func(st *state) error {
		st.speed = speed
		return nil
	})
}

func (c *Controller) SetChannel(p channel.Params) error {
	if err := p.Validate(); err != nil {
		return err
	}
	return c.do(func(st *state) error {
		st.info.Channel = p
		return st.applyAll(func(tb *testbed.Testbed) error { return tb.Apply(testbed.ChannelCommand(p)) })
	})
}

func (c *Controller) DropNext(from string) error {
	return c.do(func(st *state) error {
		return st.applyAll(func(tb *testbed.Testbed) error { return tb.Apply(testbed.DropNextCommand(from)) })
	})
}

func (c *Controller) AppCommand(hostName string, verb host.Verb, count int) error {
	return c.do(func(st *state) error {
		at := st.display
		cmd := testbed.AppCommand(hostName, host.Command{Verb: verb, Count: count})
		return st.applyAll(func(tb *testbed.Testbed) error {
			tb.Schedule(max(at, tb.Now()), cmd)
			return nil
		})
	})
}

func (c *Controller) Transfer(messages, size int) error {
	return c.do(func(st *state) error {
		start := st.display
		return st.applyAll(func(tb *testbed.Testbed) error {
			w := scenario.Workload{Messages: messages, Size: size, Interval: 10 * sim.Millisecond, Start: max(start, tb.Now())}
			return scenario.Install(tb, scenario.WorkloadSteps(tb.Family(), w))
		})
	})
}

func (c *Controller) Timeline() (Timeline, error) {
	value, err := c.call(func(st *state) (any, error) {
		if len(st.lanes) == 0 {
			return Timeline{}, ErrNoSession
		}
		tl := Timeline{Info: st.info, Clock: st.clock()}
		for _, l := range st.lanes {
			tl.Lanes = append(tl.Lanes, slices.Clone(l.frames))
		}
		return tl, nil
	})
	tl, _ := value.(Timeline)
	return tl, err
}

type fixtures struct {
	catalog *fsm.Catalog
	specs   []scenario.Spec
}

func (c *Controller) Replace(cat *fsm.Catalog, specs []scenario.Spec) error {
	_, err := c.call(func(*state) (any, error) {
		c.catalog, c.specs = cat, specs
		return nil, nil
	})
	return err
}

func (c *Controller) RunSuite() ([]scenario.Result, error) {
	value, err := c.call(func(*state) (any, error) { return fixtures{catalog: c.catalog, specs: c.specs}, nil })
	if err != nil {
		return nil, err
	}
	f, _ := value.(fixtures)
	return scenario.Runner{Catalog: f.catalog}.RunAll(f.specs), nil
}

func (c *Controller) Report(err error) {
	c.fault(err)
}
