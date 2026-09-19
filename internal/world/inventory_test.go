package world

import (
	"testing"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

func TestSlotAccounting(t *testing.T) {
	inv := NewInventory(10)

	if got := inv.Fits(protocol.Wood, 99); got != 10 {
		t.Fatalf("empty ten-slot inventory fits %d wood, want 10", got)
	}
	if !inv.Add(protocol.Wood, 6) {
		t.Fatal("adding 6 wood to an empty inventory failed")
	}
	if got := inv.UsedSlots(); got != 6 {
		t.Fatalf("6 non-stacking wood uses %d slots, want 6", got)
	}
	if got := inv.Fits(protocol.Ore, 99); got != 4 {
		t.Fatalf("with 6 slots used, %d ore fits, want 4", got)
	}
	if inv.Add(protocol.Ore, 5) {
		t.Fatal("adding 5 ore into 4 free slots should have failed")
	}
	if got := inv.Count(protocol.Ore); got != 0 {
		t.Fatalf("a rejected add left %d ore behind; adds must be all or nothing", got)
	}
}

// TestCapacityBitesAtTen is the rule behind the "bring me 15 ore" scenario. If
// resources ever start stacking, the cap stops creating a decision and that
// scenario quietly stops testing anything.
func TestCapacityBitesAtTen(t *testing.T) {
	inv := NewInventory(10)
	if got := inv.Fits(protocol.Ore, 15); got != 10 {
		t.Fatalf("a ten-slot inventory accepts %d of 15 ore, want 10", got)
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
