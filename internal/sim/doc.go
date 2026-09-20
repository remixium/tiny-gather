// Package sim owns the tick loop and every source of nondeterminism in the
// game.
//
// A run must be reproducible from (seed, ordered input log). Preserving that
// requires all of the following:
//
//   - All randomness comes from one PRNG stream, seeded from the world seed and
//     owned by this package. Never the global math/rand, and never drawn from a
//     goroutine.
//   - Draws happen only during tick resolution, in a fixed entity order.
//   - Inputs are queued on arrival but applied at tick boundaries in ascending
//     entity-id order, never in arrival order.
//   - Timing uses integer tick cooldowns. Fractional accumulators drift and
//     break replay.
//
// Reproducibility exists so that a situation can be replayed and watched again.
// An agent built on a language model and a probabilistic decision layer already
// carries two sources of randomness that cannot be removed; the world is the
// one that can, which is what makes the other two possible to study.
package sim
