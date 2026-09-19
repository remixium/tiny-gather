// Package rng provides the deterministic random source the simulation draws
// from.
//
// The type lives in its own package so that world generation and the tick loop
// can both use it without an import cycle. Ownership is unchanged: exactly one
// instance exists per run, package sim holds it, and no draw happens outside
// tick resolution.
package rng

import "math/rand/v2"

// Rand is a deterministic pseudo-random source.
//
// It wraps PCG explicitly rather than using the global generator, so that the
// algorithm is pinned: a given seed produces the same sequence on every machine
// and every Go release. That is what makes a run reproducible from its seed and
// an ordered input log.
type Rand struct {
	src *rand.Rand
}

// New returns a source seeded deterministically from seed.
func New(seed uint64) *Rand {
	// The second PCG parameter is derived from the first so that callers have a
	// single seed to record, reproduce and put in a fixture.
	return &Rand{src: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// IntN returns a value in [0, n). It panics if n <= 0.
func (r *Rand) IntN(n int) int { return r.src.IntN(n) }

// Chance reports whether an event of probability num/den occurred.
//
// Probabilities are integer ratios rather than floats so that replay never
// depends on floating point rounding. A 10% chance is Chance(1, 10).
func (r *Rand) Chance(num, den int) bool {
	switch {
	case num <= 0:
		return false
	case num >= den:
		return true
	default:
		return r.src.IntN(den) < num
	}
}

// Perm returns a deterministic permutation of [0, n).
func (r *Rand) Perm(n int) []int { return r.src.Perm(n) }
