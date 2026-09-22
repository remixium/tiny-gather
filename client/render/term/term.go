// Package term renders the game in a terminal.
//
// The renderer shows exactly what the server sent and nothing more. Terrain is
// drawn everywhere because it arrived with the welcome; entities are drawn only
// where the current frame reports them, and tiles beyond perception range are
// dimmed. There is deliberately no client-side memory of things seen earlier —
// what a human sees here is what an agent gets, and the point of the project is
// that keeping track of the rest is the player's own job.
package term

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"

	"github.com/vadremix/tiny-gather/client/conn"
	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// LogLines is how many recent messages the side panel keeps.
const LogLines = 200

// Run draws the game until the user quits or the connection ends.
func Run(ctx context.Context, c *conn.Client) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("term: opening screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("term: initialising screen: %w", err)
	}
	defer screen.Fini()
	return run(ctx, c, screen)
}

type mode int

const (
	modeNormal mode = iota
	modeChat
	modeCommand
)

// ui is everything the renderer knows. It is owned by the run loop and touched
// by nothing else.
type ui struct {
	screen tcell.Screen
	client *conn.Client
	world  protocol.MapView

	frame     conn.Frame
	haveFrame bool
	names     map[string]string

	mode   mode
	input  []rune
	log    []entry
	notice string
}

func newUI(c *conn.Client, screen tcell.Screen) *ui {
	return &ui{
		screen: screen,
		client: c,
		world:  c.Map(),
		names:  map[string]string{},
	}
}

// run is the loop proper, split from Run so that tests can drive it with a
// simulation screen.
func run(ctx context.Context, c *conn.Client, screen tcell.Screen) error {
	u := newUI(c, screen)

	keys := make(chan tcell.Event, 16)
	quit := make(chan struct{})
	go screen.ChannelEvents(keys, quit)
	defer close(quit)

	u.draw()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case f, ok := <-c.Frames():
			if !ok {
				if err := c.Err(); err != nil {
					return fmt.Errorf("term: connection ended: %w", err)
				}
				return nil
			}
			u.accept(f)
			u.draw()

		case ev := <-keys:
			switch ev := ev.(type) {
			case *tcell.EventResize:
				screen.Sync()
				u.draw()
			case *tcell.EventKey:
				if done := u.key(ctx, ev); done {
					return nil
				}
				u.draw()
			}
		}
	}
}

// accept takes a new frame, refreshing the name table and the message log.
func (u *ui) accept(f conn.Frame) {
	u.frame, u.haveFrame = f, true

	u.names[f.Observation.Self.ID] = f.Observation.Self.Name
	for _, p := range f.Observation.Player {
		u.names[p.ID] = p.Name
	}

	for _, ev := range f.Events {
		if text, st := u.describe(ev); text != "" {
			u.say(text, st)
		}
	}
}

func (u *ui) say(text string, st tcell.Style) {
	u.log = append(u.log, entry{text: text, style: st})
	if len(u.log) > LogLines {
		u.log = u.log[len(u.log)-LogLines:]
	}
}

func (u *ui) name(ref string) string {
	if n, ok := u.names[ref]; ok && n != "" {
		return n
	}
	return ref
}
