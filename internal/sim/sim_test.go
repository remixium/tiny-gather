package sim

import (
	"encoding/json"
	"testing"

	"github.com/vadremix/tiny-gather/internal/rules"
	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// script is an ordered log of actions to enqueue at given ticks: the second
// half of the (seed, input log) pair a run must be reproducible from.
type script []struct {
	atTick uint64
	player int
	action protocol.Action
}

// run plays a script against a fresh simulation and returns every event in
// order.
func run(seed uint64, players []string, ticks int, s script) []protocol.Event {
	sim := New(seed)
	ids := make([]world.ID, len(players))
	for i, name := range players {
		ids[i] = sim.Join(name).ID
	}

	var all []protocol.Event
	for tick := 0; tick < ticks; tick++ {
		for _, step := range s {
			if step.atTick == uint64(tick) {
				sim.Enqueue(ids[step.player], step.action)
			}
		}
		all = append(all, sim.Step()...)
	}
	return all
}

func encode(t *testing.T, events []protocol.Event) string {
	t.Helper()
	b, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("encoding events: %v", err)
	}
	return string(b)
}

// TestReplayIsDeterministic is the invariant the whole design rests on. Once
// mechanics are probabilistic, a scenario is scored over many seeded runs, and
// that is only meaningful if the same seed and the same inputs produce the same
// run every time.
func TestReplayIsDeterministic(t *testing.T) {
	sc := script{
		{atTick: 1, player: 0, action: protocol.Action{Kind: protocol.ActMove, Dir: protocol.DirN}},
		{atTick: 2, player: 1, action: protocol.Action{Kind: protocol.ActMove, Dir: protocol.DirS}},
		{atTick: 5, player: 0, action: protocol.Action{Kind: protocol.ActSay, Text: "over here"}},
		{atTick: 9, player: 1, action: protocol.Action{Kind: protocol.ActMove, Dir: protocol.DirE}},
		{atTick: 9, player: 0, action: protocol.Action{Kind: protocol.ActMove, Dir: protocol.DirW}},
	}
	players := []string{"ana", "bo"}

	first := encode(t, run(7, players, 60, sc))
	for i := 0; i < 5; i++ {
		if got := encode(t, run(7, players, 60, sc)); got != first {
			t.Fatalf("run %d diverged from the first run with the same seed and inputs", i+2)
		}
	}
}

// TestDifferentSeedsDiverge checks the seed actually reaches generation.
//
// It compares world layout rather than event streams, because an idle player
// produces no events at all and two empty streams would match no matter what
// the seed did.
func TestDifferentSeedsDiverge(t *testing.T) {
	layout := func(seed uint64) string {
		sim := New(seed)
		out := ""
		sim.World.Each(func(e *world.Entity) bool {
			out += e.Ref() + e.Pos.String() + ";"
			return true
		})
		return out
	}
	if layout(1) == layout(2) {
		t.Fatal("seeds 1 and 2 produced identical worlds; the seed is not reaching generation")
	}
}

// TestResolutionOrderIsByEntityID checks that inputs are applied in id order
// rather than in the order they were enqueued. Arrival order depends on network
// timing and goroutine scheduling, neither of which can be replayed.
func TestResolutionOrderIsByEntityID(t *testing.T) {
	w := world.New(20, 20)
	first := world.AddPlayer(w, "first", protocol.Pos{X: 5, Y: 5})
	second := world.AddPlayer(w, "second", protocol.Pos{X: 10, Y: 10})

	sim := NewWithWorld(w, 1)
	// Enqueue for the higher id first, so arrival order is the reverse of id
	// order.
	sim.Enqueue(second.ID, protocol.Action{Kind: protocol.ActSay, Text: "second"})
	sim.Enqueue(first.ID, protocol.Action{Kind: protocol.ActSay, Text: "first"})

	var spoken []string
	for _, ev := range sim.Step() {
		if ev.Kind == protocol.EvSaid {
			spoken = append(spoken, ev.Text)
		}
	}
	if len(spoken) != 2 {
		t.Fatalf("got %d chat events, want 2", len(spoken))
	}
	if spoken[0] != "first" || spoken[1] != "second" {
		t.Fatalf("resolved in order %v, want [first second] by entity id", spoken)
	}
}

// TestQueueOverflowIsReported: a dropped action must not vanish silently, or an
// agent has no way to tell a refused submission from one still pending.
func TestQueueOverflowIsReported(t *testing.T) {
	w := world.New(20, 20)
	p := world.AddPlayer(w, "chatty", protocol.Pos{X: 5, Y: 5})
	sim := NewWithWorld(w, 1)

	for i := 0; i < QueueDepth; i++ {
		if !sim.Enqueue(p.ID, protocol.Action{Kind: protocol.ActMove, Dir: protocol.DirN}) {
			t.Fatalf("action %d refused while the queue should still have room", i)
		}
	}
	if sim.Enqueue(p.ID, protocol.Action{Kind: protocol.ActMove, Dir: protocol.DirN}) {
		t.Fatal("the queue accepted more than QueueDepth actions")
	}

	var rejected *protocol.Event
	for _, ev := range sim.Step() {
		if ev.Kind == protocol.EvRejected && ev.Reason == protocol.RateLimited {
			rejected = &ev
		}
	}
	if rejected == nil {
		t.Fatal("the refused action produced no rejection event")
	}
}

// TestGatherTakesTime checks the rate is what the tuning constant says, since
// every economic comparison an agent makes is ultimately a rate comparison.
func TestGatherTakesTime(t *testing.T) {
	w := world.New(20, 20)
	p := world.AddPlayer(w, "logger", protocol.Pos{X: 5, Y: 5})
	tree := w.Add(&world.Entity{
		Kind: world.KindTree, Pos: protocol.Pos{X: 6, Y: 5},
		Gatherable: &world.Gatherable{Resource: protocol.Wood, Remaining: 5, Initial: 5},
	})

	sim := NewWithWorld(w, 1)
	sim.Enqueue(p.ID, protocol.Action{Kind: protocol.ActGather, Target: tree.Ref()})

	gathered := 0
	for tick := 0; tick < rules.GatherTicks; tick++ {
		for _, ev := range sim.Step() {
			if ev.Kind == protocol.EvGathered {
				gathered++
			}
		}
	}
	if gathered != 1 {
		t.Fatalf("gathered %d wood in %d ticks, want exactly 1", gathered, rules.GatherTicks)
	}
	if got := p.Carrier.Inv.Count(protocol.Wood); got != 1 {
		t.Fatalf("player holds %d wood, want 1", got)
	}
}

// TestDepletedNodeRespawns: nodes are finite but the world does not run dry.
func TestDepletedNodeRespawns(t *testing.T) {
	w := world.New(20, 20)
	p := world.AddPlayer(w, "logger", protocol.Pos{X: 5, Y: 5})
	tree := w.Add(&world.Entity{
		Kind: world.KindTree, Pos: protocol.Pos{X: 6, Y: 5},
		Gatherable: &world.Gatherable{Resource: protocol.Wood, Remaining: 1, Initial: 4},
	})

	sim := NewWithWorld(w, 1)
	sim.Enqueue(p.ID, protocol.Action{Kind: protocol.ActGather, Target: tree.Ref()})

	depleted, respawned := false, false
	for tick := 0; tick < rules.GatherTicks+rules.RespawnDelay+5; tick++ {
		for _, ev := range sim.Step() {
			switch ev.Kind {
			case protocol.EvNodeDepleted:
				depleted = true
				if ev.N != 4 {
					t.Errorf("depletion reported yield %d, want the original 4", ev.N)
				}
			case protocol.EvNodeRespawned:
				respawned = true
				if ev.Item != protocol.Wood {
					t.Errorf("respawned a %s node, want wood", ev.Item)
				}
			}
		}
	}
	if !depleted {
		t.Error("the exhausted node never reported depletion")
	}
	if !respawned {
		t.Error("no replacement node appeared")
	}
}
