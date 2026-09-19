package world

import (
	"math"
	"sort"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// NearLandmark is how close an object must be to a landmark to be described as
// being by it.
const NearLandmark = 6.0

// World is the whole simulated state: the map, the landmarks on it, and every
// entity.
//
// Entities are stored by id and iterated in ascending id order. Nothing in the
// simulation may iterate the map directly, because Go randomises map order and
// a run must be reproducible.
type World struct {
	Terrain   *Terrain
	Landmarks []Landmark

	entities map[ID]*Entity
	order    []ID
	nextID   ID
}

// New returns an empty world of the given size.
func New(w, h int) *World {
	return &World{
		Terrain:  NewTerrain(w, h),
		entities: map[ID]*Entity{},
		nextID:   1,
	}
}

// Add places e in the world, assigning it the next id.
func (w *World) Add(e *Entity) *Entity {
	e.ID = w.nextID
	w.nextID++
	w.entities[e.ID] = e
	w.order = append(w.order, e.ID)
	return e
}

// Get returns the entity with the given id, or nil.
func (w *World) Get(id ID) *Entity { return w.entities[id] }

// GetRef returns the entity named by a wire reference such as "chest_5", or
// nil.
func (w *World) GetRef(ref string) *Entity {
	id, ok := ParseRef(ref)
	if !ok {
		return nil
	}
	return w.entities[id]
}

// Remove deletes an entity.
func (w *World) Remove(id ID) {
	if _, ok := w.entities[id]; !ok {
		return
	}
	delete(w.entities, id)
	for i, got := range w.order {
		if got == id {
			w.order = append(w.order[:i], w.order[i+1:]...)
			break
		}
	}
}

// IDs returns every entity id in ascending order.
//
// This is the iteration order the tick loop resolves in. The slice is a copy, so
// callers may add or remove entities while ranging over it.
func (w *World) IDs() []ID {
	out := make([]ID, len(w.order))
	copy(out, w.order)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Each calls fn for every entity in ascending id order, stopping early if fn
// returns false.
func (w *World) Each(fn func(*Entity) bool) {
	for _, id := range w.IDs() {
		e, ok := w.entities[id]
		if !ok {
			continue
		}
		if !fn(e) {
			return
		}
	}
}

// At returns the entities standing on p, in ascending id order.
func (w *World) At(p protocol.Pos) []*Entity {
	var out []*Entity
	w.Each(func(e *Entity) bool {
		if e.Pos == p {
			out = append(out, e)
		}
		return true
	})
	return out
}

// Blocked reports whether p cannot be walked onto, whether because of terrain or
// because a blocking entity stands there.
func (w *World) Blocked(p protocol.Pos) bool {
	if w.Terrain.Blocked(p) {
		return true
	}
	blocked := false
	w.Each(func(e *Entity) bool {
		if e.Pos == p && e.Blocks() {
			blocked = true
			return false
		}
		return true
	})
	return blocked
}

// Players returns every player entity in ascending id order.
func (w *World) Players() []*Entity {
	var out []*Entity
	w.Each(func(e *Entity) bool {
		if e.Kind == KindPlayer {
			out = append(out, e)
		}
		return true
	})
	return out
}

// NearestLandmark names the landmark closest to p, or "" when none is close
// enough to be worth mentioning.
func (w *World) NearestLandmark(p protocol.Pos) string {
	best := ""
	bestDist := math.Inf(1)
	for _, l := range w.Landmarks {
		if d := l.Distance(p); d < bestDist {
			best, bestDist = l.Name, d
		}
	}
	if bestDist > NearLandmark {
		return ""
	}
	return best
}

// Distance returns the straight-line distance between two tiles.
func Distance(a, b protocol.Pos) float64 { return hypot(a.X-b.X, a.Y-b.Y) }

func hypot(dx, dy int) float64 {
	return math.Sqrt(float64(dx*dx + dy*dy))
}
