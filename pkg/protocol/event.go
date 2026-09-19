package protocol

// EventKind names something that happened in the world.
type EventKind string

const (
	// EvRejected reports an invalid action. Nothing happened; the actor should
	// re-plan.
	EvRejected EventKind = "action_rejected"
	// EvResolved reports a valid action that was attempted. It carries Success,
	// because an attempt may legitimately achieve nothing; the actor may simply
	// try again.
	EvResolved EventKind = "action_resolved"

	EvGathered      EventKind = "gathered"
	EvDropped       EventKind = "dropped"
	EvPickedUp      EventKind = "picked_up"
	EvDeposited     EventKind = "deposited"
	EvWithdrew      EventKind = "withdrew"
	EvSaid          EventKind = "said"
	EvNodeDepleted  EventKind = "node_depleted"
	EvNodeRespawned EventKind = "node_respawned"
)

// Event is one thing that happened, stamped with the tick it happened on.
//
// Events are the substrate an agent learns from, so each carries enough to
// compute a rate: when, who, what was attempted, what it cost and what it
// yielded. Every transfer of goods produces one, which lets an evaluation
// harness reconstruct who moved what to whom — though never what was promised,
// because the game does not know.
type Event struct {
	Tick uint64    `json:"tick"`
	Kind EventKind `json:"kind"`

	Actor  string     `json:"actor,omitempty"`
	Action ActionKind `json:"action,omitempty"`
	Target string     `json:"target,omitempty"`

	// Reason is set on rejections only.
	Reason Reason `json:"reason,omitempty"`
	// Success is set on resolutions only. It is a pointer so that the
	// unsuccessful case survives JSON encoding rather than being omitted.
	Success *bool `json:"success,omitempty"`

	Item Item   `json:"item,omitempty"`
	N    int    `json:"n,omitempty"`
	Text string `json:"text,omitempty"`
	Pos  *Pos   `json:"pos,omitempty"`

	Consumed map[Item]int `json:"consumed,omitempty"`
	Produced map[Item]int `json:"produced,omitempty"`
}

// Rejected builds a rejection event.
func Rejected(tick uint64, actor string, a ActionKind, why Reason) Event {
	return Event{Tick: tick, Kind: EvRejected, Actor: actor, Action: a, Reason: why}
}

// Resolved builds a resolution event.
func Resolved(tick uint64, actor string, a ActionKind, success bool) Event {
	return Event{Tick: tick, Kind: EvResolved, Actor: actor, Action: a, Success: &success}
}

// Succeeded reports whether e is a resolution that achieved something.
func (e Event) Succeeded() bool { return e.Success != nil && *e.Success }
