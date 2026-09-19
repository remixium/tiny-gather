package rules

import (
	"testing"

	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// scene builds a compact world with everything the rejection table needs.
//
// The player stands at the centre with all four neighbouring tiles occupied by
// something they can act on: a stocked tree, an exhausted tree, a plain chest
// and an ore-only chest, plus a pile underfoot.
type scene struct {
	w         *world.World
	player    *world.Entity
	tree      *world.Entity
	emptyTree *world.Entity
	farTree   *world.Entity
	chest     *world.Entity
	oreChest  *world.Entity
	pile      *world.Entity
}

func newScene(t *testing.T) *scene {
	t.Helper()
	w := world.New(30, 30)
	s := &scene{w: w}

	s.player = world.AddPlayer(w, "tester", protocol.Pos{X: 5, Y: 5})
	s.tree = w.Add(&world.Entity{
		Kind: world.KindTree, Pos: protocol.Pos{X: 6, Y: 5},
		Gatherable: &world.Gatherable{Resource: protocol.Wood, Remaining: 5, Initial: 5},
	})
	s.emptyTree = w.Add(&world.Entity{
		Kind: world.KindTree, Pos: protocol.Pos{X: 5, Y: 4},
		Gatherable: &world.Gatherable{Resource: protocol.Wood, Remaining: 0, Initial: 5},
	})
	s.farTree = w.Add(&world.Entity{
		Kind: world.KindTree, Pos: protocol.Pos{X: 20, Y: 20},
		Gatherable: &world.Gatherable{Resource: protocol.Wood, Remaining: 5, Initial: 5},
	})
	s.chest = w.Add(&world.Entity{
		Kind: world.KindChest, Pos: protocol.Pos{X: 4, Y: 5},
		Container: &world.Container{Color: "blue", Inv: world.NewInventory(12)},
	})
	s.oreChest = w.Add(&world.Entity{
		Kind: world.KindChest, Pos: protocol.Pos{X: 5, Y: 6},
		Container: &world.Container{Color: "red", Filter: protocol.Ore, Inv: world.NewInventory(12)},
	})

	s.pile = w.Add(&world.Entity{
		Kind: world.KindGroundItem, Pos: protocol.Pos{X: 5, Y: 5},
		Stack: &world.Stack{Item: protocol.Ore, N: 20},
	})
	return s
}

func (s *scene) apply(a protocol.Action) []protocol.Event {
	return Apply(s.w, 1, s.player, a)
}

func rejection(t *testing.T, events []protocol.Event) protocol.Reason {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("no events returned")
	}
	if events[0].Kind != protocol.EvRejected {
		t.Fatalf("first event is %s, want a rejection", events[0].Kind)
	}
	return events[0].Reason
}

func resolution(t *testing.T, events []protocol.Event) protocol.Event {
	t.Helper()
	if len(events) == 0 {
		t.Fatal("no events returned")
	}
	if events[0].Kind != protocol.EvResolved {
		t.Fatalf("first event is %s (reason %q), want a resolution",
			events[0].Kind, events[0].Reason)
	}
	return events[0]
}

// TestRejectionReasons covers every reason the rules can currently produce.
// Each row is a situation an agent has to tell apart from the others, because
// each implies a different recovery.
func TestRejectionReasons(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*scene)
		act   func(*scene) protocol.Action
		want  protocol.Reason
	}{
		{
			name: "gather out of reach",
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActGather, Target: s.farTree.Ref()}
			},
			want: protocol.NotAdjacent,
		},
		{
			name: "gather an exhausted node",
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActGather, Target: s.emptyTree.Ref()}
			},
			want: protocol.NodeEmpty,
		},
		{
			name: "gather with nowhere to put it",
			setup: func(s *scene) {
				s.player.Carrier.Inv.Add(protocol.Ore, 10)
			},
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActGather, Target: s.tree.Ref()}
			},
			want: protocol.InventoryFull,
		},
		{
			name: "gather something that is not there",
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActGather, Target: "tree_9999"}
			},
			want: protocol.NoSuchEntity,
		},
		{
			name: "gather something that is not a node",
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActGather, Target: s.chest.Ref()}
			},
			want: protocol.NoSuchEntity,
		},
		{
			name: "walk into an obstacle",
			setup: func(s *scene) {
				s.w.Terrain.SetBlocked(protocol.Pos{X: 5, Y: 4}, true)
			},
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActMove, Dir: protocol.DirN}
			},
			want: protocol.BlockedTile,
		},
		{
			name: "walk before the cooldown expires",
			setup: func(s *scene) {
				s.player.MoveCooldown = 2
			},
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActMove, Dir: protocol.DirE}
			},
			want: protocol.RateLimited,
		},
		{
			name: "drop what you do not have",
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActDrop, Item: protocol.Wood, N: 5}
			},
			want: protocol.InsufficientItems,
		},
		{
			name: "withdraw what is not there",
			act: func(s *scene) protocol.Action {
				return protocol.Action{
					Kind: protocol.ActWithdraw, Target: s.chest.Ref(),
					Item: protocol.Wood, N: 1,
				}
			},
			want: protocol.InsufficientItems,
		},
		{
			name: "deposit into a filtered container",
			setup: func(s *scene) {
				s.player.Carrier.Inv.Add(protocol.Wood, 3)
			},
			act: func(s *scene) protocol.Action {
				return protocol.Action{
					Kind: protocol.ActDeposit, Target: s.oreChest.Ref(),
					Item: protocol.Wood, N: 1,
				}
			},
			want: protocol.WrongResource,
		},
		{
			name: "deposit into a full container",
			setup: func(s *scene) {
				// Twelve non-stacking wood fills all twelve slots.
				s.chest.Container.Inv.Add(protocol.Wood, 12)
				s.player.Carrier.Inv.Add(protocol.Wood, 3)
			},
			act: func(s *scene) protocol.Action {
				return protocol.Action{
					Kind: protocol.ActDeposit, Target: s.chest.Ref(),
					Item: protocol.Wood, N: 3,
				}
			},
			want: protocol.ContainerFull,
		},
		{
			name: "pick up with a full inventory",
			setup: func(s *scene) {
				s.player.Carrier.Inv.Add(protocol.Wood, 10)
			},
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActPickup, Target: s.pile.Ref()}
			},
			want: protocol.InventoryFull,
		},
		{
			name: "speak before the cooldown expires",
			setup: func(s *scene) {
				s.player.SayCooldown = 5
			},
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActSay, Text: "hello"}
			},
			want: protocol.RateLimited,
		},
		{
			name: "walk in no direction",
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActMove, Dir: "sideways"}
			},
			want: protocol.MalformedAction,
		},
		{
			name: "drop a negative amount",
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActDrop, Item: protocol.Wood, N: -1}
			},
			want: protocol.MalformedAction,
		},
		{
			name: "drop an item that does not exist",
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActDrop, Item: "diamonds", N: 1}
			},
			want: protocol.MalformedAction,
		},
		{
			name: "say nothing",
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: protocol.ActSay, Text: ""}
			},
			want: protocol.MalformedAction,
		},
		{
			name: "attempt an unknown action",
			act: func(s *scene) protocol.Action {
				return protocol.Action{Kind: "teleport"}
			},
			want: protocol.MalformedAction,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newScene(t)
			if tc.setup != nil {
				tc.setup(s)
			}
			if got := rejection(t, s.apply(tc.act(s))); got != tc.want {
				t.Fatalf("rejected with %q, want %q", got, tc.want)
			}
		})
	}
}
