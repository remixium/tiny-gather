package world

import (
	"encoding/json"
	"fmt"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// Fixture is a world written out by hand rather than generated.
//
// Scenarios need exact situations — two blue chests at known distances, a tree
// with two wood left — and hunting for a seed that happens to produce one is
// both slow and brittle. A fixture bypasses generation entirely. Entities are
// created in file order, so ids are stable and a scenario can refer to
// "chest_2" and mean it.
type Fixture struct {
	Name      string          `json:"name"`
	W         int             `json:"w"`
	H         int             `json:"h"`
	Blocked   []protocol.Pos  `json:"blocked,omitempty"`
	Landmarks []Landmark      `json:"landmarks,omitempty"`
	Entities  []FixtureEntity `json:"entities"`
}

// FixtureEntity describes one entity to create. Fields not relevant to the kind
// are omitted.
type FixtureEntity struct {
	Kind Kind         `json:"kind"`
	Name string       `json:"name,omitempty"`
	Pos  protocol.Pos `json:"pos"`

	Resource  protocol.Item `json:"resource,omitempty"`
	Remaining int           `json:"remaining,omitempty"`

	Color    string                `json:"color,omitempty"`
	Filter   protocol.Item         `json:"filter,omitempty"`
	Slots    int                   `json:"slots,omitempty"`
	Contents map[protocol.Item]int `json:"contents,omitempty"`

	Item  protocol.Item `json:"item,omitempty"`
	Count int           `json:"count,omitempty"`
}

// LoadFixture builds a world from fixture JSON.
func LoadFixture(data []byte) (*World, *Fixture, error) {
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, nil, fmt.Errorf("parsing fixture: %w", err)
	}
	w, err := f.Build()
	if err != nil {
		return nil, nil, err
	}
	return w, &f, nil
}

// Build turns the fixture into a world.
func (f *Fixture) Build() (*World, error) {
	if f.W <= 0 || f.H <= 0 {
		return nil, fmt.Errorf("fixture %q: size must be positive, got %dx%d", f.Name, f.W, f.H)
	}
	w := New(f.W, f.H)
	w.Landmarks = f.Landmarks
	for _, p := range f.Blocked {
		if !w.Terrain.In(p) {
			return nil, fmt.Errorf("fixture %q: blocked tile %s is off the map", f.Name, p)
		}
		w.Terrain.SetBlocked(p, true)
	}

	for i, fe := range f.Entities {
		e, err := fe.build()
		if err != nil {
			return nil, fmt.Errorf("fixture %q entity %d: %w", f.Name, i, err)
		}
		if !w.Terrain.In(e.Pos) {
			return nil, fmt.Errorf("fixture %q entity %d: position %s is off the map", f.Name, i, e.Pos)
		}
		w.Add(e)
	}
	return w, nil
}

func (fe FixtureEntity) build() (*Entity, error) {
	e := &Entity{Kind: fe.Kind, Name: fe.Name, Pos: fe.Pos}

	slots := fe.Slots
	switch fe.Kind {
	case KindTree, KindRock:
		if !protocol.KnownItem(fe.Resource) {
			return nil, fmt.Errorf("unknown resource %q", fe.Resource)
		}
		e.Gatherable = &Gatherable{Resource: fe.Resource, Remaining: fe.Remaining, Initial: fe.Remaining}

	case KindChest, KindStockpile:
		if slots == 0 {
			slots = ChestSlots
		}
		if fe.Filter != "" && !protocol.KnownItem(fe.Filter) {
			return nil, fmt.Errorf("unknown filter %q", fe.Filter)
		}
		inv := NewInventory(slots)
		// Contents are applied in a stable order so that a fixture that
		// overfills a container fails the same way every time.
		for _, it := range protocol.AllItems() {
			if n := fe.Contents[it]; n > 0 && !inv.Add(it, n) {
				return nil, fmt.Errorf("contents do not fit: %d %s in %d slots", n, it, slots)
			}
		}
		e.Container = &Container{Color: fe.Color, Filter: fe.Filter, Inv: inv}

	case KindPlayer:
		if slots == 0 {
			slots = PlayerSlots
		}
		inv := NewInventory(slots)
		for _, it := range protocol.AllItems() {
			if n := fe.Contents[it]; n > 0 && !inv.Add(it, n) {
				return nil, fmt.Errorf("inventory does not fit: %d %s in %d slots", n, it, slots)
			}
		}
		e.Carrier = &Carrier{Inv: inv}

	case KindGroundItem:
		if !protocol.KnownItem(fe.Item) {
			return nil, fmt.Errorf("unknown item %q", fe.Item)
		}
		if fe.Count <= 0 {
			return nil, fmt.Errorf("ground item needs a positive count, got %d", fe.Count)
		}
		e.Stack = &Stack{Item: fe.Item, N: fe.Count}

	default:
		return nil, fmt.Errorf("unknown entity kind %q", fe.Kind)
	}
	return e, nil
}
