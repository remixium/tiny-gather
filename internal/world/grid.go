package world

import "github.com/vadremix/tiny-gather/pkg/protocol"

// Terrain is the static map: what is impassable before any entity is placed on
// it. The pond and the fence line live here; trees and chests do not, because
// they come and go.
type Terrain struct {
	W, H    int
	blocked []bool
}

// NewTerrain returns an entirely walkable map of the given size.
func NewTerrain(w, h int) *Terrain {
	return &Terrain{W: w, H: h, blocked: make([]bool, w*h)}
}

// In reports whether p is inside the map.
func (t *Terrain) In(p protocol.Pos) bool {
	return p.X >= 0 && p.Y >= 0 && p.X < t.W && p.Y < t.H
}

// Index returns the flat index of p, which must be inside the map.
func (t *Terrain) Index(p protocol.Pos) int { return p.Y*t.W + p.X }

// Blocked reports whether the terrain at p is impassable. Positions outside the
// map are blocked.
func (t *Terrain) Blocked(p protocol.Pos) bool {
	if !t.In(p) {
		return true
	}
	return t.blocked[t.Index(p)]
}

// SetBlocked marks p impassable or clear.
func (t *Terrain) SetBlocked(p protocol.Pos, blocked bool) {
	if t.In(p) {
		t.blocked[t.Index(p)] = blocked
	}
}

// Landmark is a named region that exists so that phrases like "the chest by the
// house" refer to something.
type Landmark struct {
	Name string       `json:"name"`
	Pos  protocol.Pos `json:"pos"`
	W    int          `json:"w"`
	H    int          `json:"h"`
}

// Contains reports whether p lies inside the landmark.
func (l Landmark) Contains(p protocol.Pos) bool {
	return p.X >= l.Pos.X && p.X < l.Pos.X+l.W &&
		p.Y >= l.Pos.Y && p.Y < l.Pos.Y+l.H
}

// Distance returns the straight-line distance from p to the landmark's nearest
// edge, or 0 when p is inside it.
func (l Landmark) Distance(p protocol.Pos) float64 {
	dx := 0
	switch {
	case p.X < l.Pos.X:
		dx = l.Pos.X - p.X
	case p.X >= l.Pos.X+l.W:
		dx = p.X - (l.Pos.X + l.W - 1)
	}
	dy := 0
	switch {
	case p.Y < l.Pos.Y:
		dy = l.Pos.Y - p.Y
	case p.Y >= l.Pos.Y+l.H:
		dy = p.Y - (l.Pos.Y + l.H - 1)
	}
	return hypot(dx, dy)
}

// Center returns the landmark's middle tile.
func (l Landmark) Center() protocol.Pos {
	return protocol.Pos{X: l.Pos.X + l.W/2, Y: l.Pos.Y + l.H/2}
}
