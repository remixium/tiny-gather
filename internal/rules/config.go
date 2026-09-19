package rules

// Timing constants, all expressed in ticks at the simulation's fixed rate.
//
// Every duration is a whole number of ticks rather than a rate in seconds.
// Fractional accumulators drift by different amounts depending on when they are
// sampled, which would make a replay diverge from the run it replays.
const (
	// TickHz is the simulation rate. Durations below are in ticks.
	TickHz = 20

	// MoveCooldown is the ticks between steps, giving about 6.7 tiles a second.
	MoveCooldown = 3

	// GatherTicks is how long one unit of a resource takes to extract.
	GatherTicks = 20

	// SayCooldown throttles chat.
	SayCooldown = 20

	// MaxSayLength bounds a single chat message.
	MaxSayLength = 240

	// RespawnDelay is how long after a node is exhausted before another appears
	// elsewhere.
	RespawnDelay = 600
)
