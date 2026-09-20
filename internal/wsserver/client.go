package wsserver

import (
	"context"
	"log/slog"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// client is one connection.
//
// player and token are written only by the Run goroutine, which is also the
// only goroutine that reads them. The read loop hands frames over rather than
// acting on them, so a connection never touches the world itself.
type client struct {
	ws  *websocket.Conn
	out chan protocol.ServerMessage
	log *slog.Logger

	player world.ID
	token  string

	// overflowed records that the send queue filled, so that the connection is
	// closed once rather than on every subsequent frame.
	overflowed bool
}

// send queues a frame, disconnecting a client that cannot keep up.
//
// Dropping the frame instead would leave an agent with a hole in the event
// record it reasons from and no indication that anything was missing, which is
// worse than a clean disconnection it can detect and recover from.
func (c *client) send(msg protocol.ServerMessage) {
	if c.overflowed {
		return
	}
	select {
	case c.out <- msg:
	default:
		c.overflowed = true
		c.log.Warn("client fell behind; disconnecting", "player", c.player)
		c.close(websocket.StatusTryAgainLater, "client too slow")
	}
}

func (c *client) fail(reason string) {
	c.send(protocol.ServerMessage{Type: protocol.MsgError, Error: reason})
}

func (c *client) close(code websocket.StatusCode, reason string) {
	_ = c.ws.Close(code, reason)
}

// readLoop decodes client frames and hands them to the server goroutine.
func (c *client) readLoop(ctx context.Context, s *Server) {
	for {
		var msg protocol.ClientMessage
		if err := wsjson.Read(ctx, c.ws, &msg); err != nil {
			if ctx.Err() == nil {
				c.log.Debug("connection closed", "error", err)
			}
			return
		}
		select {
		case s.commands <- command{from: c, msg: msg}:
		case <-ctx.Done():
			return
		}
	}
}

// writeLoop sends queued frames.
func (c *client) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.out:
			if err := wsjson.Write(ctx, c.ws, msg); err != nil {
				if ctx.Err() == nil {
					c.log.Debug("write failed", "error", err)
				}
				c.close(websocket.StatusInternalError, "write failed")
				return
			}
		}
	}
}
