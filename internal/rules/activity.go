package rules

import (
	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// Advance moves an actor's in-flight activity on by one tick.
//
// Gathering accrues progress and yields one unit at a time, so that an
// interrupted gather loses only the unit in progress. Every way the activity can
// end — the node running out, the actor running out of room — resolves
// unsuccessfully rather than being silently dropped, because an actor that stops
// gathering needs to know why.
func Advance(w *world.World, tick uint64, actor *world.Entity) []protocol.Event {
	if actor.Activity.Kind != protocol.ActGather {
		return nil
	}

	node := w.Get(actor.Activity.Target)
	if node == nil || node.Gatherable == nil || node.Gatherable.Remaining <= 0 {
		return stop(tick, actor)
	}
	res := node.Gatherable.Resource
	if actor.Carrier == nil || actor.Carrier.Inv.Fits(res, 1) < 1 {
		return stop(tick, actor)
	}

	actor.Activity.Progress++
	if actor.Activity.Progress < GatherTicks {
		return nil
	}
	actor.Activity.Progress = 0

	node.Gatherable.Remaining--
	actor.Carrier.Inv.Add(res, 1)

	events := []protocol.Event{{
		Tick: tick, Kind: protocol.EvGathered, Actor: actor.Ref(),
		Target: node.Ref(), Item: res, N: 1, Pos: &node.Pos,
	}}

	if node.Gatherable.Remaining <= 0 {
		// The depletion event carries the resource and original yield so that
		// the caller can schedule a replacement without re-inspecting an entity
		// that is about to stop existing.
		events = append(events, protocol.Event{
			Tick: tick, Kind: protocol.EvNodeDepleted, Target: node.Ref(),
			Item: res, N: node.Gatherable.Initial, Pos: &node.Pos,
		})
		w.Remove(node.ID)
		events = append(events, stop(tick, actor)...)
	}
	return events
}

func stop(tick uint64, actor *world.Entity) []protocol.Event {
	if actor.Activity.Idle() {
		return nil
	}
	kind := actor.Activity.Kind
	actor.Activity = world.Activity{}
	return []protocol.Event{protocol.Resolved(tick, actor.Ref(), kind, false)}
}
