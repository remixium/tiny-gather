// Package world holds the grid, the entities on it, and seeded world
// generation.
//
// Entities are composed from traits rather than subclassed by kind: a tree is
// positioned and gatherable, a goblin is positioned, mobile, damageable and
// hostile. Adding content should mean composing existing traits, not adding a
// type switch.
//
// World generation draws from the seeded stream owned by package sim, so that a
// world is reproducible from its seed alone.
package world
