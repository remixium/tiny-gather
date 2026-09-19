package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPosRoundTrip(t *testing.T) {
	in := Pos{X: 12, Y: 8}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	if string(b) != "[12,8]" {
		t.Fatalf("encoded as %s, want [12,8]", b)
	}
	var out Pos
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out != in {
		t.Fatalf("round trip gave %v, want %v", out, in)
	}
}

// TestUnsuccessfulResolutionSurvivesEncoding is the reason Success is a pointer.
// As a plain bool it would be omitted when false, and an agent would see a
// resolution with no outcome at all — exactly the case the rejected/resolved
// split exists to make legible.
func TestUnsuccessfulResolutionSurvivesEncoding(t *testing.T) {
	b, err := json.Marshal(Resolved(5, "player_1", ActGather, false))
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	if !strings.Contains(string(b), `"success":false`) {
		t.Fatalf("an unsuccessful resolution encoded as %s, losing the outcome", b)
	}

	var back Event
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if back.Success == nil {
		t.Fatal("success was lost in the round trip")
	}
	if back.Succeeded() {
		t.Fatal("an unsuccessful resolution decoded as successful")
	}
}

func TestRejectionCarriesNoOutcome(t *testing.T) {
	ev := Rejected(3, "player_1", ActMove, BlockedTile)
	if ev.Success != nil {
		t.Fatal("a rejection has no outcome; nothing was attempted")
	}
	if ev.Succeeded() {
		t.Fatal("a rejection reported success")
	}
}

func TestSlotAccountingForNonStackingItems(t *testing.T) {
	tests := []struct {
		item Item
		n    int
		want int
	}{
		{Wood, 0, 0},
		{Wood, 1, 1},
		{Wood, 10, 10},
		{Ore, 7, 7},
	}
	for _, tc := range tests {
		if got := SlotsFor(tc.item, tc.n); got != tc.want {
			t.Errorf("SlotsFor(%s, %d) = %d, want %d", tc.item, tc.n, got, tc.want)
		}
	}
}

func TestDirDelta(t *testing.T) {
	tests := []struct {
		dir    Dir
		dx, dy int
		ok     bool
	}{
		{DirN, 0, -1, true},
		{DirS, 0, 1, true},
		{DirE, 1, 0, true},
		{DirW, -1, 0, true},
		{"up", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, tc := range tests {
		dx, dy, ok := tc.dir.Delta()
		if dx != tc.dx || dy != tc.dy || ok != tc.ok {
			t.Errorf("Dir(%q).Delta() = %d, %d, %v; want %d, %d, %v",
				tc.dir, dx, dy, ok, tc.dx, tc.dy, tc.ok)
		}
	}
}

func TestAdjacency(t *testing.T) {
	centre := Pos{X: 5, Y: 5}
	adjacent := []Pos{{5, 5}, {4, 5}, {6, 5}, {5, 4}, {5, 6}}
	for _, p := range adjacent {
		if !centre.Adjacent(p) {
			t.Errorf("%v should be within interaction range of %v", p, centre)
		}
	}
	// Diagonals are not adjacent, because movement is four-direction.
	for _, p := range []Pos{{4, 4}, {6, 6}, {3, 5}} {
		if centre.Adjacent(p) {
			t.Errorf("%v should not be within interaction range of %v", p, centre)
		}
	}
}
