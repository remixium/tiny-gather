package protocol

// ActionKind names something a player may attempt.
type ActionKind string

const (
	ActMove     ActionKind = "move"
	ActGather   ActionKind = "gather"
	ActDrop     ActionKind = "drop"
	ActPickup   ActionKind = "pickup"
	ActDeposit  ActionKind = "deposit"
	ActWithdraw ActionKind = "withdraw"
	ActSay      ActionKind = "say"
)

// Dir is a movement direction. Movement is four-direction, which keeps the
// reflex action space small.
type Dir string

const (
	DirN Dir = "N"
	DirS Dir = "S"
	DirE Dir = "E"
	DirW Dir = "W"
)

// Delta returns the tile offset for d, and whether d was a valid direction.
func (d Dir) Delta() (dx, dy int, ok bool) {
	switch d {
	case DirN:
		return 0, -1, true
	case DirS:
		return 0, 1, true
	case DirE:
		return 1, 0, true
	case DirW:
		return -1, 0, true
	default:
		return 0, 0, false
	}
}

// Action is a single attempt by a player. Fields not relevant to the kind are
// left zero.
type Action struct {
	Kind   ActionKind `json:"kind"`
	Dir    Dir        `json:"dir,omitempty"`
	Target string     `json:"target,omitempty"`
	Item   Item       `json:"item,omitempty"`
	N      int        `json:"n,omitempty"`
	Text   string     `json:"text,omitempty"`
}
