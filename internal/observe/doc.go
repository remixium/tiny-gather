// Package observe builds the per-player observation from world state.
//
// The server does all geometry and pathfinding here so that agents never
// compute spatial relationships themselves. Observations are radius-limited:
// they report what a player can currently perceive and nothing more. This
// package maintains no remembered world on anyone's behalf — memory is the
// agent's responsibility, and the gap between what was seen and what is true is
// deliberate.
//
// Declarative facts are public: costs, capacities, filters, tool requirements.
// Empirical facts are never included: success probabilities, drop rates, and
// damage numbers have to be inferred from observed outcomes.
package observe
