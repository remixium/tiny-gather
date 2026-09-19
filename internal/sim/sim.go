package sim

import (
	"github.com/vadremix/tiny-gather/internal/rng"
	"github.com/vadremix/tiny-gather/internal/rules"
	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// QueueDepth is how many actions a player may have waiting.
//
// One action is applied per player per tick. A short queue absorbs an agent that
// thinks in bursts without letting it bank an unbounded backlog and act on a
// world that has moved on.
const QueueDepth = 8

// Sim owns the tick loop, the world, and the single random source.
//
// A run is reproducible from its seed and an ordered log of enqueued actions. To
// keep that true, every draw from the source happens inside Step, in a fixed
// entity order, and inputs are applied in ascending entity-id order rather than
// in the order they arrived.
type Sim struct {
	World *world.World

	rand     *rng.Rand
	tick     uint64
	queues   map[world.ID][]protocol.Action
	deferred []protocol.Event
	respawns []respawn
}

type respawn struct {
	Due      uint64
	Kind     world.Kind
	Resource protocol.Item
	Yield    int
}

// New generates a world from seed and returns a simulation over it.
func New(seed uint64) *Sim {
	r := rng.New(seed)
	return newSim(world.Generate(r), r)
}

// NewWithWorld returns a simulation over an existing world, which is how
// fixtures are run.
func NewWithWorld(w *world.World, seed uint64) *Sim {
	return newSim(w, rng.New(seed))
}

func newSim(w *world.World, r *rng.Rand) *Sim {
	return &Sim{World: w, rand: r, queues: map[world.ID][]protocol.Action{}}
}

// Tick returns the number of ticks resolved so far.
func (s *Sim) Tick() uint64 { return s.tick }

// Rand exposes the random source so that world setup can draw from the same
// stream. It must not be used outside setup or tick resolution.
func (s *Sim) Rand() *rng.Rand { return s.rand }

// Join adds a player at a spawn point near the house.
func (s *Sim) Join(name string) *world.Entity {
	return world.AddPlayer(s.World, name, world.Spawn(s.World, s.rand))
}

// Enqueue queues an action for a player.
//
// A full queue does not silently drop the action: the rejection is held and
// emitted with the next tick's events, so that an agent watching the event
// stream sees every action it submitted accounted for.
func (s *Sim) Enqueue(actor world.ID, a protocol.Action) bool {
	e := s.World.Get(actor)
	if e == nil {
		return false
	}
	if len(s.queues[actor]) >= QueueDepth {
		s.deferred = append(s.deferred,
			protocol.Rejected(s.tick, e.Ref(), a.Kind, protocol.RateLimited))
		return false
	}
	s.queues[actor] = append(s.queues[actor], a)
	return true
}

// Step resolves one tick and returns everything that happened during it.
//
// The order within a tick is fixed: cooldowns expire, then queued actions are
// applied one per player, then in-flight activities advance, then respawns due
// this tick land. Changing this order changes replay, so it is part of the
// contract rather than an implementation detail.
func (s *Sim) Step() []protocol.Event {
	s.tick++

	events := s.deferred
	s.deferred = nil

	for _, id := range s.World.IDs() {
		e := s.World.Get(id)
		if e == nil {
			continue
		}
		if e.MoveCooldown > 0 {
			e.MoveCooldown--
		}
		if e.SayCooldown > 0 {
			e.SayCooldown--
		}
	}

	for _, id := range s.World.IDs() {
		e := s.World.Get(id)
		if e == nil || e.Kind != world.KindPlayer {
			continue
		}
		queued := s.queues[id]
		if len(queued) == 0 {
			continue
		}
		a := queued[0]
		s.queues[id] = queued[1:]
		events = append(events, rules.Apply(s.World, s.tick, e, a)...)
	}

	for _, id := range s.World.IDs() {
		e := s.World.Get(id)
		if e == nil || e.Kind != world.KindPlayer {
			continue
		}
		events = append(events, rules.Advance(s.World, s.tick, e)...)
	}

	for _, ev := range events {
		if ev.Kind == protocol.EvNodeDepleted {
			s.scheduleRespawn(ev)
		}
	}
	events = append(events, s.processRespawns()...)

	return events
}

func (s *Sim) scheduleRespawn(ev protocol.Event) {
	kind := world.KindTree
	if ev.Item == protocol.Ore {
		kind = world.KindRock
	}
	s.respawns = append(s.respawns, respawn{
		Due:      s.tick + rules.RespawnDelay,
		Kind:     kind,
		Resource: ev.Item,
		Yield:    ev.N,
	})
}

// processRespawns places any nodes whose delay has elapsed.
//
// Respawns are handled in the order they were scheduled, and each draws its
// position from the shared source, so the sequence of draws is a function of the
// tick history alone.
func (s *Sim) processRespawns() []protocol.Event {
	var events []protocol.Event
	kept := s.respawns[:0]
	for _, r := range s.respawns {
		if r.Due > s.tick {
			kept = append(kept, r)
			continue
		}
		p, ok := world.FreeTile(s.World, s.rand)
		if !ok {
			// Nowhere to put it; try again next tick rather than losing the node.
			r.Due = s.tick + 1
			kept = append(kept, r)
			continue
		}
		e := s.World.Add(&world.Entity{
			Kind: r.Kind,
			Pos:  p,
			Gatherable: &world.Gatherable{
				Resource: r.Resource, Remaining: r.Yield, Initial: r.Yield,
			},
		})
		events = append(events, protocol.Event{
			Tick: s.tick, Kind: protocol.EvNodeRespawned, Target: e.Ref(),
			Item: r.Resource, N: r.Yield, Pos: &p,
		})
	}
	s.respawns = kept
	return events
}
