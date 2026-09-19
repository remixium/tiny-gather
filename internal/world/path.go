package world

import "github.com/vadremix/tiny-gather/pkg/protocol"

// Unreachable is the path cost reported for a tile no walk can reach.
const Unreachable = -1

// Flood returns the number of steps needed to walk from origin to every tile,
// with Unreachable where no route exists.
//
// A breadth-first flood is used rather than A* per target because every step
// costs the same and an observation needs the cost to many objects at once: one
// pass answers for the whole map, where A* would re-search per object.
func (w *World) Flood(origin protocol.Pos) []int {
	t := w.Terrain
	cost := make([]int, t.W*t.H)
	for i := range cost {
		cost[i] = Unreachable
	}
	if !t.In(origin) {
		return cost
	}

	// Blocking entities are collected once, rather than rescanned per tile.
	blocked := make([]bool, t.W*t.H)
	w.Each(func(e *Entity) bool {
		if e.Blocks() && t.In(e.Pos) {
			blocked[t.Index(e.Pos)] = true
		}
		return true
	})

	cost[t.Index(origin)] = 0
	queue := []protocol.Pos{origin}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		here := cost[t.Index(p)]
		for _, d := range []protocol.Dir{protocol.DirN, protocol.DirS, protocol.DirE, protocol.DirW} {
			dx, dy, _ := d.Delta()
			n := protocol.Pos{X: p.X + dx, Y: p.Y + dy}
			if !t.In(n) {
				continue
			}
			i := t.Index(n)
			if cost[i] != Unreachable || t.Blocked(n) || blocked[i] {
				continue
			}
			cost[i] = here + 1
			queue = append(queue, n)
		}
	}
	return cost
}

// CostTo returns the steps needed to reach a position where the target can be
// interacted with, given a flood from the actor.
//
// Interaction happens from an adjacent tile, so for anything that blocks its own
// tile the answer is the cheapest neighbour rather than the tile itself. A chest
// is never stood on, but it is still reachable.
func (w *World) CostTo(flood []int, target protocol.Pos) int {
	t := w.Terrain
	best := Unreachable
	consider := func(p protocol.Pos) {
		if !t.In(p) {
			return
		}
		c := flood[t.Index(p)]
		if c == Unreachable {
			return
		}
		if best == Unreachable || c < best {
			best = c
		}
	}
	consider(target)
	for _, d := range []protocol.Dir{protocol.DirN, protocol.DirS, protocol.DirE, protocol.DirW} {
		dx, dy, _ := d.Delta()
		consider(protocol.Pos{X: target.X + dx, Y: target.Y + dy})
	}
	return best
}
