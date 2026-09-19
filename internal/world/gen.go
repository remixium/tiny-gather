package world

import (
	"github.com/vadremix/tiny-gather/internal/rng"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// Dimensions and populations for a generated world.
const (
	GenWidth  = 40
	GenHeight = 30

	GenTrees = 18
	GenRocks = 12

	TreeYield = 5
	RockYield = 8

	ChestSlots      = 12
	StockpileSlots  = 200
	PlayerSlots     = 10
	PlayerSpawnArea = 4
)

// Generate builds a world from a random source.
//
// Every placement draws from r in a fixed order, so the same seed always yields
// the same world. Obstacles are placed before resources so that the scatter
// cannot land on them.
func Generate(r *rng.Rand) *World {
	w := New(GenWidth, GenHeight)

	house := Landmark{Name: "house", Pos: protocol.Pos{X: 6, Y: 5}, W: 6, H: 5}
	pond := Landmark{Name: "pond", Pos: protocol.Pos{X: 24, Y: 16}, W: 7, H: 6}
	fence := Landmark{Name: "fence", Pos: protocol.Pos{X: 14, Y: 3}, W: 1, H: 14}
	w.Landmarks = []Landmark{house, pond, fence}

	// The house is a hollow building with a doorway, the pond is solid, and the
	// fence is a wall with one gap in it. All three exist so that the walk to
	// something is meaningfully longer than the line to it.
	for x := house.Pos.X; x < house.Pos.X+house.W; x++ {
		for y := house.Pos.Y; y < house.Pos.Y+house.H; y++ {
			edge := x == house.Pos.X || x == house.Pos.X+house.W-1 ||
				y == house.Pos.Y || y == house.Pos.Y+house.H-1
			door := x == house.Pos.X+house.W/2 && y == house.Pos.Y+house.H-1
			if edge && !door {
				w.Terrain.SetBlocked(protocol.Pos{X: x, Y: y}, true)
			}
		}
	}
	for x := pond.Pos.X; x < pond.Pos.X+pond.W; x++ {
		for y := pond.Pos.Y; y < pond.Pos.Y+pond.H; y++ {
			w.Terrain.SetBlocked(protocol.Pos{X: x, Y: y}, true)
		}
	}
	gap := fence.Pos.Y + fence.H/2
	for y := fence.Pos.Y; y < fence.Pos.Y+fence.H; y++ {
		if y != gap {
			w.Terrain.SetBlocked(protocol.Pos{X: fence.Pos.X, Y: y}, true)
		}
	}

	// Containers are placed at fixed spots rather than scattered. Two of them
	// share a colour on purpose, so that "put it in the blue chest" is ambiguous
	// in every generated world rather than only in a lucky one.
	w.Add(&Entity{
		Kind:      KindChest,
		Pos:       protocol.Pos{X: 13, Y: 9},
		Container: &Container{Color: "blue", Inv: NewInventory(ChestSlots)},
	})
	w.Add(&Entity{
		Kind:      KindChest,
		Pos:       protocol.Pos{X: 23, Y: 15},
		Container: &Container{Color: "blue", Inv: NewInventory(ChestSlots)},
	})
	w.Add(&Entity{
		Kind:      KindChest,
		Pos:       protocol.Pos{X: 9, Y: 12},
		Container: &Container{Color: "red", Filter: protocol.Ore, Inv: NewInventory(ChestSlots)},
	})
	w.Add(&Entity{
		Kind:      KindStockpile,
		Pos:       protocol.Pos{X: 11, Y: 11},
		Container: &Container{Color: "grey", Inv: NewInventory(StockpileSlots)},
	})

	for i := 0; i < GenTrees; i++ {
		if p, ok := FreeTile(w, r); ok {
			w.Add(&Entity{
				Kind:       KindTree,
				Pos:        p,
				Gatherable: &Gatherable{Resource: protocol.Wood, Remaining: TreeYield, Initial: TreeYield},
			})
		}
	}
	for i := 0; i < GenRocks; i++ {
		if p, ok := FreeTile(w, r); ok {
			w.Add(&Entity{
				Kind:       KindRock,
				Pos:        p,
				Gatherable: &Gatherable{Resource: protocol.Ore, Remaining: RockYield, Initial: RockYield},
			})
		}
	}

	return w
}

// AddPlayer puts a named player into the world.
func AddPlayer(w *World, name string, p protocol.Pos) *Entity {
	return w.Add(&Entity{
		Kind:    KindPlayer,
		Name:    name,
		Pos:     p,
		Carrier: &Carrier{Inv: NewInventory(PlayerSlots)},
	})
}

// Spawn returns a free tile near the house for a joining player.
func Spawn(w *World, r *rng.Rand) protocol.Pos {
	var anchor protocol.Pos
	for _, l := range w.Landmarks {
		if l.Name == "house" {
			anchor = l.Center()
		}
	}
	var candidates []protocol.Pos
	for dx := -PlayerSpawnArea; dx <= PlayerSpawnArea; dx++ {
		for dy := -PlayerSpawnArea; dy <= PlayerSpawnArea; dy++ {
			p := protocol.Pos{X: anchor.X + dx, Y: anchor.Y + dy}
			if w.Terrain.In(p) && !w.Blocked(p) {
				candidates = append(candidates, p)
			}
		}
	}
	if len(candidates) == 0 {
		if p, ok := FreeTile(w, r); ok {
			return p
		}
		return protocol.Pos{}
	}
	return candidates[r.IntN(len(candidates))]
}

// FreeTile picks an unoccupied walkable tile.
//
// Candidates are gathered by scanning the map in index order rather than by
// rejection sampling, so the number of draws taken from r does not depend on how
// crowded the world happens to be.
func FreeTile(w *World, r *rng.Rand) (protocol.Pos, bool) {
	var candidates []protocol.Pos
	for y := 0; y < w.Terrain.H; y++ {
		for x := 0; x < w.Terrain.W; x++ {
			p := protocol.Pos{X: x, Y: y}
			if !w.Blocked(p) && len(w.At(p)) == 0 {
				candidates = append(candidates, p)
			}
		}
	}
	if len(candidates) == 0 {
		return protocol.Pos{}, false
	}
	return candidates[r.IntN(len(candidates))], true
}
