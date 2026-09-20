package wsserver_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/vadremix/tiny-gather/client/conn"
	"github.com/vadremix/tiny-gather/internal/sim"
	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/internal/wsserver"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// harness starts a real server over a real HTTP listener, so that everything
// below exercises the protocol rather than a stand-in for it.
type harness struct {
	url string
	sim *sim.Sim
}

func start(t *testing.T, s *sim.Sim) *harness {
	t.Helper()

	srv := wsserver.New(s, wsserver.Options{TickRate: 5 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Run(ctx) }()

	http := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		http.Close()
		cancel()
	})

	return &harness{url: "ws" + strings.TrimPrefix(http.URL, "http"), sim: s}
}

func (h *harness) join(t *testing.T, name string) *conn.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := conn.Dial(ctx, h.url, conn.Options{Name: name})
	if err != nil {
		t.Fatalf("joining as %s: %v", name, err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// nextFrame waits for one tick, failing rather than hanging if none arrives.
func nextFrame(t *testing.T, c *conn.Client) conn.Frame {
	t.Helper()
	select {
	case f, ok := <-c.Frames():
		if !ok {
			t.Fatalf("connection closed: %v", c.Err())
		}
		return f
	case <-time.After(5 * time.Second):
		t.Fatal("no frame arrived within 5s")
		return conn.Frame{}
	}
}

// await consumes frames until want reports satisfied, so that a test does not
// depend on which tick a change lands on.
func await(t *testing.T, c *conn.Client, what string, want func(conn.Frame) bool) conn.Frame {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case f, ok := <-c.Frames():
			if !ok {
				t.Fatalf("connection closed while waiting for %s: %v", what, c.Err())
			}
			if want(f) {
				return f
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func TestJoinAssignsIdentity(t *testing.T) {
	h := start(t, sim.New(1))
	c := h.join(t, "ana")

	if c.PlayerID() == "" {
		t.Error("server assigned no player id")
	}
	if c.Token() == "" {
		t.Error("server issued no reconnect token")
	}

	f := nextFrame(t, c)
	if f.Observation.Self.ID != c.PlayerID() {
		t.Errorf("observation is for %q, want %q", f.Observation.Self.ID, c.PlayerID())
	}
	if f.Observation.Self.Name != "ana" {
		t.Errorf("player name is %q, want ana", f.Observation.Self.Name)
	}
}

// TestTokenReattachesToTheSamePlayer is the identity invariant. A returning
// client proves who it is with its token; a rejoin under any name is a new
// player, so a reputation cannot be shed by reconnecting.
func TestTokenReattachesToTheSamePlayer(t *testing.T) {
	h := start(t, sim.New(1))

	first := h.join(t, "ana")
	id, token := first.PlayerID(), first.Token()
	_ = first.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	again, err := conn.Dial(ctx, h.url, conn.Options{Token: token})
	if err != nil {
		t.Fatalf("reconnecting with a token: %v", err)
	}
	defer again.Close()

	if again.PlayerID() != id {
		t.Errorf("reconnected as %q, want the original %q", again.PlayerID(), id)
	}

	fresh := h.join(t, "ana")
	if fresh.PlayerID() == id {
		t.Error("rejoining under the same name reattached to the original player; " +
			"names are not identity")
	}
}

func TestUnknownTokenIsRefused(t *testing.T) {
	h := start(t, sim.New(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := conn.Dial(ctx, h.url, conn.Options{Token: "not-a-real-token"}); err == nil {
		t.Fatal("an unrecognised token was accepted")
	}
}

// TestTwoClientsSeeOneWorld is the point of the phase: two connections over the
// real protocol observing the same authoritative state.
func TestTwoClientsSeeOneWorld(t *testing.T) {
	h := start(t, sim.NewWithWorld(world.New(30, 30), 1))

	a := h.join(t, "a-client")
	b := h.join(t, "b-client")

	// Each should see the other: both spawn near the house, well inside the
	// observation radius.
	fa := await(t, a, "a to see b", func(f conn.Frame) bool {
		return len(f.Observation.Player) >= 1
	})
	fb := await(t, b, "b to see a", func(f conn.Frame) bool {
		return len(f.Observation.Player) >= 1
	})

	sawB := false
	for _, p := range fa.Observation.Player {
		if p.ID == b.PlayerID() {
			sawB = true
		}
	}
	if !sawB {
		t.Errorf("a does not see b (%s) among %d players", b.PlayerID(), len(fa.Observation.Player))
	}
	sawA := false
	for _, p := range fb.Observation.Player {
		if p.ID == a.PlayerID() {
			sawA = true
		}
	}
	if !sawA {
		t.Errorf("b does not see a (%s) among %d players", a.PlayerID(), len(fb.Observation.Player))
	}
}

// TestChatReachesTheOtherClient exercises the whole round trip: an action sent
// by one client becomes an event delivered to another.
func TestChatReachesTheOtherClient(t *testing.T) {
	h := start(t, sim.New(1))
	a := h.join(t, "speaker")
	b := h.join(t, "listener")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Do(ctx, protocol.Action{Kind: protocol.ActSay, Text: "over here"}); err != nil {
		t.Fatalf("sending chat: %v", err)
	}

	await(t, b, "the listener to hear it", func(f conn.Frame) bool {
		for _, ev := range f.Events {
			if ev.Kind == protocol.EvSaid && ev.Text == "over here" && ev.Actor == a.PlayerID() {
				return true
			}
		}
		return false
	})
}

// TestRejectionReachesTheActor: a rejected action is a game event, not a
// protocol error, and has to come back through the event stream where an agent
// is already looking.
func TestRejectionReachesTheActor(t *testing.T) {
	h := start(t, sim.New(1))
	c := h.join(t, "walker")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Do(ctx, protocol.Action{Kind: protocol.ActMove, Dir: "nowhere"}); err != nil {
		t.Fatalf("sending action: %v", err)
	}

	await(t, c, "the rejection", func(f conn.Frame) bool {
		for _, ev := range f.Events {
			if ev.Kind == protocol.EvRejected && ev.Reason == protocol.MalformedAction {
				return true
			}
		}
		return false
	})
	if err := c.Err(); err != nil {
		t.Errorf("a rejected action was reported as a connection error: %v", err)
	}
}

// TestActionChangesWorldState closes the loop: an action over the wire moves
// the authoritative entity, and the change comes back in a later observation.
func TestActionChangesWorldState(t *testing.T) {
	w := world.New(30, 30)
	h := start(t, sim.NewWithWorld(w, 1))
	c := h.join(t, "walker")

	start := nextFrame(t, c).Observation.Self.Pos

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Do(ctx, protocol.Action{Kind: protocol.ActMove, Dir: protocol.DirN}); err != nil {
		t.Fatalf("sending move: %v", err)
	}

	moved := await(t, c, "the player to move", func(f conn.Frame) bool {
		return f.Observation.Self.Pos != start
	})
	if moved.Observation.Self.Pos.Y != start.Y-1 {
		t.Errorf("moved to %s from %s, want one tile north",
			moved.Observation.Self.Pos, start)
	}
}

// TestActingBeforeJoiningIsRefused guards one of the few cases that really is a
// protocol error rather than a game outcome. It uses a raw socket, because a
// well-behaved client cannot produce it.
func TestActingBeforeJoiningIsRefused(t *testing.T) {
	h := start(t, sim.New(1))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, h.url, nil)
	if err != nil {
		t.Fatalf("dialling: %v", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "done")

	act := protocol.Action{Kind: protocol.ActSay, Text: "hello"}
	if err := wsjson.Write(ctx, ws, protocol.ClientMessage{
		Type: protocol.MsgAct, Action: &act,
	}); err != nil {
		t.Fatalf("writing act: %v", err)
	}

	var reply protocol.ServerMessage
	if err := wsjson.Read(ctx, ws, &reply); err != nil {
		t.Fatalf("reading reply: %v", err)
	}
	if reply.Type != protocol.MsgError {
		t.Fatalf("server replied with %q, want an error", reply.Type)
	}
}

// TestUnknownMessageTypeIsRefused: an unrecognised frame is answered rather
// than ignored, so a client speaking a newer protocol finds out.
func TestUnknownMessageTypeIsRefused(t *testing.T) {
	h := start(t, sim.New(1))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ws, _, err := websocket.Dial(ctx, h.url, nil)
	if err != nil {
		t.Fatalf("dialling: %v", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "done")

	if err := wsjson.Write(ctx, ws, protocol.ClientMessage{Type: "levitate"}); err != nil {
		t.Fatalf("writing frame: %v", err)
	}

	var reply protocol.ServerMessage
	if err := wsjson.Read(ctx, ws, &reply); err != nil {
		t.Fatalf("reading reply: %v", err)
	}
	if reply.Type != protocol.MsgError {
		t.Fatalf("server replied with %q, want an error", reply.Type)
	}
}
