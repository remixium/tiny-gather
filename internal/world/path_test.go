package world

import (
	"testing"

	"github.com/vadremix/tiny-gather/internal/rng"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// TestPathCostDivergesFromDistance is the reason both numbers are reported.
// A wall between two tiles leaves the straight line unchanged while making the
// walk much longer, and deciding which measure of "nearest" to trust is a
// judgement the agent makes.
func TestPathCostDivergesFromDistance(t *testing.T) {
	w := New(11, 11)
	for y := 0; y < 10; y++ {
		w.Terrain.SetBlocked(protocol.Pos{X: 5, Y: y}, true)
	}

	from := protocol.Pos{X: 4, Y: 0}
	to := protocol.Pos{X: 6, Y: 0}

	dist := Distance(from, to)
	flood := w.Flood(from)
	cost := w.CostTo(flood, to)

	if dist != 2 {
		t.Fatalf("straight-line distance is %v, want 2", dist)
	}
	if cost <= int(dist) {
		t.Fatalf("path cost is %d but the wall should make it far longer than %v", cost, dist)
	}
	if cost != 21 {
		t.Fatalf("path cost is %d, want 21 (down 10, across 2, back up 9)", cost)
	}
}

func TestUnreachableIsReported(t *testing.T) {
	w := New(9, 9)
	for y := 0; y < 9; y++ {
		w.Terrain.SetBlocked(protocol.Pos{X: 4, Y: y}, true)
	}
	flood := w.Flood(protocol.Pos{X: 0, Y: 0})
	if got := w.CostTo(flood, protocol.Pos{X: 8, Y: 8}); got != Unreachable {
		t.Fatalf("cost across a full wall is %d, want %d", got, Unreachable)
	}
}

// TestCostToBlockingEntity checks that things you cannot stand on are still
// reachable: you interact with a chest from beside it, not on top of it.
func TestCostToBlockingEntity(t *testing.T) {
	w := New(9, 9)
	chest := w.Add(&Entity{
		Kind:      KindChest,
		Pos:       protocol.Pos{X: 4, Y: 4},
		Container: &Container{Color: "blue", Inv: NewInventory(12)},
	})
	flood := w.Flood(protocol.Pos{X: 0, Y: 4})
	if got := w.CostTo(flood, chest.Pos); got != 3 {
		t.Fatalf("cost to a chest four tiles away is %d, want 3 (its near side)", got)
	}
	if !w.Blocked(chest.Pos) {
		t.Fatal("a chest should block its own tile")
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	a := Generate(rng.New(2024))
	b := Generate(rng.New(2024))

	idsA, idsB := a.IDs(), b.IDs()
	if len(idsA) != len(idsB) {
		t.Fatalf("same seed produced %d and %d entities", len(idsA), len(idsB))
	}
	for i := range idsA {
		ea, eb := a.Get(idsA[i]), b.Get(idsB[i])
		if ea.Ref() != eb.Ref() || ea.Pos != eb.Pos {
			t.Fatalf("entity %d differs: %s at %s versus %s at %s",
				i, ea.Ref(), ea.Pos, eb.Ref(), eb.Pos)
		}
	}
}

// TestGeneratedWorldIsAmbiguous checks the generator always produces the
// ambiguity the "put it in the blue chest" scenario depends on, rather than
// leaving it to a lucky seed.
func TestGeneratedWorldIsAmbiguous(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3, 99, 12345} {
		w := Generate(rng.New(seed))
		blue := 0
		filtered := 0
		w.Each(func(e *Entity) bool {
			if e.Container != nil && e.Container.Color == "blue" {
				blue++
			}
			if e.Container != nil && e.Container.Filter != "" {
				filtered++
			}
			return true
		})
		if blue < 2 {
			t.Fatalf("seed %d produced %d blue chests, want at least 2", seed, blue)
		}
		if filtered < 1 {
			t.Fatalf("seed %d produced no filtered container", seed)
		}
	}
}
