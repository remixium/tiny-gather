package protocol

// Reason explains why an action was rejected.
//
// A rejection means the action was invalid and nothing happened at all. It is
// categorically different from an action that was accepted, attempted, and
// simply did not succeed — see Outcome on the event type.
type Reason string

const (
	NotAdjacent Reason = "not_adjacent"
	// OutOfRange is reserved for ranged actions, which do not exist yet.
	OutOfRange        Reason = "out_of_range"
	InventoryFull     Reason = "inventory_full"
	ContainerFull     Reason = "container_full"
	WrongResource     Reason = "wrong_resource"
	NodeEmpty         Reason = "node_empty"
	NoSuchEntity      Reason = "no_such_entity"
	InsufficientItems Reason = "insufficient_items"
	BlockedTile       Reason = "blocked_tile"
	// NotEquipped is reserved for tool requirements, which arrive with v2.
	NotEquipped     Reason = "not_equipped"
	RateLimited     Reason = "rate_limited"
	MalformedAction Reason = "malformed_action"
)
