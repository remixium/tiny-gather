// Package rules validates actions and computes their outcomes.
//
// Outcomes come in two kinds that must never be collapsed into one:
//
//   - Rejected: the action was invalid and nothing happened. The actor should
//     re-plan.
//   - Resolved: the action was valid and was attempted, but may not have
//     achieved anything. The actor may simply try again.
//
// A build attempt that fails its probability roll is resolved and unsuccessful:
// it consumed material and produced nothing. That is a different fact from
// being told the tile was out of reach, and agents act on the difference.
package rules
