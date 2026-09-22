package observe

import (
	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// Radius is how far a player perceives, in tiles.
//
// Limiting perception is what makes trust a real problem rather than a lookup.
// Confirming that someone left something in a chest requires walking there, and
// that walk is the window in which they can act unseen. With a global view,
// noticing a broken promise would be arithmetic.
const Radius = 12.0

// Build assembles what the viewing player can currently perceive.
//
// All geometry is done here. Path costs come from a single breadth-first flood
// from the viewer, so the cost to every object is known after one pass.
func Build(w *world.World, tick uint64, viewer world.ID) protocol.Observation {
	self := w.Get(viewer)
	if self == nil {
		return protocol.Observation{Tick: tick}
	}

	obs := protocol.Observation{
		Tick: tick,
		Self: protocol.SelfView{
			ID:    self.Ref(),
			Name:  self.Name,
			Pos:   self.Pos,
			Doing: self.Doing(w),
		},
	}
	if self.Carrier != nil {
		obs.Self.Slots = self.Carrier.Inv.Slots
		obs.Self.UsedSlots = self.Carrier.Inv.UsedSlots()
		obs.Self.Inventory = self.Carrier.Inv.Snapshot()
	}

	// Landmarks are reported regardless of range. A house does not stop existing
	// because you walked away from it, and phrases like "the chest by the house"
	// have to keep working at a distance.
	for _, l := range w.Landmarks {
		obs.Landmark = append(obs.Landmark, protocol.LandmarkView{Name: l.Name, Pos: l.Center()})
	}

	flood := w.Flood(self.Pos)
	w.Each(func(e *world.Entity) bool {
		if e.ID == viewer {
			return true
		}
		dist := world.Distance(self.Pos, e.Pos)
		if dist > Radius {
			return true
		}
		cost := w.CostTo(flood, e.Pos)

		if e.Kind == world.KindPlayer {
			obs.Player = append(obs.Player, protocol.PlayerView{
				ID:       e.Ref(),
				Name:     e.Name,
				Pos:      e.Pos,
				Distance: dist,
				PathCost: cost,
				Doing:    e.Doing(w),
				Thinking: e.Thinking,
			})
			return true
		}

		view := protocol.ObjectView{
			ID:       e.Ref(),
			Type:     string(e.Kind),
			Pos:      e.Pos,
			Distance: dist,
			PathCost: cost,
			Near:     w.NearestLandmark(e.Pos),
		}
		// Only declarative facts go in: what a thing is, what it holds, what it
		// still has, what it will accept. Nothing empirical — no yields per
		// attempt, no odds — because those have to be learned from outcomes.
		if g := e.Gatherable; g != nil {
			view.Remaining = g.Remaining
			view.Resource = g.Resource
		}
		if c := e.Container; c != nil {
			view.Color = c.Color
			view.Filter = c.Filter
			view.Contents = c.Inv.Snapshot()
			view.FreeSlots = c.Inv.FreeSlots()
		}
		if s := e.Stack; s != nil {
			view.Item = s.Item
			view.Count = s.N
		}
		obs.Object = append(obs.Object, view)
		return true
	})

	return obs
}

// Map describes the static terrain for a client that has just joined.
func Map(w *world.World) protocol.MapView {
	t := w.Terrain
	view := protocol.MapView{W: t.W, H: t.H, Radius: Radius}
	for y := 0; y < t.H; y++ {
		for x := 0; x < t.W; x++ {
			p := protocol.Pos{X: x, Y: y}
			if t.Blocked(p) {
				view.Blocked = append(view.Blocked, p)
			}
		}
	}
	return view
}

// ForPlayer selects the events a player perceives.
//
// A player always receives the outcome of their own actions, whether or not
// anything visible came of it. Everything else has to happen somewhere they can
// see: a deposit made on the far side of the map produces no event for them,
// which is precisely why checking on someone requires going there.
func ForPlayer(w *world.World, viewer world.ID, events []protocol.Event) []protocol.Event {
	self := w.Get(viewer)
	if self == nil {
		return nil
	}
	ref := self.Ref()

	var out []protocol.Event
	for _, ev := range events {
		switch {
		case ev.Actor == ref:
			out = append(out, ev)
		case ev.Pos != nil && world.Distance(self.Pos, *ev.Pos) <= Radius:
			out = append(out, ev)
		}
	}
	return out
}
