package protocol

// ClientMessageType names something a client sends to the server.
type ClientMessageType string

const (
	// MsgJoin enters the world. A client that has played before sends the token
	// it was given, and is reattached to the same player.
	MsgJoin ClientMessageType = "join"
	// MsgAct queues an action.
	MsgAct ClientMessageType = "act"
	// MsgThinking marks a player as deliberating, so that other players can see
	// the difference between an agent that is thinking and one that has stopped
	// working.
	MsgThinking ClientMessageType = "thinking"
)

// ClientMessage is a single frame from a client. Fields not relevant to the
// type are left zero.
type ClientMessage struct {
	Type ClientMessageType `json:"type"`

	Name  string `json:"name,omitempty"`
	Token string `json:"token,omitempty"`

	Action   *Action `json:"action,omitempty"`
	Thinking bool    `json:"thinking,omitempty"`
}

// ServerMessageType names something the server sends to a client.
type ServerMessageType string

const (
	// MsgWelcome answers a join with the player's identity and reconnect token.
	MsgWelcome ServerMessageType = "welcome"
	// MsgTick carries one tick: what the player can now perceive, and what they
	// perceived happening during it.
	MsgTick ServerMessageType = "tick"
	// MsgError reports a frame the server could not act on at all. It is not
	// how a rejected action is reported — that is an event, because a rejection
	// is part of the game rather than a protocol failure.
	MsgError ServerMessageType = "error"
)

// ServerMessage is a single frame to a client.
type ServerMessage struct {
	Type ServerMessageType `json:"type"`
	Tick uint64            `json:"tick"`

	// PlayerID and Token appear on a welcome. The token is the client's proof
	// of identity on a later reconnect, and is never shown to anyone else.
	PlayerID string `json:"player_id,omitempty"`
	Token    string `json:"token,omitempty"`

	Observation *Observation `json:"observation,omitempty"`
	Events      []Event      `json:"events,omitempty"`

	Error string `json:"error,omitempty"`
}
