package world

import (
	"sort"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// Inventory holds items against a slot budget.
//
// Capacity is measured in slots rather than in items, because a flat item cap
// cannot accommodate things that stack very differently. Two hundred wood
// occupies four slots; the same count of an unstackable item would not fit at
// all.
type Inventory struct {
	Slots  int
	counts map[protocol.Item]int
}

// NewInventory returns an empty inventory with the given slot capacity.
func NewInventory(slots int) *Inventory {
	return &Inventory{Slots: slots, counts: map[protocol.Item]int{}}
}

// Count returns how many of it are held.
func (inv *Inventory) Count(it protocol.Item) int { return inv.counts[it] }

// Total returns the number of items held, across all kinds.
func (inv *Inventory) Total() int {
	n := 0
	for _, c := range inv.counts {
		n += c
	}
	return n
}

// UsedSlots returns how many slots the contents occupy.
func (inv *Inventory) UsedSlots() int {
	used := 0
	for it, n := range inv.counts {
		used += protocol.SlotsFor(it, n)
	}
	return used
}

// FreeSlots returns how many slots remain.
func (inv *Inventory) FreeSlots() int {
	free := inv.Slots - inv.UsedSlots()
	if free < 0 {
		return 0
	}
	return free
}

// Fits returns how many of it can still be accepted, which may be zero.
func (inv *Inventory) Fits(it protocol.Item, n int) int {
	if n <= 0 {
		return 0
	}
	before := protocol.SlotsFor(it, inv.counts[it])
	room := inv.Slots - inv.UsedSlots() + before
	if room <= 0 {
		return 0
	}
	capacity := room*protocol.StackSize(it) - inv.counts[it]
	if capacity < 0 {
		capacity = 0
	}
	if n < capacity {
		return n
	}
	return capacity
}

// Add stores n of it, reporting whether it all fit. Nothing is stored unless
// all of it fits.
func (inv *Inventory) Add(it protocol.Item, n int) bool {
	if n <= 0 || inv.Fits(it, n) < n {
		return false
	}
	inv.counts[it] += n
	return true
}

// Remove takes n of it, reporting whether that many were present. Nothing is
// removed unless all of it is present.
func (inv *Inventory) Remove(it protocol.Item, n int) bool {
	if n <= 0 || inv.counts[it] < n {
		return false
	}
	inv.counts[it] -= n
	if inv.counts[it] == 0 {
		delete(inv.counts, it)
	}
	return true
}

// Kinds lists the items held, sorted, so that callers iterating contents do not
// depend on map order.
func (inv *Inventory) Kinds() []protocol.Item {
	kinds := make([]protocol.Item, 0, len(inv.counts))
	for it := range inv.counts {
		kinds = append(kinds, it)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	return kinds
}

// Snapshot returns a copy of the contents, or nil when empty.
func (inv *Inventory) Snapshot() map[protocol.Item]int {
	if len(inv.counts) == 0 {
		return nil
	}
	out := make(map[protocol.Item]int, len(inv.counts))
	for it, n := range inv.counts {
		out[it] = n
	}
	return out
}
