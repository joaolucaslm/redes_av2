package app

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"bancada/internal/channel"
	"bancada/internal/host"
	"bancada/internal/scenario"
	"bancada/internal/session"
	"bancada/internal/sim"
)

type wailsEmitter struct {
	ctx context.Context
}

func (e *wailsEmitter) Emit(topic string, payload any) {
	runtime.EventsEmit(e.ctx, topic, payload)
}

type Runtime struct {
	mu         sync.Mutex
	sources    Sources
	emitter    *wailsEmitter
	controller *session.Controller
	cancel     context.CancelFunc
}

func NewRuntime(sources Sources) *Runtime {
	emitter := &wailsEmitter{}
	return &Runtime{
		sources:    sources,
		emitter:    emitter,
		controller: session.New(sources.Catalog, sources.Specs, emitter),
		cancel:     func() {},
	}
}

func (r *Runtime) Startup(ctx context.Context) {
	r.emitter.ctx = ctx
	loop, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	go r.controller.Run(loop)
}

func (r *Runtime) Shutdown(context.Context) {
	r.cancel()
}

func (r *Runtime) Bench() *Bench {
	return &Bench{runtime: r}
}

type Bench struct {
	runtime *Runtime
}

func (b *Bench) ctl() *session.Controller { return b.runtime.controller }

func (r *Runtime) current() Sources {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sources
}

func (b *Bench) Catalog() session.Catalog {
	src := b.runtime.current()
	return session.BuildCatalog(src.Catalog, src.Specs, src.Warnings)
}

func (b *Bench) ReloadDefinitions() (session.Catalog, error) {
	src, err := LoadSources()
	if err != nil {
		return session.Catalog{}, err
	}
	if err := b.ctl().Replace(src.Catalog, src.Specs); err != nil {
		return session.Catalog{}, err
	}
	b.runtime.mu.Lock()
	b.runtime.sources = src
	b.runtime.mu.Unlock()
	return session.BuildCatalog(src.Catalog, src.Specs, src.Warnings), nil
}

func (b *Bench) Load(req session.LoadRequest) (session.Info, error) {
	return b.ctl().Load(req)
}

func (b *Bench) Reset() (session.Info, error) {
	return b.ctl().Reset()
}

func (b *Bench) Timeline() (session.Timeline, error) {
	return b.ctl().Timeline()
}

func (b *Bench) Play() error { return b.ctl().Play() }

func (b *Bench) Pause() error { return b.ctl().Pause() }

func (b *Bench) StepForward() error { return b.ctl().StepForward() }

func (b *Bench) StepBack() error { return b.ctl().StepBack() }

func (b *Bench) SeekFrame(lane, frame int) error { return b.ctl().SeekFrame(lane, frame) }

func (b *Bench) SeekTime(micros int64) error { return b.ctl().SeekTime(sim.VirtualTime(micros)) }

func (b *Bench) SetSpeed(speed float64) error { return b.ctl().SetSpeed(speed) }

func (b *Bench) SetChannel(params channel.Params) error { return b.ctl().SetChannel(params) }

func (b *Bench) DropNext(from string) error { return b.ctl().DropNext(from) }

func (b *Bench) AppCommand(hostName, verb string, count int) error {
	return b.ctl().AppCommand(hostName, host.Verb(verb), count)
}

func (b *Bench) Transfer(messages, size int) error { return b.ctl().Transfer(messages, size) }

func (b *Bench) RunSuite() ([]scenario.Result, error) { return b.ctl().RunSuite() }
