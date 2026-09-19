package observe

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

func testWorld() (*world.World, *world.Entity) {
	w := world.New(60, 60)
	w.Landmarks = []world.Landmark{
		{Name: "house", Pos: protocol.Pos{X: 28, Y: 28}, W: 4, H: 4},
	}
	viewer := world.AddPlayer(w, "viewer", protocol.Pos{X: 30, Y: 30})
	return w, viewer
}

// TestRadiusLimitsWhatIsSeen is the rule that makes checking on someone cost a
// walk. Without it, noticing that a promised item never arrived would be
// arithmetic rather than something an agent has to go and find out.
func TestRadiusLimitsWhatIsSeen(t *testing.T) {
	w, viewer := testWorld()
	near := w.Add(&world.Entity{
		Kind: world.KindTree, Pos: protocol.Pos{X: 35, Y: 30},
		Gatherable: &world.Gatherable{Resource: protocol.Wood, Remaining: 5},
	})
	far := w.Add(&world.Entity{
		Kind: world.KindTree, Pos: protocol.Pos{X: 50, Y: 30},
		Gatherable: &world.Gatherable{Resource: protocol.Wood, Remaining: 5},
	})

	obs := Build(w, 1, viewer.ID)

	seen := map[string]bool{}
	for _, o := range obs.Object {
		seen[o.ID] = true
	}
	if !seen[near.Ref()] {
		t.Errorf("tree 5 tiles away was not reported")
	}
	if seen[far.Ref()] {
		t.Errorf("tree 20 tiles away was reported; the radius is %v", Radius)
	}
}

func TestLandmarksAreAlwaysReported(t *testing.T) {
	w, viewer := testWorld()
	w.Landmarks = append(w.Landmarks, world.Landmark{
		Name: "pond", Pos: protocol.Pos{X: 0, Y: 0}, W: 3, H: 3,
	})

	obs := Build(w, 1, viewer.ID)
	names := map[string]bool{}
	for _, l := range obs.Landmark {
		names[l.Name] = true
	}
	if !names["pond"] {
		t.Error("a landmark far outside the radius was omitted; landmarks do not stop existing")
	}
}

// TestNoEmpiricalFactsLeak guards the information rule. Anything an agent is
// supposed to learn from experience must never appear in an observation, and
// this is easiest to break by accident when adding a field.
func TestNoEmpiricalFactsLeak(t *testing.T) {
	w, viewer := testWorld()
	w.Add(&world.Entity{
		Kind: world.KindTree, Pos: protocol.Pos{X: 32, Y: 30},
		Gatherable: &world.Gatherable{Resource: protocol.Wood, Remaining: 3, Initial: 5},
	})
	w.Add(&world.Entity{
		Kind: world.KindChest, Pos: protocol.Pos{X: 31, Y: 30},
		Container: &world.Container{Color: "blue", Inv: world.NewInventory(12)},
	})

	encoded, err := json.Marshal(Build(w, 1, viewer.ID))
	if err != nil {
		t.Fatalf("encoding observation: %v", err)
	}
	for _, banned := range []string{
		"probability", "chance", "odds", "rate", "drop_rate", "success_rate", "damage",
	} {
		if strings.Contains(strings.ToLower(string(encoded)), banned) {
			t.Errorf("observation contains %q, which is an empirical fact agents must learn", banned)
		}
	}
}

// TestDeclarativeFactsArePresent is the other half of the same rule: what a
// character would simply know has to be there, or the agent is guessing at
// things it should not have to.
func TestDeclarativeFactsArePresent(t *testing.T) {
	w, viewer := testWorld()
	chest := w.Add(&world.Entity{
		Kind: world.KindChest, Pos: protocol.Pos{X: 31, Y: 30},
		Container: &world.Container{
			Color: "red", Filter: protocol.Ore, Inv: world.NewInventory(12),
		},
	})

	obs := Build(w, 1, viewer.ID)
	var view *protocol.ObjectView
	for i := range obs.Object {
		if obs.Object[i].ID == chest.Ref() {
			view = &obs.Object[i]
		}
	}
	if view == nil {
		t.Fatal("the adjacent chest was not reported at all")
	}
	switch {
	case view.Color != "red":
		t.Errorf("colour is %q, want red", view.Color)
	case view.Filter != protocol.Ore:
		t.Errorf("filter is %q, want ore", view.Filter)
	case view.FreeSlots != 12:
		t.Errorf("free slots is %d, want 12", view.FreeSlots)
	case view.Near != "house":
		t.Errorf("nearest landmark is %q, want house", view.Near)
	}
}

// TestNoOwnershipIsReported: ownership was removed deliberately, so nothing may
// reintroduce it through the observation.
func TestNoOwnershipIsReported(t *testing.T) {
	w, viewer := testWorld()
	w.Add(&world.Entity{
		Kind: world.KindChest, Pos: protocol.Pos{X: 31, Y: 30},
		Container: &world.Container{Color: "blue", Inv: world.NewInventory(12)},
	})
	encoded, err := json.Marshal(Build(w, 1, viewer.ID))
	if err != nil {
		t.Fatalf("encoding observation: %v", err)
	}
	if strings.Contains(string(encoded), "owner") {
		t.Errorf("observation mentions an owner: %s", encoded)
	}
}

// TestOtherPlayersInventoryIsPrivate: what someone is carrying cannot be read
// off them. Once equipment exists, showing something will be a real action.
func TestOtherPlayersInventoryIsPrivate(t *testing.T) {
	w, viewer := testWorld()
	other := world.AddPlayer(w, "sam", protocol.Pos{X: 33, Y: 30})
	other.Carrier.Inv.Add(protocol.Ore, 7)

	obs := Build(w, 1, viewer.ID)
	if len(obs.Player) != 1 {
		t.Fatalf("saw %d other players, want 1", len(obs.Player))
	}
	encoded, err := json.Marshal(obs.Player[0])
	if err != nil {
		t.Fatalf("encoding player view: %v", err)
	}
	if strings.Contains(string(encoded), "ore") {
		t.Errorf("another player's carried items were visible: %s", encoded)
	}
}

// TestEventsAreFilteredByRange: a deposit on the far side of the map produces
// nothing for a distant watcher, which is precisely why checking on someone
// means going there.
func TestEventsAreFilteredByRange(t *testing.T) {
	w, viewer := testWorld()
	farPos := protocol.Pos{X: 55, Y: 55}
	nearPos := protocol.Pos{X: 33, Y: 30}

	events := []protocol.Event{
		{Tick: 1, Kind: protocol.EvSaid, Actor: "player_99", Text: "far away", Pos: &farPos},
		{Tick: 1, Kind: protocol.EvSaid, Actor: "player_98", Text: "close by", Pos: &nearPos},
		{Tick: 1, Kind: protocol.EvRejected, Actor: viewer.Ref(), Reason: protocol.BlockedTile},
	}

	got := ForPlayer(w, viewer.ID, events)
	if len(got) != 2 {
		t.Fatalf("delivered %d events, want 2", len(got))
	}
	for _, ev := range got {
		if ev.Text == "far away" {
			t.Error("an event beyond the radius was delivered")
		}
	}

	var ownFeedback bool
	for _, ev := range got {
		if ev.Actor == viewer.Ref() {
			ownFeedback = true
		}
	}
	if !ownFeedback {
		t.Error("the viewer's own rejection was withheld; action feedback is never range-limited")
	}
}

// TestObservationStaysCompact: observations are billed per token when they reach
// an agent, so the encoded size of a busy view is worth watching.
func TestObservationStaysCompact(t *testing.T) {
	w, viewer := testWorld()
	for i := 0; i < 20; i++ {
		w.Add(&world.Entity{
			Kind: world.KindTree, Pos: protocol.Pos{X: 26 + i%8, Y: 26 + i/8},
			Gatherable: &world.Gatherable{Resource: protocol.Wood, Remaining: 5},
		})
	}
	encoded, err := json.Marshal(Build(w, 1, viewer.ID))
	if err != nil {
		t.Fatalf("encoding observation: %v", err)
	}
	if len(encoded) > 4096 {
		t.Errorf("an observation of 20 nearby objects encodes to %d bytes, which is larger "+
			"than intended; check for fields that should be omitted when empty", len(encoded))
	}
	t.Logf("20-object observation encodes to %d bytes", len(encoded))
}
