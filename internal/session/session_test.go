package session

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	fsmfiles "bancada/fsm"
	"bancada/internal/channel"
	"bancada/internal/host"
	"bancada/internal/scenario"
	"bancada/internal/sim"
	"bancada/internal/testbed"
	scenariofiles "bancada/scenarios"
)

type recorder struct {
	mu     sync.Mutex
	topics map[string]int
}

func (r *recorder) Emit(topic string, payload any) {
	if _, err := json.Marshal(payload); err != nil {
		panic(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.topics[topic]++
}

func (r *recorder) count(topic string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.topics[topic]
}

func start(t *testing.T) (*Controller, *recorder) {
	t.Helper()
	cat, err := testbed.LoadCatalog(fsmfiles.Files)
	if err != nil {
		t.Fatal(err)
	}
	specs, err := scenario.LoadSpecs(scenariofiles.Files)
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{topics: make(map[string]int)}
	ctl := New(cat, specs, rec)
	ctx, cancel := context.WithCancel(context.Background())
	go ctl.Run(ctx)
	t.Cleanup(cancel)
	return ctl, rec
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func snapshotAt(t *testing.T, ctl *Controller, laneIndex int) string {
	t.Helper()
	tl, err := ctl.Timeline()
	must(t, err)
	frames := tl.Lanes[laneIndex]
	data, err := json.Marshal(frames[tl.Clock.Cursors[laneIndex]].Snapshot)
	must(t, err)
	return string(data)
}

func TestCallsBeforeLoadFail(t *testing.T) {
	ctl, _ := start(t)
	if err := ctl.Play(); err != ErrNoSession {
		t.Fatalf("Play sem sessão = %v", err)
	}
	if err := ctl.SetSpeed(100); err == nil {
		t.Fatal("velocidade inválida aceita")
	}
}

func TestStepBackReturnsExactlyToEarlierSnapshot(t *testing.T) {
	ctl, rec := start(t)
	_, err := ctl.Load(LoadRequest{Scenario: "06-fast-retransmit"})
	must(t, err)
	for range 12 {
		must(t, ctl.StepForward())
	}
	at12 := snapshotAt(t, ctl, 0)
	for range 20 {
		must(t, ctl.StepForward())
	}
	for range 20 {
		must(t, ctl.StepBack())
	}
	if snapshotAt(t, ctl, 0) != at12 {
		t.Fatal("voltar 20 passos não restaurou o snapshot original")
	}
	if rec.count(TopicSession) != 1 || rec.count(TopicPatch) == 0 || rec.count(TopicClock) == 0 {
		t.Fatalf("eventos emitidos: %v", rec.topics)
	}
}

func TestCommandInThePastForksLikeAFreshRun(t *testing.T) {
	params := channel.DefaultParams()
	params.Loss = 0.5
	prepare := func(ctl *Controller) {
		_, err := ctl.Load(LoadRequest{Protocols: []string{"tcp"}})
		must(t, err)
		must(t, ctl.AppCommand("B", host.VerbListen, 0))
		must(t, ctl.AppCommand("A", host.VerbConnect, 0))
	}

	forked, _ := start(t)
	prepare(forked)
	for range 30 {
		must(t, forked.StepForward())
	}
	tl, err := forked.Timeline()
	must(t, err)
	must(t, forked.SeekFrame(0, 4))
	must(t, forked.SetChannel(params))
	for range 26 {
		must(t, forked.StepForward())
	}

	direct, _ := start(t)
	prepare(direct)
	for range 4 {
		must(t, direct.StepForward())
	}
	must(t, direct.SetChannel(params))
	for range 26 {
		must(t, direct.StepForward())
	}

	if snapshotAt(t, forked, 0) != snapshotAt(t, direct, 0) {
		t.Fatal("o futuro reescrito a partir do passo 4 diverge de uma execução direta")
	}
	after, err := forked.Timeline()
	must(t, err)
	before, _ := json.Marshal(tl.Lanes[0][:4])
	kept, _ := json.Marshal(after.Lanes[0][:4])
	if string(before) != string(kept) {
		t.Fatal("o fork alterou frames anteriores ao ponto do comando")
	}
}

func TestPlaybackReachesTheSameEndAsStepping(t *testing.T) {
	stepped, _ := start(t)
	_, err := stepped.Load(LoadRequest{Scenario: "04-normal-close"})
	must(t, err)
	for range 500 {
		must(t, stepped.StepForward())
	}

	played, _ := start(t)
	_, err = played.Load(LoadRequest{Scenario: "04-normal-close"})
	must(t, err)
	must(t, played.SetSpeed(MaxSpeed))
	must(t, played.Play())
	deadline := time.Now().Add(20 * time.Second)
	for {
		tl, err := played.Timeline()
		must(t, err)
		if tl.Clock.Done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("reprodução não terminou")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if snapshotAt(t, played, 0) != snapshotAt(t, stepped, 0) {
		t.Fatal("tocar e avançar passo a passo deveriam chegar ao mesmo estado final")
	}
}

func TestConcurrentCallersDoNotRace(t *testing.T) {
	ctl, _ := start(t)
	_, err := ctl.Load(LoadRequest{Scenario: "10-udp-vs-tcp", SideBySide: true})
	must(t, err)
	must(t, ctl.SetSpeed(MaxSpeed))
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(worker), 7))
			for range 40 {
				switch rng.IntN(9) {
				case 0:
					ctl.Play()
				case 1:
					ctl.Pause()
				case 2:
					ctl.StepForward()
				case 3:
					ctl.StepBack()
				case 4:
					ctl.SeekTime(sim.VirtualTime(rng.IntN(2000)) * sim.Millisecond)
				case 5:
					ctl.SetSpeed([]float64{0.25, 1, 64}[rng.IntN(3)])
				case 6:
					ctl.DropNext("")
				case 7:
					if _, err := ctl.Timeline(); err != nil {
						t.Error(err)
					}
				default:
					p := channel.DefaultParams()
					p.Loss = rng.Float64() / 2
					ctl.SetChannel(p)
				}
			}
		}()
	}
	wg.Wait()
	must(t, ctl.Pause())
	tl, err := ctl.Timeline()
	must(t, err)
	for laneIndex, frames := range tl.Lanes {
		for i, f := range frames {
			if f.Index != i {
				t.Fatalf("faixa %d: frame %d com índice %d", laneIndex, i, f.Index)
			}
			if i > 0 && f.At < frames[i-1].At {
				t.Fatalf("faixa %d: tempo regrediu no frame %d", laneIndex, i)
			}
		}
		if tl.Clock.Cursors[laneIndex] >= len(frames) {
			t.Fatalf("cursor fora da linha do tempo")
		}
	}
}

func TestCatalogDescribesDiagrams(t *testing.T) {
	cat, err := testbed.LoadCatalog(fsmfiles.Files)
	must(t, err)
	specs, err := scenario.LoadSpecs(scenariofiles.Files)
	must(t, err)
	dto := BuildCatalog(cat, specs, nil)
	for _, p := range dto.Protocols {
		if p.ID == "tcp" {
			if len(p.Machines) != 2 || len(p.Machines[0].States) != 11 {
				t.Fatalf("TCP deveria expor conexão (11 estados) e congestionamento: %d máquinas", len(p.Machines))
			}
			return
		}
	}
	t.Fatal("TCP ausente do catálogo")
}
