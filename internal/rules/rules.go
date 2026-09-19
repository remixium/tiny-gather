// Package rules validates actions and computes their outcomes.
//
// Outcomes come in two kinds that must never be collapsed into one:
//
//   - Rejected: the action was invalid and nothing happened. The actor should
//     re-plan.
//   - Resolved: the action was valid and was attempted, but may not have
//     achieved anything. The actor may simply try again.
//
// Once building exists, an attempt that fails its probability roll will be
// resolved and unsuccessful: it consumed material and produced nothing. That is
// a different fact from being told the tile was out of reach, and agents act on
// the difference.
//
// A partial result is a success, not a rejection. Depositing five wood into a
// container with room for three resolves successfully with N set to three; the
// caller compares that against what it asked for. Only an action that could
// move nothing at all is rejected.
package rules

import (
	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// Apply attempts a single action on behalf of actor.
//
// The returned slice always begins with exactly one rejection or resolution
// event, followed by any detail events describing what actually moved.
//
// Nothing here draws from the random source: every v1 action is deterministic
// given the world state. Stochastic actions arrive with building, and will take
// the source as a parameter at that point.
func Apply(w *world.World, tick uint64, actor *world.Entity, a protocol.Action) []protocol.Event {
	reject := func(why protocol.Reason) []protocol.Event {
		return []protocol.Event{protocol.Rejected(tick, actor.Ref(), a.Kind, why)}
	}

	switch a.Kind {
	case protocol.ActMove:
		return applyMove(w, tick, actor, a, reject)
	case protocol.ActGather:
		return applyGather(w, tick, actor, a, reject)
	case protocol.ActDrop:
		return applyDrop(w, tick, actor, a, reject)
	case protocol.ActPickup:
		return applyPickup(w, tick, actor, a, reject)
	case protocol.ActDeposit:
		return applyTransfer(w, tick, actor, a, reject, true)
	case protocol.ActWithdraw:
		return applyTransfer(w, tick, actor, a, reject, false)
	case protocol.ActSay:
		return applySay(tick, actor, a, reject)
	default:
		return reject(protocol.MalformedAction)
	}
}

type rejectFn func(protocol.Reason) []protocol.Event

func applyMove(w *world.World, tick uint64, actor *world.Entity, a protocol.Action, reject rejectFn) []protocol.Event {
	dx, dy, ok := a.Dir.Delta()
	if !ok {
		return reject(protocol.MalformedAction)
	}
	if actor.MoveCooldown > 0 {
		return reject(protocol.RateLimited)
	}
	dest := protocol.Pos{X: actor.Pos.X + dx, Y: actor.Pos.Y + dy}
	if w.Blocked(dest) {
		return reject(protocol.BlockedTile)
	}

	// Moving cancels gathering. This is what makes going to fetch something a
	// commitment rather than a free action.
	events := cancelActivity(tick, actor)

	actor.Pos = dest
	actor.MoveCooldown = MoveCooldown
	resolved := protocol.Resolved(tick, actor.Ref(), a.Kind, true)
	resolved.Pos = &dest
	return append([]protocol.Event{resolved}, events...)
}

func applyGather(w *world.World, tick uint64, actor *world.Entity, a protocol.Action, reject rejectFn) []protocol.Event {
	target := w.GetRef(a.Target)
	if target == nil || target.Gatherable == nil {
		return reject(protocol.NoSuchEntity)
	}
	if !actor.Pos.Adjacent(target.Pos) {
		return reject(protocol.NotAdjacent)
	}
	if target.Gatherable.Remaining <= 0 {
		return reject(protocol.NodeEmpty)
	}
	if actor.Carrier == nil || actor.Carrier.Inv.Fits(target.Gatherable.Resource, 1) < 1 {
		return reject(protocol.InventoryFull)
	}

	// Re-issuing gather against the same node keeps the progress already made,
	// so that an agent re-confirming its intent does not reset the clock.
	if actor.Activity.Kind != protocol.ActGather || actor.Activity.Target != target.ID {
		actor.Activity = world.Activity{Kind: protocol.ActGather, Target: target.ID}
	}
	resolved := protocol.Resolved(tick, actor.Ref(), a.Kind, true)
	resolved.Target = target.Ref()
	resolved.Item = target.Gatherable.Resource
	return []protocol.Event{resolved}
}

func applyDrop(w *world.World, tick uint64, actor *world.Entity, a protocol.Action, reject rejectFn) []protocol.Event {
	if a.N <= 0 || !protocol.KnownItem(a.Item) {
		return reject(protocol.MalformedAction)
	}
	if actor.Carrier == nil || actor.Carrier.Inv.Count(a.Item) < a.N {
		return reject(protocol.InsufficientItems)
	}
	actor.Carrier.Inv.Remove(a.Item, a.N)

	// Dropping onto a tile that already holds the same item merges with it,
	// rather than leaving two piles to disambiguate.
	var pile *world.Entity
	for _, e := range w.At(actor.Pos) {
		if e.Stack != nil && e.Stack.Item == a.Item {
			pile = e
			break
		}
	}
	if pile == nil {
		pile = w.Add(&world.Entity{
			Kind:  world.KindGroundItem,
			Pos:   actor.Pos,
			Stack: &world.Stack{Item: a.Item, N: a.N},
		})
	} else {
		pile.Stack.N += a.N
	}

	resolved := protocol.Resolved(tick, actor.Ref(), a.Kind, true)
	resolved.Item, resolved.N, resolved.Target = a.Item, a.N, pile.Ref()
	resolved.Consumed = map[protocol.Item]int{a.Item: a.N}
	detail := protocol.Event{
		Tick: tick, Kind: protocol.EvDropped, Actor: actor.Ref(),
		Target: pile.Ref(), Item: a.Item, N: a.N, Pos: &actor.Pos,
	}
	return []protocol.Event{resolved, detail}
}

func applyPickup(w *world.World, tick uint64, actor *world.Entity, a protocol.Action, reject rejectFn) []protocol.Event {
	target := w.GetRef(a.Target)
	if target == nil || target.Stack == nil {
		return reject(protocol.NoSuchEntity)
	}
	if !actor.Pos.Adjacent(target.Pos) {
		return reject(protocol.NotAdjacent)
	}
	if actor.Carrier == nil {
		return reject(protocol.MalformedAction)
	}
	took := actor.Carrier.Inv.Fits(target.Stack.Item, target.Stack.N)
	if took <= 0 {
		return reject(protocol.InventoryFull)
	}

	item := target.Stack.Item
	actor.Carrier.Inv.Add(item, took)
	target.Stack.N -= took
	ref := target.Ref()
	if target.Stack.N == 0 {
		w.Remove(target.ID)
	}

	resolved := protocol.Resolved(tick, actor.Ref(), a.Kind, true)
	resolved.Item, resolved.N, resolved.Target = item, took, ref
	resolved.Produced = map[protocol.Item]int{item: took}
	detail := protocol.Event{
		Tick: tick, Kind: protocol.EvPickedUp, Actor: actor.Ref(),
		Target: ref, Item: item, N: took, Pos: &actor.Pos,
	}
	return []protocol.Event{resolved, detail}
}

// applyTransfer handles deposit and withdraw, which differ only in direction.
func applyTransfer(w *world.World, tick uint64, actor *world.Entity, a protocol.Action, reject rejectFn, into bool) []protocol.Event {
	target := w.GetRef(a.Target)
	if target == nil || target.Container == nil {
		return reject(protocol.NoSuchEntity)
	}
	if a.N <= 0 || !protocol.KnownItem(a.Item) {
		return reject(protocol.MalformedAction)
	}
	if !actor.Pos.Adjacent(target.Pos) {
		return reject(protocol.NotAdjacent)
	}
	if actor.Carrier == nil {
		return reject(protocol.MalformedAction)
	}

	// There is no ownership check here, and there is not meant to be one. Any
	// player may take from any container.
	src, dst := actor.Carrier.Inv, target.Container.Inv
	full, empty := protocol.ContainerFull, protocol.InsufficientItems
	if !into {
		src, dst = dst, src
		full = protocol.InventoryFull
	}

	if into && !target.Container.Accepts(a.Item) {
		return reject(protocol.WrongResource)
	}
	if src.Count(a.Item) <= 0 {
		return reject(empty)
	}
	moved := a.N
	if have := src.Count(a.Item); have < moved {
		moved = have
	}
	if fits := dst.Fits(a.Item, moved); fits < moved {
		moved = fits
	}
	if moved <= 0 {
		return reject(full)
	}

	src.Remove(a.Item, moved)
	dst.Add(a.Item, moved)

	kind := protocol.EvDeposited
	if !into {
		kind = protocol.EvWithdrew
	}
	resolved := protocol.Resolved(tick, actor.Ref(), a.Kind, true)
	resolved.Item, resolved.N, resolved.Target = a.Item, moved, target.Ref()
	if into {
		resolved.Consumed = map[protocol.Item]int{a.Item: moved}
	} else {
		resolved.Produced = map[protocol.Item]int{a.Item: moved}
	}
	detail := protocol.Event{
		Tick: tick, Kind: kind, Actor: actor.Ref(),
		Target: target.Ref(), Item: a.Item, N: moved, Pos: &target.Pos,
	}
	return []protocol.Event{resolved, detail}
}

func applySay(tick uint64, actor *world.Entity, a protocol.Action, reject rejectFn) []protocol.Event {
	if a.Text == "" || len(a.Text) > MaxSayLength {
		return reject(protocol.MalformedAction)
	}
	if actor.SayCooldown > 0 {
		return reject(protocol.RateLimited)
	}
	actor.SayCooldown = SayCooldown

	resolved := protocol.Resolved(tick, actor.Ref(), a.Kind, true)
	detail := protocol.Event{
		Tick: tick, Kind: protocol.EvSaid, Actor: actor.Ref(),
		Text: a.Text, Pos: &actor.Pos,
	}
	return []protocol.Event{resolved, detail}
}

func cancelActivity(tick uint64, actor *world.Entity) []protocol.Event {
	if actor.Activity.Idle() {
		return nil
	}
	kind := actor.Activity.Kind
	actor.Activity = world.Activity{}
	// The cancelled activity resolves unsuccessfully: it was a valid thing to
	// attempt and it produced nothing, which is exactly what the actor needs to
	// know to decide whether to start again.
	return []protocol.Event{protocol.Resolved(tick, actor.Ref(), kind, false)}
}
