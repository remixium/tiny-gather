// Package wsserver carries the simulation over WebSockets.
//
// The simulation is not safe for concurrent use, and making it so would invite
// exactly the nondeterminism the design rules out. Instead every piece of
// simulation state is owned by a single goroutine running Run. Connections read
// frames on their own goroutines and hand them over through channels; nothing
// else touches the world.
//
// A live server is not itself reproducible, because network timing decides
// which tick an action lands on. Reproducibility belongs to replay: a recorded
// seed and ordered input log, fed through the simulation directly. The server's
// job is to keep the ordering rules intact so that such a log means something —
// inputs are drained at tick boundaries, never applied mid-tick.
package wsserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/vadremix/tiny-gather/internal/observe"
	"github.com/vadremix/tiny-gather/internal/rules"
	"github.com/vadremix/tiny-gather/internal/sim"
	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// DefaultOutBuffer is how many frames may be queued for a client before it is
// disconnected.
//
// Dropping frames instead would be worse than disconnecting: events are the
// record an agent learns from, and silently losing some would leave it drawing
// conclusions from an incomplete history without knowing.
const DefaultOutBuffer = 64

// Options configures a server.
type Options struct {
	// TickRate is the interval between ticks. Zero means the simulation's own
	// rate.
	TickRate time.Duration
	// OutBuffer is the per-client send queue depth. Zero means DefaultOutBuffer.
	OutBuffer int
	// Logger receives connection and error notices. Zero means no logging.
	Logger *slog.Logger
}

func (o Options) withDefaults() Options {
	if o.TickRate <= 0 {
		o.TickRate = time.Second / rules.TickHz
	}
	if o.OutBuffer <= 0 {
		o.OutBuffer = DefaultOutBuffer
	}
	if o.Logger == nil {
		o.Logger = slog.New(slog.DiscardHandler)
	}
	return o
}

// Server runs a simulation and serves it to connected clients.
type Server struct {
	sim  *sim.Sim
	opts Options

	commands   chan command
	register   chan *client
	unregister chan *client

	// Owned by the Run goroutine; nothing else may read or write these.
	clients  map[*client]struct{}
	sessions map[string]world.ID
}

type command struct {
	from *client
	msg  protocol.ClientMessage
}

// New returns a server over an existing simulation.
func New(s *sim.Sim, opts Options) *Server {
	return &Server{
		sim:        s,
		opts:       opts.withDefaults(),
		commands:   make(chan command, 256),
		register:   make(chan *client),
		unregister: make(chan *client),
		clients:    map[*client]struct{}{},
		sessions:   map[string]world.ID{},
	}
}

// Run drives the tick loop until ctx is cancelled.
//
// Registration, commands and ticks are all handled here, on one goroutine, in
// whatever order they arrive. Commands queued between two ticks are applied
// before the tick that follows them, so an action never lands halfway through
// a tick's resolution.
func (s *Server) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.opts.TickRate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.closeAll()
			return ctx.Err()

		case c := <-s.register:
			s.clients[c] = struct{}{}

		case c := <-s.unregister:
			delete(s.clients, c)

		case cmd := <-s.commands:
			s.handle(cmd)

		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *Server) tick() {
	events := s.sim.Step()
	for c := range s.clients {
		if c.player == 0 {
			continue
		}
		obs := observe.Build(s.sim.World, s.sim.Tick(), c.player)
		c.send(protocol.ServerMessage{
			Type:        protocol.MsgTick,
			Tick:        s.sim.Tick(),
			Observation: &obs,
			Events:      observe.ForPlayer(s.sim.World, c.player, events),
		})
	}
}

func (s *Server) handle(cmd command) {
	switch cmd.msg.Type {
	case protocol.MsgJoin:
		s.handleJoin(cmd)

	case protocol.MsgAct:
		if cmd.from.player == 0 {
			cmd.from.fail("join before acting")
			return
		}
		if cmd.msg.Action == nil {
			cmd.from.fail("act frame carried no action")
			return
		}
		// A refused enqueue is not an error: the simulation reports it as a
		// rejection event, which is where an agent already looks.
		s.sim.Enqueue(cmd.from.player, *cmd.msg.Action)

	case protocol.MsgThinking:
		if e := s.sim.World.Get(cmd.from.player); e != nil {
			e.Thinking = cmd.msg.Thinking
		}

	default:
		cmd.from.fail(fmt.Sprintf("unknown message type %q", cmd.msg.Type))
	}
}

func (s *Server) handleJoin(cmd command) {
	c := cmd.from
	if c.player != 0 {
		c.fail("already joined")
		return
	}

	// A returning client proves who it is with the token it was issued, not
	// with a name. Names are not identity: anyone can claim one, and a
	// reputation that a rejoin under a different name escapes is worthless.
	if token := cmd.msg.Token; token != "" {
		if id, ok := s.sessions[token]; ok {
			if e := s.sim.World.Get(id); e != nil {
				c.player, c.token = id, token
				c.send(s.welcome(e, token))
				return
			}
		}
		c.fail("unrecognised token")
		return
	}

	name := cmd.msg.Name
	if name == "" {
		c.fail("join requires a name")
		return
	}
	token, err := newToken()
	if err != nil {
		c.fail("could not issue a session token")
		return
	}

	e := s.sim.Join(name)
	s.sessions[token] = e.ID
	c.player, c.token = e.ID, token
	c.send(s.welcome(e, token))
}

func (s *Server) welcome(e *world.Entity, token string) protocol.ServerMessage {
	obs := observe.Build(s.sim.World, s.sim.Tick(), e.ID)
	return protocol.ServerMessage{
		Type:        protocol.MsgWelcome,
		Tick:        s.sim.Tick(),
		PlayerID:    e.Ref(),
		Token:       token,
		Observation: &obs,
	}
}

func (s *Server) closeAll() {
	for c := range s.clients {
		c.close(websocket.StatusGoingAway, "server shutting down")
		delete(s.clients, c)
	}
}

// newToken draws from the cryptographic source rather than the simulation's.
//
// Session tokens are not part of world state, so this does not touch replay;
// drawing them from the seeded stream would make them predictable from a seed,
// which is a different and worse problem.
func newToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// Handler upgrades HTTP requests into game connections.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			// This is a game server meant to be run locally and connected to by
			// a client or an agent, not a browser page on someone else's site,
			// so there is no origin to check against.
			InsecureSkipVerify: true,
		})
		if err != nil {
			s.opts.Logger.Warn("websocket upgrade failed", "error", err)
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		c := &client{
			ws:  ws,
			out: make(chan protocol.ServerMessage, s.opts.OutBuffer),
			log: s.opts.Logger,
		}

		select {
		case s.register <- c:
		case <-ctx.Done():
			ws.Close(websocket.StatusGoingAway, "server stopped")
			return
		}
		defer func() {
			select {
			case s.unregister <- c:
			case <-time.After(time.Second):
			}
		}()

		go c.writeLoop(ctx)
		c.readLoop(ctx, s)
	})
}
