package protocol

// Item identifies a kind of carryable thing.
type Item string

// Items carryable in v1. Gold and the hammer arrive with v2; the stack table
// below is the only place that needs to learn about them.
const (
	Wood Item = "wood"
	Ore  Item = "ore"
)

// stackSizes records how many of an item occupy a single inventory slot. An
// item missing from the table does not stack.
//
// Resources deliberately do not stack, which keeps a player's effective
// capacity at ten items and preserves the decision the cap exists to create:
// asked for fifteen ore, an agent has to come back with ten and say so. Slots
// exist for the sake of currency, which stacks deeply and would be uncarryable
// under a flat item cap.
var stackSizes = map[Item]int{
	Wood: 1,
	Ore:  1,
}

// AllItems lists every defined item in a stable order.
//
// Callers that iterate items use this rather than ranging over a map, so that
// behaviour does not vary between runs.
func AllItems() []Item { return []Item{Wood, Ore} }

// KnownItem reports whether it is a defined item.
func KnownItem(it Item) bool {
	_, ok := stackSizes[it]
	return ok
}

// StackSize returns how many of it fit in one slot.
func StackSize(it Item) int {
	if n, ok := stackSizes[it]; ok {
		return n
	}
	return 1
}

// SlotsFor returns how many slots n of it occupies.
func SlotsFor(it Item, n int) int {
	if n <= 0 {
		return 0
	}
	s := StackSize(it)
	return (n + s - 1) / s
}
