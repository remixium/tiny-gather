package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

func loadFile(t *testing.T, name string) (*World, *Fixture) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "fixtures", name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	w, f, err := LoadFixture(data)
	if err != nil {
		t.Fatalf("loading fixture: %v", err)
	}
	return w, f
}

// TestScenarioFixtureLoads checks the shipped fixture sets up the ambiguity the
// "put it in the blue chest" scenario needs, and that ids are stable enough for
// a scenario to name one.
func TestScenarioFixtureLoads(t *testing.T) {
	w, f := loadFile(t, "two-blue-chests.json")

	if f.Name != "two-blue-chests" {
		t.Errorf("fixture name is %q", f.Name)
	}

	var blue []*Entity
	w.Each(func(e *Entity) bool {
		if e.Container != nil && e.Container.Color == "blue" {
			blue = append(blue, e)
		}
		return true
	})
	if len(blue) != 2 {
		t.Fatalf("fixture has %d blue chests, want exactly 2 so the request is ambiguous", len(blue))
	}

	// The two chests must be far enough apart that "the near one" is a real
	// distinction rather than a coin flip.
	if d := Distance(blue[0].Pos, blue[1].Pos); d < 5 {
		t.Errorf("the blue chests are only %v apart; the ambiguity needs separation", d)
	}
}

// TestFixtureEntityIDsAreStable: scenarios refer to entities by name, so file
// order must decide ids.
func TestFixtureEntityIDsAreStable(t *testing.T) {
	w, _ := loadFile(t, "two-blue-chests.json")
	ids := w.IDs()
	if len(ids) == 0 {
		t.Fatal("fixture produced no entities")
	}
	first := w.Get(ids[0])
	if first.Kind != KindPlayer || first.Name != "agent" {
		t.Fatalf("first entity is %s, want the player listed first in the file", first.Ref())
	}

	again, _ := loadFile(t, "two-blue-chests.json")
	againIDs := again.IDs()
	for i := range ids {
		if w.Get(ids[i]).Ref() != again.Get(againIDs[i]).Ref() {
			t.Fatalf("entity %d differs between loads", i)
		}
	}
}

func TestFixtureRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"no size", `{"name":"x","entities":[]}`},
		{"entity off the map", `{"name":"x","w":5,"h":5,"entities":[
			{"kind":"tree","resource":"wood","remaining":1,"pos":[99,99]}]}`},
		{"unknown kind", `{"name":"x","w":5,"h":5,"entities":[
			{"kind":"dragon","pos":[1,1]}]}`},
		{"unknown resource", `{"name":"x","w":5,"h":5,"entities":[
			{"kind":"tree","resource":"mithril","remaining":1,"pos":[1,1]}]}`},
		{"contents do not fit", `{"name":"x","w":5,"h":5,"entities":[
			{"kind":"chest","pos":[1,1],"slots":2,"contents":{"wood":9}}]}`},
		{"blocked tile off the map", `{"name":"x","w":5,"h":5,"blocked":[[9,9]],"entities":[]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := LoadFixture([]byte(tc.json)); err == nil {
				t.Fatal("expected an error, got none")
			}
		})
	}
}

// TestFixtureRoundTrip checks the format a fixture is written in is the format
// it is read back from, so scenarios can be recorded as well as hand-written.
func TestFixtureRoundTrip(t *testing.T) {
	_, f := loadFile(t, "two-blue-chests.json")
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("encoding fixture: %v", err)
	}
	w2, f2, err := LoadFixture(data)
	if err != nil {
		t.Fatalf("reloading encoded fixture: %v", err)
	}
	if f2.Name != f.Name || len(w2.IDs()) != len(f.Entities) {
		t.Fatalf("round trip lost content: %d entities from %d", len(w2.IDs()), len(f.Entities))
	}
}

// TestFixturePositionsEncodeAsPairs guards the compact wire form. Positions are
// the most repeated field an agent receives.
func TestFixturePositionsEncodeAsPairs(t *testing.T) {
	b, err := json.Marshal(protocol.Pos{X: 12, Y: 8})
	if err != nil {
		t.Fatalf("encoding position: %v", err)
	}
	if string(b) != "[12,8]" {
		t.Fatalf("position encodes as %s, want [12,8]", b)
	}
}
