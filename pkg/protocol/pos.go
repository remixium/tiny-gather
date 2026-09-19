package protocol

import (
	"encoding/json"
	"fmt"
)

// Pos is a tile coordinate.
//
// It marshals as a two-element array ([12, 8]) rather than an object, because
// positions are the most repeated field in an observation and agents are billed
// per token for what they receive.
type Pos struct {
	X int
	Y int
}

// MarshalJSON encodes p as [x, y].
func (p Pos) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]int{p.X, p.Y})
}

// UnmarshalJSON decodes [x, y].
func (p *Pos) UnmarshalJSON(b []byte) error {
	var a [2]int
	if err := json.Unmarshal(b, &a); err != nil {
		return fmt.Errorf("decoding position: %w", err)
	}
	p.X, p.Y = a[0], a[1]
	return nil
}

func (p Pos) String() string { return fmt.Sprintf("(%d,%d)", p.X, p.Y) }

// Manhattan returns the four-direction step distance between p and q.
func (p Pos) Manhattan(q Pos) int {
	return abs(p.X-q.X) + abs(p.Y-q.Y)
}

// Adjacent reports whether an actor at p can interact with something at q.
//
// Interaction range is the tile itself or one of its four orthogonal
// neighbours, matching the four-direction movement model.
func (p Pos) Adjacent(q Pos) bool { return p.Manhattan(q) <= 1 }

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
