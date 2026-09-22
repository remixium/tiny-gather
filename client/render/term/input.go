package term

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// sendTimeout bounds how long a keypress waits on the socket.
const sendTimeout = 2 * time.Second

// key handles one keypress, reporting true when the user has asked to quit.
func (u *ui) key(ctx context.Context, ev *tcell.EventKey) bool {
	u.notice = ""

	switch u.mode {
	case modeChat, modeCommand:
		return u.keyPrompt(ctx, ev)
	}

	if ev.Key() == tcell.KeyCtrlC || ev.Key() == tcell.KeyEscape {
		return true
	}

	switch ev.Key() {
	case tcell.KeyUp:
		u.move(ctx, protocol.DirN)
	case tcell.KeyDown:
		u.move(ctx, protocol.DirS)
	case tcell.KeyLeft:
		u.move(ctx, protocol.DirW)
	case tcell.KeyRight:
		u.move(ctx, protocol.DirE)
	case tcell.KeyEnter:
		u.mode, u.input = modeChat, nil
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'q':
			return true
		case 'w':
			u.move(ctx, protocol.DirN)
		case 's':
			u.move(ctx, protocol.DirS)
		case 'a':
			u.move(ctx, protocol.DirW)
		case 'd':
			u.move(ctx, protocol.DirE)
		case 'g':
			u.gatherNearest(ctx)
		case 'p':
			u.pickupNearest(ctx)
		case 't':
			u.mode, u.input = modeChat, nil
		case ':':
			u.mode, u.input = modeCommand, nil
		}
	}
	return false
}

// keyPrompt edits the input line in chat or command mode.
func (u *ui) keyPrompt(ctx context.Context, ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		u.mode, u.input = modeNormal, nil
	case tcell.KeyEnter:
		text := strings.TrimSpace(string(u.input))
		mode := u.mode
		u.mode, u.input = modeNormal, nil
		if text == "" {
			return false
		}
		if mode == modeChat {
			u.do(ctx, protocol.Action{Kind: protocol.ActSay, Text: text})
		} else {
			u.command(ctx, text)
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(u.input) > 0 {
			u.input = u.input[:len(u.input)-1]
		}
	case tcell.KeyRune:
		u.input = append(u.input, ev.Rune())
	}
	return false
}

func (u *ui) move(ctx context.Context, d protocol.Dir) {
	u.do(ctx, protocol.Action{Kind: protocol.ActMove, Dir: d})
}

// gatherNearest targets the closest node the player is standing beside.
// Choosing a target is the client's convenience; whether it can be gathered is
// still the server's call.
func (u *ui) gatherNearest(ctx context.Context) {
	if id := u.adjacent(func(o protocol.ObjectView) bool {
		return o.Type == "tree" || o.Type == "rock"
	}); id != "" {
		u.do(ctx, protocol.Action{Kind: protocol.ActGather, Target: id})
		return
	}
	u.notice = "nothing to gather here"
}

func (u *ui) pickupNearest(ctx context.Context) {
	if id := u.adjacent(func(o protocol.ObjectView) bool {
		return o.Type == "ground_item"
	}); id != "" {
		u.do(ctx, protocol.Action{Kind: protocol.ActPickup, Target: id})
		return
	}
	u.notice = "nothing to pick up here"
}

// adjacent returns the id of the closest object satisfying ok that the player
// is within reach of, or "".
func (u *ui) adjacent(ok func(protocol.ObjectView) bool) string {
	if !u.haveFrame {
		return ""
	}
	self := u.frame.Observation.Self.Pos
	best, bestDist := "", 0.0
	for _, o := range u.frame.Observation.Object {
		if !ok(o) || !self.Adjacent(o.Pos) {
			continue
		}
		if best == "" || o.Distance < bestDist {
			best, bestDist = o.ID, o.Distance
		}
	}
	return best
}

// command parses a typed action. The forms mirror the protocol one to one, so
// that anything an agent can send, a person can try by hand.
func (u *ui) command(ctx context.Context, text string) {
	f := strings.Fields(text)
	if len(f) == 0 {
		return
	}
	usage := func(s string) { u.notice = "usage: " + s }

	switch f[0] {
	case "say":
		if len(f) < 2 {
			usage("say <text>")
			return
		}
		u.do(ctx, protocol.Action{Kind: protocol.ActSay, Text: strings.Join(f[1:], " ")})

	case "move":
		if len(f) != 2 {
			usage("move n|s|e|w")
			return
		}
		u.move(ctx, protocol.Dir(strings.ToUpper(f[1])))

	case "gather", "pickup":
		if len(f) != 2 {
			usage(f[0] + " <id>")
			return
		}
		u.do(ctx, protocol.Action{Kind: protocol.ActionKind(f[0]), Target: f[1]})

	case "drop":
		if len(f) != 3 {
			usage("drop <item> <n>")
			return
		}
		n, err := strconv.Atoi(f[2])
		if err != nil {
			usage("drop <item> <n>")
			return
		}
		u.do(ctx, protocol.Action{Kind: protocol.ActDrop, Item: protocol.Item(f[1]), N: n})

	case "deposit", "withdraw":
		if len(f) != 4 {
			usage(f[0] + " <id> <item> <n>")
			return
		}
		n, err := strconv.Atoi(f[3])
		if err != nil {
			usage(f[0] + " <id> <item> <n>")
			return
		}
		u.do(ctx, protocol.Action{
			Kind: protocol.ActionKind(f[0]), Target: f[1],
			Item: protocol.Item(f[2]), N: n,
		})

	default:
		u.notice = fmt.Sprintf("unknown command %q", f[0])
	}
}

func (u *ui) do(ctx context.Context, a protocol.Action) {
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	if err := u.client.Do(ctx, a); err != nil {
		u.notice = err.Error()
	}
}
