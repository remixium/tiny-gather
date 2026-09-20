package world

import (
	"testing"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

func TestSlotAccounting(t *testing.T) {
	inv := NewInventory(10) // ten slots of fifty

	if got := inv.Fits(protocol.Wood, 600); got != 500 {
		t.Fatalf("empty ten-slot inventory fits %d wood, want 500", got)
	}
	if !inv.Add(protocol.Wood, 60) {
		t.Fatal("adding 60 wood to an empty inventory failed")
	}
	if got := inv.UsedSlots(); got != 2 {
		t.Fatalf("60 wood uses %d slots, want 2", got)
	}
	if got := inv.Fits(protocol.Ore, 999); got != 400 {
		t.Fatalf("with 2 slots used, %d ore fits, want 400", got)
	}
	if inv.Add(protocol.Ore, 401) {
		t.Fatal("adding 401 ore into 8 free slots should have failed")
	}
	if got := inv.Count(protocol.Ore); got != 0 {
		t.Fatalf("a rejected add left %d ore behind; adds must be all or nothing", got)
	}
}

// TestPartialStacksAcceptMore checks the subtle half of the slot arithmetic: a
// half-filled stack can still be topped up when every slot is spoken for, but
// nothing of a different kind can be.
func TestPartialStacksAcceptMore(t *testing.T) {
	inv := NewInventory(2)
	inv.Add(protocol.Wood, 60) // one full stack, one holding ten

	if got := inv.UsedSlots(); got != 2 {
		t.Fatalf("60 wood uses %d of 2 slots, want 2", got)
	}
	if got := inv.Fits(protocol.Wood, 100); got != 40 {
		t.Fatalf("the part-filled stack accepts %d more wood, want 40", got)
	}
	if got := inv.Fits(protocol.Ore, 1); got != 0 {
		t.Fatalf("a full inventory accepted %d ore; there is no free slot", got)
	}
}

func TestRemoveIsAllOrNothing(t *testing.T) {
	inv := NewInventory(10)
	inv.Add(protocol.Wood, 3)
	if inv.Remove(protocol.Wood, 4) {
		t.Fatal("removing 4 wood from a stock of 3 should have failed")
	}
	if got := inv.Count(protocol.Wood); got != 3 {
		t.Fatalf("failed removal changed the count to %d, want 3", got)
	}
	if !inv.Remove(protocol.Wood, 3) {
		t.Fatal("removing all 3 wood failed")
	}
	if got := inv.Snapshot(); got != nil {
		t.Fatalf("emptied inventory snapshot is %v, want nil", got)
	}
}

func TestKindsAreSorted(t *testing.T) {
	inv := NewInventory(10)
	inv.Add(protocol.Ore, 1)
	inv.Add(protocol.Wood, 1)
	kinds := inv.Kinds()
	for i := 1; i < len(kinds); i++ {
		if kinds[i-1] > kinds[i] {
			t.Fatalf("Kinds() returned %v, which is not sorted", kinds)
		}
	}
}
