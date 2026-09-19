package protocol

// Observation is what one player can currently perceive.
//
// It is built entirely by the server. All geometry and pathfinding is done here
// so that agents never compute spatial relationships themselves.
//
// Two rules govern its contents. It is radius-limited: entities appear only
// while in range, and nothing remembers them once they leave, because memory is
// the agent's responsibility. And it carries declarative facts only —
// capacities, costs, filters, what remains in a node — never empirical ones such
// as success probabilities or drop rates, which have to be inferred from
// observed outcomes.
type Observation struct {
	Tick   uint64       `json:"tick"`
	Self   SelfView     `json:"self"`
	Object []ObjectView `json:"objects,omitempty"`
	Player []PlayerView `json:"players,omitempty"`
	// Landmark positions are always included regardless of range: a house does
	// not stop existing because you walked away from it.
	Landmark []LandmarkView `json:"landmarks,omitempty"`
}

// SelfView is the viewing player's own state, which is never range-limited.
type SelfView struct {
	ID        string       `json:"id"`
	Name      string       `json:"name,omitempty"`
	Pos       Pos          `json:"pos"`
	Slots     int          `json:"slots"`
	UsedSlots int          `json:"used_slots"`
	Inventory map[Item]int `json:"inventory,omitempty"`
	Doing     string       `json:"doing,omitempty"`
}

// ObjectView is a non-player entity in range.
type ObjectView struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	// Distance is straight-line; PathCost is the number of steps actually
	// needed, accounting for obstacles, and is -1 when unreachable. They diverge
	// around the pond and the fence, and which one is the right measure of
	// "nearest" is a judgement the agent makes.
	Distance float64 `json:"distance"`
	PathCost int     `json:"path_cost"`
	Near     string  `json:"near,omitempty"`

	// Gatherable nodes.
	Remaining int  `json:"remaining,omitempty"`
	Resource  Item `json:"resource,omitempty"`

	// Containers.
	Color     string       `json:"color,omitempty"`
	Filter    Item         `json:"filter,omitempty"`
	Contents  map[Item]int `json:"contents,omitempty"`
	FreeSlots int          `json:"free_slots,omitempty"`

	// Ground items.
	Item  Item `json:"item,omitempty"`
	Count int  `json:"count,omitempty"`
}

// PlayerView is another player in range.
//
// It reports what is outwardly visible only. Carried items are private; once
// equipment exists, equipped items will appear here, which will make showing
// someone what you are holding a real action rather than a UI affordance.
type PlayerView struct {
	ID       string  `json:"id"`
	Name     string  `json:"name,omitempty"`
	Pos      Pos     `json:"pos"`
	Distance float64 `json:"distance"`
	PathCost int     `json:"path_cost"`
	Doing    string  `json:"doing,omitempty"`
	Thinking bool    `json:"thinking,omitempty"`
}

// LandmarkView is a named region used to make phrases like "the chest by the
// house" refer to something.
type LandmarkView struct {
	Name string `json:"name"`
	Pos  Pos    `json:"pos"`
}
