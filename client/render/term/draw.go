package term

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// sidebarWidth is the fixed width of the panel to the right of the map.
const sidebarWidth = 32

// draw repaints the whole screen from the current frame.
func (u *ui) draw() {
	s := u.screen
	s.Clear()
	w, h := s.Size()

	mapW := w - sidebarWidth - 1
	mapH := h - 1
	if mapW < 8 || mapH < 4 {
		u.text(0, 0, "terminal too small", styleNotice)
		s.Show()
		return
	}

	u.drawMap(0, 0, mapW, mapH)
	u.drawSidebar(mapW+1, 0, sidebarWidth, mapH)
	u.drawBottom(h-1, w)
	s.Show()
}

// drawMap paints a viewport of the world centred on the player.
func (u *ui) drawMap(x0, y0, w, h int) {
	if u.world.W == 0 || u.world.H == 0 {
		u.text(x0, y0, "waiting for map...", styleMuted)
		return
	}

	var self protocol.Pos
	var landmarks []protocol.LandmarkView
	if u.haveFrame {
		self = u.frame.Observation.Self.Pos
		landmarks = u.frame.Observation.Landmark
	}

	// The viewport follows the player but never scrolls past the map edge,
	// so a small map sits still in a large terminal.
	ox, oy := self.X-w/2, self.Y-h/2
	ox = clamp(ox, 0, max(0, u.world.W-w))
	oy = clamp(oy, 0, max(0, u.world.H-h))

	blocked := map[protocol.Pos]bool{}
	for _, p := range u.world.Blocked {
		blocked[p] = true
	}

	for sy := 0; sy < h; sy++ {
		for sx := 0; sx < w; sx++ {
			p := protocol.Pos{X: ox + sx, Y: oy + sy}
			if p.X >= u.world.W || p.Y >= u.world.H {
				continue
			}
			seen := u.haveFrame && dist(self, p) <= u.world.Radius
			switch {
			case blocked[p]:
				r, st := terrainStyle(p, landmarks)
				if !seen {
					st = faded(st)
				}
				u.screen.SetContent(x0+sx, y0+sy, r, nil, st)
			case seen:
				u.screen.SetContent(x0+sx, y0+sy, '.', nil, styleFloor)
			}
		}
	}

	put := func(p protocol.Pos, r rune, st tcell.Style) {
		sx, sy := p.X-ox, p.Y-oy
		if sx >= 0 && sy >= 0 && sx < w && sy < h {
			u.screen.SetContent(x0+sx, y0+sy, r, nil, st)
		}
	}

	if !u.haveFrame {
		return
	}
	obs := u.frame.Observation

	for _, o := range obs.Object {
		r, st := objectStyle(o)
		put(o.Pos, r, st)
	}
	for _, p := range obs.Player {
		st := stylePlayer
		if p.Thinking {
			st = st.Italic(true)
		}
		put(p.Pos, '&', st)
	}
	put(self, '@', styleSelf)

	// Labels go on last and only into cells nothing stands in, so a landmark
	// name never hides the chest that is next to it.
	for _, l := range obs.Landmark {
		lx, ly := l.Pos.X-ox+1, l.Pos.Y-oy
		if ly < 0 || ly >= h {
			continue
		}
		for i, r := range l.Name {
			cx := lx + i
			if cx < 0 || cx >= w || occupied(u.screen, x0+cx, y0+ly) {
				continue
			}
			u.screen.SetContent(x0+cx, y0+ly, r, nil, styleLabel)
		}
	}
}

// occupied reports whether a cell already shows an entity rather than floor,
// terrain or nothing.
func occupied(s tcell.Screen, x, y int) bool {
	r, _, _, _ := s.GetContent(x, y)
	switch r {
	case ' ', '.', '#', '~', '|', 0:
		return false
	}
	return true
}

// part is a run of text in one style, for lines that mix colours.
type part struct {
	text  string
	style tcell.Style
}

// drawSidebar lists the player's state, what is nearby, and recent messages.
func (u *ui) drawSidebar(x0, y0, w, h int) {
	y := y0
	line := func(ps ...part) {
		if y >= y0+h {
			return
		}
		x := x0
		for _, p := range ps {
			if x-x0 >= w {
				break
			}
			s := p.text
			if x-x0+len(s) > w {
				s = fit(s, w-(x-x0))
			}
			u.text(x, y, s, p.style)
			x += len(s)
		}
		y++
	}
	plain := func(s string, st tcell.Style) { line(part{fit(s, w), st}) }

	if !u.haveFrame {
		plain("connecting...", styleMuted)
		return
	}
	obs := u.frame.Observation
	self := obs.Self

	line(part{self.Name, styleTitle}, part{fmt.Sprintf("  tick %d", u.frame.Tick), styleMuted})
	plain(fmt.Sprintf("%s at %d,%d", self.ID, self.Pos.X, self.Pos.Y), styleBase)
	if self.Doing != "" {
		plain("doing: "+self.Doing, styleNotice)
	}
	plain("", styleBase)

	plain(fmt.Sprintf("inventory  %d/%d slots", self.UsedSlots, self.Slots), styleTitle)
	if len(self.Inventory) == 0 {
		plain("  empty", styleMuted)
	}
	for _, it := range sortedItems(self.Inventory) {
		line(
			part{"  ", styleBase},
			part{fmt.Sprintf("%-8s", it), itemStyle(it)},
			part{fmt.Sprint(self.Inventory[it]), styleBase},
		)
	}
	plain("", styleBase)

	plain("nearby", styleTitle)
	near := append([]protocol.ObjectView(nil), obs.Object...)
	sort.Slice(near, func(i, j int) bool { return near[i].Distance < near[j].Distance })
	shown := 0
	for _, o := range near {
		if shown >= 8 {
			plain(fmt.Sprintf("  ... %d more", len(near)-shown), styleMuted)
			break
		}
		// The list line takes the object's own colour, so the entry and the
		// glyph on the map read as the same thing.
		_, st := objectStyle(o)
		plain("  "+summary(o), st.Bold(false))
		shown++
	}
	for _, p := range obs.Player {
		note := ""
		if p.Thinking {
			note = " (thinking)"
		}
		plain(fmt.Sprintf("  & %s %.0f away%s", p.Name, p.Distance, note), stylePlayer.Bold(false))
	}
	plain("", styleBase)

	// Whatever room is left goes to the log, newest at the bottom.
	remaining := y0 + h - y
	if remaining <= 0 {
		return
	}
	start := len(u.log) - remaining
	if start < 0 {
		start = 0
	}
	for _, e := range u.log[start:] {
		plain(e.text, e.style)
	}
}

// summary is one line about an object: id, walk cost, and what matters about
// it. Path cost is shown rather than straight-line distance because it is the
// number that decides how long getting there takes.
func summary(o protocol.ObjectView) string {
	cost := "?"
	if o.PathCost >= 0 {
		cost = fmt.Sprint(o.PathCost)
	}
	var detail string
	switch o.Type {
	case "tree", "rock":
		detail = fmt.Sprintf("%d %s", o.Remaining, o.Resource)
	case "chest", "stockpile":
		detail = o.Color
		if o.Filter != "" {
			detail += " " + string(o.Filter) + "-only"
		}
		if n := total(o.Contents); n > 0 {
			detail += fmt.Sprintf(" [%d]", n)
		}
	case "ground_item":
		detail = fmt.Sprintf("%d %s", o.Count, o.Item)
	}
	s := fmt.Sprintf("%s %s: %s", o.ID, cost, detail)
	if o.Near != "" {
		s += " by " + o.Near
	}
	return s
}

// drawBottom paints the input line or the key hints.
func (u *ui) drawBottom(y, w int) {
	switch u.mode {
	case modeChat:
		u.text(0, y, fit("say: "+string(u.input), w), stylePrompt)
	case modeCommand:
		u.text(0, y, fit(":"+string(u.input), w), stylePrompt)
	default:
		if u.notice != "" {
			u.text(0, y, fit(u.notice, w), styleNotice)
			return
		}
		u.text(0, y, fit("wasd move  g gather  p pick up  t talk  : command  q quit", w), styleMuted)
	}
}

func (u *ui) text(x, y int, s string, st tcell.Style) {
	for i, r := range s {
		u.screen.SetContent(x+i, y, r, nil, st)
	}
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if len(s) <= w {
		return s + strings.Repeat(" ", w-len(s))
	}
	if w == 1 {
		return s[:1]
	}
	return s[:w-1] + "…"
}

func sortedItems(m map[protocol.Item]int) []protocol.Item {
	out := make([]protocol.Item, 0, len(m))
	for it := range m {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func total(m map[protocol.Item]int) int {
	n := 0
	for _, c := range m {
		n += c
	}
	return n
}

func dist(a, b protocol.Pos) float64 {
	dx, dy := float64(a.X-b.X), float64(a.Y-b.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
