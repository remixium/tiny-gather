package rng

import "testing"

func TestSameSeedSameSequence(t *testing.T) {
	a, b := New(42), New(42)
	for i := 0; i < 1000; i++ {
		if x, y := a.IntN(1000), b.IntN(1000); x != y {
			t.Fatalf("draw %d diverged: %d != %d", i, x, y)
		}
	}
}

func TestDifferentSeedsDiverge(t *testing.T) {
	a, b := New(1), New(2)
	same := 0
	for i := 0; i < 100; i++ {
		if a.IntN(1000) == b.IntN(1000) {
			same++
		}
	}
	if same > 10 {
		t.Fatalf("seeds 1 and 2 produced %d/100 identical draws; they are not independent", same)
	}
}

// TestSequenceIsPinned guards the generator itself.
//
// If PCG is swapped for something else, or the seed derivation changes, every
// recorded scenario and every replay silently becomes invalid while still
// looking fine. Pinning the sequence makes that a deliberate change with a
// failing test attached rather than a silent one.
func TestSequenceIsPinned(t *testing.T) {
	want := []int{40, 28, 4, 98, 89, 57, 45, 28}

	r := New(12345)
	for i, w := range want {
		if got := r.IntN(100); got != w {
			t.Fatalf("draw %d is %d, want %d: the generator has changed, "+
				"which invalidates every recorded seed", i, got, w)
		}
	}
}

func TestChanceBounds(t *testing.T) {
	r := New(7)
	if r.Chance(0, 10) {
		t.Fatal("Chance(0, 10) should never occur")
	}
	if !r.Chance(10, 10) {
		t.Fatal("Chance(10, 10) should always occur")
	}
	if r.Chance(-1, 10) {
		t.Fatal("negative numerator should never occur")
	}
}

// TestChanceDistribution checks the ratio is honoured closely enough that a
// tuned probability means what it says. It is a fixed seed, so it cannot flake.
func TestChanceDistribution(t *testing.T) {
	r := New(99)
	hits := 0
	const trials = 100000
	for i := 0; i < trials; i++ {
		if r.Chance(1, 10) {
			hits++
		}
	}
	if hits < 9500 || hits > 10500 {
		t.Fatalf("Chance(1, 10) hit %d/%d, want about 10000", hits, trials)
	}
}
