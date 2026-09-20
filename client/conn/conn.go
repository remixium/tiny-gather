// Package conn is a client for the game protocol.
//
// It is deliberately renderer-agnostic and carries no game logic: the terminal
// client, the Ebiten client and the scenario runner all drive the same type, so
// that none of them can drift into speaking a slightly different protocol.
package conn

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// FrameBuffer is how many ticks a caller may fall behind before frames start
// being dropped locally.
const FrameBuffer = 256

// Frame is one tick as a client sees it: what is currently perceivable, and
// what was perceived happening.
type Frame struct {
	Tick        uint64
	Observation protocol.Observation
	Events      []protocol.Event
}

// Client is a connection to a game server.
type Client struct {
	ws       *websocket.Conn
	playerID string
	token    string

	frames chan Frame
	done   chan struct{}
	// cancel ends the read loop. The loop runs for the life of the client, not
	// for the life of the context that dialled it, so that a caller may impose
	// a timeout on connecting without also capping the session.
	cancel context.CancelFunc

	// writeMu serialises writes: the underlying connection permits only one
	// writer at a time, and callers may act from any goroutine.
	writeMu sync.Mutex

	closeOnce sync.Once
	mu        sync.Mutex
	err       error
}

// Options configures a dial.
type Options struct {
	// Name identifies a new player. Ignored when Token is set.
	Name string
	// Token reattaches to a player from an earlier session.
	Token string
}

// Dial connects to a server and joins, returning once the server has confirmed
// the join.
func Dial(ctx context.Context, url string, opts Options) (*Client, error) {
	if opts.Name == "" && opts.Token == "" {
		return nil, errors.New("conn: joining needs a name or a token")
	}

	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("conn: dialling %s: %w", url, err)
	}
	// Observations of a busy world are well past the default limit.
	ws.SetReadLimit(1 << 20)

	c := &Client{
		ws:     ws,
		frames: make(chan Frame, FrameBuffer),
		done:   make(chan struct{}),
	}

	join := protocol.ClientMessage{Type: protocol.MsgJoin, Name: opts.Name, Token: opts.Token}
	if err := wsjson.Write(ctx, ws, join); err != nil {
		ws.Close(websocket.StatusInternalError, "join failed")
		return nil, fmt.Errorf("conn: sending join: %w", err)
	}

	// The welcome is read synchronously so that Dial returns a client that is
	// already identified, rather than one whose id arrives at some later point.
	var welcome protocol.ServerMessage
	if err := wsjson.Read(ctx, ws, &welcome); err != nil {
		ws.Close(websocket.StatusInternalError, "no welcome")
		return nil, fmt.Errorf("conn: awaiting welcome: %w", err)
	}
	switch welcome.Type {
	case protocol.MsgWelcome:
	case protocol.MsgError:
		ws.Close(websocket.StatusNormalClosure, "join refused")
		return nil, fmt.Errorf("conn: join refused: %s", welcome.Error)
	default:
		ws.Close(websocket.StatusProtocolError, "unexpected frame")
		return nil, fmt.Errorf("conn: expected a welcome, got %q", welcome.Type)
	}

	c.playerID, c.token = welcome.PlayerID, welcome.Token

	lifetime, cancel := context.WithCancel(context.WithoutCancel(ctx))
	c.cancel = cancel
	go c.readLoop(lifetime)
	return c, nil
}

// PlayerID is the entity reference the server assigned, such as "player_3".
func (c *Client) PlayerID() string { return c.playerID }

// Token is the credential for reattaching to this player later. It is the
// client's to keep; it is never shown to another player.
func (c *Client) Token() string { return c.token }

// Frames yields one value per tick. It is closed when the connection ends.
func (c *Client) Frames() <-chan Frame { return c.frames }

// Err reports why the connection ended, or nil if it ended cleanly.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Do queues an action.
//
// It returns once the action has been sent, not once it has been resolved: the
// outcome arrives later as an event, because whether an action succeeds is a
// question about the world rather than about the connection.
func (c *Client) Do(ctx context.Context, a protocol.Action) error {
	return c.write(ctx, protocol.ClientMessage{Type: protocol.MsgAct, Action: &a})
}

// SetThinking marks the player as deliberating, so that others can tell a slow
// agent from a stopped one.
func (c *Client) SetThinking(ctx context.Context, thinking bool) error {
	return c.write(ctx, protocol.ClientMessage{Type: protocol.MsgThinking, Thinking: thinking})
}

func (c *Client) write(ctx context.Context, msg protocol.ClientMessage) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := wsjson.Write(ctx, c.ws, msg); err != nil {
		return fmt.Errorf("conn: sending %s: %w", msg.Type, err)
	}
	return nil
}

// Close ends the connection.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.done)
		err = c.ws.Close(websocket.StatusNormalClosure, "client closing")
		c.cancel()
	})
	return err
}

func (c *Client) readLoop(ctx context.Context) {
	defer close(c.frames)

	for {
		var msg protocol.ServerMessage
		if err := wsjson.Read(ctx, c.ws, &msg); err != nil {
			select {
			case <-c.done:
			default:
				c.setErr(err)
			}
			return
		}

		switch msg.Type {
		case protocol.MsgTick:
			if msg.Observation == nil {
				continue
			}
			frame := Frame{Tick: msg.Tick, Observation: *msg.Observation, Events: msg.Events}
			select {
			case c.frames <- frame:
			case <-c.done:
				return
			case <-ctx.Done():
				return
			default:
				// The caller is not keeping up. Dropping here is a local
				// choice and is visible to the caller as a gap in tick
				// numbers, unlike a silent loss on the wire.
			}

		case protocol.MsgError:
			c.setErr(fmt.Errorf("conn: server error: %s", msg.Error))

		case protocol.MsgWelcome:
			// A second welcome means a reconnect was negotiated elsewhere.
			c.playerID, c.token = msg.PlayerID, msg.Token
		}
	}
}

func (c *Client) setErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		c.err = err
	}
}
