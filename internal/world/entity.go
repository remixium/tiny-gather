package world

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// ID identifies an entity. Ids are assigned in creation order and are the
// ordering key the tick loop uses, so that resolution order never depends on
// map iteration or on the order inputs arrived.
type ID uint32

// Kind is what an entity is, for the purpose of naming and rendering it. It
// does not determine behaviour: traits do.
type Kind string

const (
	KindTree       Kind = "tree"
	KindRock       Kind = "rock"
	KindChest      Kind = "chest"
	KindStockpile  Kind = "stockpile"
	KindPlayer     Kind = "player"
	KindGroundItem Kind = "ground_item"
)

// Gatherable marks an entity that yields a resource when worked.
type Gatherable struct {
	Resource  protocol.Item
	Remaining int
	Initial   int
}

// Container marks an entity that holds items.
//
// There is deliberately no owner field. Any player may deposit into or withdraw
// from any container: a chest is a public cache, not a safe, so that agreements
// between players rest on trust rather than on a mechanic.
type Container struct {
	Color  string
	Filter protocol.Item
	Inv    *Inventory
}

// Accepts reports whether the container's filter permits it.
func (c *Container) Accepts(it protocol.Item) bool {
	return c.Filter == "" || c.Filter == it
}

// Carrier marks an entity that carries items.
type Carrier struct {
	Inv *Inventory
}

// Stack is a quantity of one item lying on the ground.
type Stack struct {
	Item protocol.Item
	N    int
}

// Activity is what a player is currently doing across ticks.
//
// Gathering is not instantaneous: it accrues progress and is cancelled by
// moving, which is what makes "go and get wood" a commitment rather than a
// single action.
type Activity struct {
	Kind     protocol.ActionKind
	Target   ID
	Progress int
}

// Idle reports whether nothing is in progress.
func (a Activity) Idle() bool { return a.Kind == "" }

// Entity is anything in the world.
//
// Behaviour comes from the trait pointers being present, not from Kind. A tree
// is positioned and gatherable; a chest is positioned with a container; a player
// is positioned with a carrier and an activity. Adding goblins or structures
// should mean setting different traits, not adding a type switch.
type Entity struct {
	ID   ID
	Kind Kind
	Name string
	Pos  protocol.Pos

	Gatherable *Gatherable
	Container  *Container
	Carrier    *Carrier
	Stack      *Stack

	Activity     Activity
	MoveCooldown int
	SayCooldown  int
	Thinking     bool
}

// Ref is the entity's identifier on the wire, for example "tree_7".
//
// Ids are readable rather than opaque because they appear in observations that
// language models reason over, and "the blue chest" is easier to connect to
// chest_5 than to an integer.
func (e *Entity) Ref() string { return fmt.Sprintf("%s_%d", e.Kind, e.ID) }

// Blocks reports whether the entity prevents movement onto its tile.
//
// Players do not block. In a world this small, mutually blocking players
// deadlock doorways and give the reflex layer a problem the game does not need.
func (e *Entity) Blocks() bool {
	switch e.Kind {
	case KindTree, KindRock, KindChest, KindStockpile:
		return true
	default:
		return false
	}
}

// Doing renders the entity's current activity for an observation, or "" when
// idle.
func (e *Entity) Doing(w *World) string {
	if e.Activity.Idle() {
		return ""
	}
	target := w.Get(e.Activity.Target)
	if target == nil {
		return string(e.Activity.Kind)
	}
	return fmt.Sprintf("%s %s", e.Activity.Kind, target.Ref())
}

// ParseRef recovers an id from a wire reference such as "chest_5".
func ParseRef(ref string) (ID, bool) {
	i := strings.LastIndexByte(ref, '_')
	if i < 0 || i == len(ref)-1 {
		return 0, false
	}
	n, err := strconv.ParseUint(ref[i+1:], 10, 32)
	if err != nil {
		return 0, false
	}
	return ID(n), true
}
