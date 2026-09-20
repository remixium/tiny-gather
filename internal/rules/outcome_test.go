package rules

import (
	"testing"

	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// TestPartialDepositIsASuccess checks that moving less than asked for resolves
// rather than being rejected. The caller compares N against what it requested
// and decides whether to explain the shortfall; the rules do not decide that on
// its behalf.
func TestPartialDepositIsASuccess(t *testing.T) {
	s := newScene(t)
	// One slot holding 47 of a 50-stack: room for exactly 3 more.
	s.chest.Container.Inv = world.NewInventory(1)
	s.chest.Container.Inv.Add(protocol.Wood, 47)
	s.player.Carrier.Inv.Add(protocol.Wood, 8)

	ev := resolution(t, s.apply(protocol.Action{
		Kind: protocol.ActDeposit, Target: s.chest.Ref(),
		Item: protocol.Wood, N: 8,
	}))

	if !ev.Succeeded() {
		t.Fatal("a partial deposit should succeed, not fail")
	}
	if ev.N != 3 {
		t.Fatalf("deposited %d wood, want 3 (all that fit)", ev.N)
	}
	if got := s.player.Carrier.Inv.Count(protocol.Wood); got != 5 {
		t.Fatalf("player kept %d wood, want 5", got)
	}
	if got := s.chest.Container.Inv.Count(protocol.Wood); got != 50 {
		t.Fatalf("chest holds %d wood, want 50", got)
	}
}

func TestPartialPickup(t *testing.T) {
	s := newScene(t)
	// One slot holding 47 ore: room for 3 of the pile's 20.
	s.player.Carrier.Inv = world.NewInventory(1)
	s.player.Carrier.Inv.Add(protocol.Ore, 47)

	ev := resolution(t, s.apply(protocol.Action{
		Kind: protocol.ActPickup, Target: s.pile.Ref(),
	}))

	if ev.N != 3 {
		t.Fatalf("picked up %d ore, want 3", ev.N)
	}
	if got := s.pile.Stack.N; got != 17 {
		t.Fatalf("pile has %d ore left, want 17", got)
	}
}

// TestMovingCancelsGathering is what makes fetching something a commitment. The
// cancellation resolves unsuccessfully rather than vanishing, so the actor
// learns that the gather stopped and why it produced nothing.
func TestMovingCancelsGathering(t *testing.T) {
	s := newScene(t)

	resolution(t, s.apply(protocol.Action{
		Kind: protocol.ActGather, Target: s.tree.Ref(),
	}))
	if s.player.Activity.Idle() {
		t.Fatal("gather did not start")
	}

	// Clear the chest to the west so there is somewhere to step.
	s.w.Remove(s.chest.ID)
	s.player.MoveCooldown = 0
	events := s.apply(protocol.Action{Kind: protocol.ActMove, Dir: protocol.DirW})

	if !s.player.Activity.Idle() {
		t.Fatal("moving did not cancel the gather")
	}
	var cancelled *protocol.Event
	for i := range events {
		if events[i].Kind == protocol.EvResolved && events[i].Action == protocol.ActGather {
			cancelled = &events[i]
		}
	}
	if cancelled == nil {
		t.Fatalf("no resolution for the cancelled gather in %v", events)
	}
	if cancelled.Succeeded() {
		t.Fatal("a cancelled gather resolved successfully; it produced nothing")
	}
}

// TestReissuingGatherKeepsProgress: an agent re-confirming its intent should
// not reset the clock it is already running.
func TestReissuingGatherKeepsProgress(t *testing.T) {
	s := newScene(t)
	gather := protocol.Action{Kind: protocol.ActGather, Target: s.tree.Ref()}

	resolution(t, s.apply(gather))
	s.player.Activity.Progress = 15
	resolution(t, s.apply(gather))

	if got := s.player.Activity.Progress; got != 15 {
		t.Fatalf("progress reset to %d, want 15 kept", got)
	}
}

func TestSwitchingNodeResetsProgress(t *testing.T) {
	s := newScene(t)
	resolution(t, s.apply(protocol.Action{Kind: protocol.ActGather, Target: s.tree.Ref()}))
	s.player.Activity.Progress = 15

	s.emptyTree.Gatherable.Remaining = 3
	resolution(t, s.apply(protocol.Action{Kind: protocol.ActGather, Target: s.emptyTree.Ref()}))

	if got := s.player.Activity.Progress; got != 0 {
		t.Fatalf("progress is %d after switching nodes, want 0", got)
	}
}

// TestRejectedMoveDoesNotCancelGathering: a rejection means nothing happened at
// all. Walking into a wall must not quietly cost an agent the gather it had
// running, or the two outcome kinds would not actually be distinct in practice.
func TestRejectedMoveDoesNotCancelGathering(t *testing.T) {
	s := newScene(t)
	resolution(t, s.apply(protocol.Action{Kind: protocol.ActGather, Target: s.tree.Ref()}))
	s.player.Activity.Progress = 12
	s.player.MoveCooldown = 0

	// West is the chest: a blocked tile, so the move is rejected.
	if got := rejection(t, s.apply(protocol.Action{
		Kind: protocol.ActMove, Dir: protocol.DirW,
	})); got != protocol.BlockedTile {
		t.Fatalf("rejected with %q, want %q", got, protocol.BlockedTile)
	}
	if s.player.Activity.Idle() {
		t.Fatal("a rejected move cancelled the gather; nothing should have happened")
	}
	if got := s.player.Activity.Progress; got != 12 {
		t.Fatalf("progress is %d after a rejected move, want 12 untouched", got)
	}
}

// TestDropMergesWithExistingPile keeps the ground from filling with piles that
// an agent would then have to disambiguate between.
func TestDropMergesWithExistingPile(t *testing.T) {
	s := newScene(t)
	s.player.Carrier.Inv.Add(protocol.Ore, 4)

	ev := resolution(t, s.apply(protocol.Action{
		Kind: protocol.ActDrop, Item: protocol.Ore, N: 4,
	}))

	if ev.Target != s.pile.Ref() {
		t.Fatalf("dropped onto %s, want a merge into the existing %s", ev.Target, s.pile.Ref())
	}
	if got := s.pile.Stack.N; got != 24 {
		t.Fatalf("pile holds %d ore, want 24", got)
	}
}

// TestAnyoneMayTakeFromAnyContainer is the absence of a rule, and is tested
// because its absence is deliberate. Containers are public caches; nothing
// mechanical protects what is left in one, which is what makes an agreement
// between players rest on trust.
func TestAnyoneMayTakeFromAnyContainer(t *testing.T) {
	s := newScene(t)
	s.chest.Container.Inv.Add(protocol.Wood, 5)

	ev := resolution(t, s.apply(protocol.Action{
		Kind: protocol.ActWithdraw, Target: s.chest.Ref(),
		Item: protocol.Wood, N: 5,
	}))

	if !ev.Succeeded() || ev.N != 5 {
		t.Fatalf("withdrawal moved %d wood (success %v), want 5 and success",
			ev.N, ev.Succeeded())
	}
}

// TestTransfersAreLogged: events are the substrate an agent learns from, so
// every movement of goods has to leave a record with enough in it to reconstruct
// who moved what to whom.
func TestTransfersAreLogged(t *testing.T) {
	s := newScene(t)
	s.player.Carrier.Inv.Add(protocol.Wood, 5)

	events := s.apply(protocol.Action{
		Kind: protocol.ActDeposit, Target: s.chest.Ref(),
		Item: protocol.Wood, N: 5,
	})

	var detail *protocol.Event
	for i := range events {
		if events[i].Kind == protocol.EvDeposited {
			detail = &events[i]
		}
	}
	if detail == nil {
		t.Fatalf("no deposited event in %v", events)
	}
	switch {
	case detail.Actor != s.player.Ref():
		t.Errorf("actor is %q, want %q", detail.Actor, s.player.Ref())
	case detail.Target != s.chest.Ref():
		t.Errorf("target is %q, want %q", detail.Target, s.chest.Ref())
	case detail.Item != protocol.Wood || detail.N != 5:
		t.Errorf("logged %d %s, want 5 wood", detail.N, detail.Item)
	case detail.Tick == 0:
		t.Error("event has no tick stamp")
	}
}
