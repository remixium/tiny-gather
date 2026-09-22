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

var (
	styleBase    = tcell.StyleDefault
	styleDim     = tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
	styleWall    = tcell.StyleDefault.Foreground(tcell.ColorGray)
	styleWallDim = tcell.StyleDefault.Foreground(tcell.ColorDarkSlateGray)
	styleSelf    = tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true)
	stylePlayer  = tcell.StyleDefault.Foreground(tcell.ColorAqua)
	styleTree    = tcell.StyleDefault.Foreground(tcell.ColorGreen)
	styleRock    = tcell.StyleDefault.Foreground(tcell.ColorSilver)
	styleItem    = tcell.StyleDefault.Foreground(tcell.ColorFuchsia)
	styleStock   = tcell.StyleDefault.Foreground(tcell.ColorYellow)
	styleLabel   = tcell.StyleDefault.Foreground(tcell.ColorOlive)
	styleTitle   = tcell.StyleDefault.Bold(true)
	styleNotice  = tcell.StyleDefault.Foreground(tcell.ColorYellow)
	styleInput   = tcell.StyleDefault.Reverse(true)
)

// chestStyle colours a container by its declared colour, so "the blue chest"
// on screen is the blue chest in the observation.
func chestStyle(color string) tcell.Style {
	switch color {
	case "blue":
		return styleBase.Foreground(tcell.ColorBlue)
	case "red":
		return styleBase.Foreground(tcell.ColorRed)
	case "green":
		return styleBase.Foreground(tcell.ColorGreen)
	case "yellow":
		return styleBase.Foreground(tcell.ColorYellow)
	case "grey", "gray":
		return styleBase.Foreground(tcell.ColorGray)
	default:
		return styleBase.Foreground(tcell.ColorWhite)
	}
}

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
		u.text(x0, y0, "waiting for map...", styleDim)
		return
	}

	var self protocol.Pos
	if u.haveFrame {
		self = u.frame.Observation.Self.Pos
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
			case blocked[p] && seen:
				u.screen.SetContent(x0+sx, y0+sy, '#', nil, styleWall)
			case blocked[p]:
				u.screen.SetContent(x0+sx, y0+sy, '#', nil, styleWallDim)
			case seen:
				u.screen.SetContent(x0+sx, y0+sy, '.', nil, styleDim)
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
		r, st := glyph(o)
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
			if cx < 0 || cx >= w {
				continue
			}
			if occupied(u.screen, x0+cx, y0+ly) {
				continue
			}
			u.screen.SetContent(x0+cx, y0+ly, r, nil, styleLabel)
		}
	}
}

// occupied reports whether a cell already shows an entity rather than floor,
// wall or nothing.
func occupied(s tcell.Screen, x, y int) bool {
	r, _, _, _ := s.GetContent(x, y)
	switch r {
	case ' ', '.', '#', 0:
		return false
	}
	return true
}
func glyph(o protocol.ObjectView) (rune, tcell.Style) {
	switch o.Type {
	case "tree":
		return 'T', styleTree
	case "rock":
		return '^', styleRock
	case "chest":
		return '=', chestStyle(o.Color)
	case "stockpile":
		return 'S', styleStock
	case "ground_item":
		return '*', styleItem
	default:
		return '?', styleBase
	}
}

// drawSidebar lists the player's state, what is nearby, and recent messages.
func (u *ui) drawSidebar(x0, y0, w, h int) {
	y := y0
	line := func(s string, st tcell.Style) {
		if y < y0+h {
			u.text(x0, y, fit(s, w), st)
			y++
		}
	}

	if !u.haveFrame {
		line("connecting...", styleDim)
		return
	}
	obs := u.frame.Observation
	self := obs.Self

	line(fmt.Sprintf("%s  tick %d", self.Name, u.frame.Tick), styleTitle)
	line(fmt.Sprintf("%s at %d,%d", self.ID, self.Pos.X, self.Pos.Y), styleBase)
	if self.Doing != "" {
		line("doing: "+self.Doing, styleNotice)
	}
	line("", styleBase)

	line(fmt.Sprintf("inventory (%d/%d slots)", self.UsedSlots, self.Slots), styleTitle)
	if len(self.Inventory) == 0 {
		line("  empty", styleDim)
	}
	for _, it := range sortedItems(self.Inventory) {
		line(fmt.Sprintf("  %-8s %d", it, self.Inventory[it]), styleBase)
	}
	line("", styleBase)

	line("nearby", styleTitle)
	near := append([]protocol.ObjectView(nil), obs.Object...)
	sort.Slice(near, func(i, j int) bool { return near[i].Distance < near[j].Distance })
	shown := 0
	for _, o := range near {
		if shown >= 8 {
			line(fmt.Sprintf("  ... %d more", len(near)-shown), styleDim)
			break
		}
		line("  "+summary(o), styleBase)
		shown++
	}
	for _, p := range obs.Player {
		st := stylePlayer
		note := ""
		if p.Thinking {
			note = " (thinking)"
		}
		line(fmt.Sprintf("  & %s %.0f away%s", p.Name, p.Distance, note), st)
	}
	line("", styleBase)

	// Whatever room is left goes to the log, newest at the bottom.
	remaining := y0 + h - y
	if remaining <= 0 {
		return
	}
	start := len(u.log) - remaining
	if start < 0 {
		start = 0
	}
	for _, l := range u.log[start:] {
		line(l, styleBase)
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
		u.text(0, y, fit("say: "+string(u.input), w), styleInput)
	case modeCommand:
		u.text(0, y, fit(":"+string(u.input), w), styleInput)
	default:
		if u.notice != "" {
			u.text(0, y, fit(u.notice, w), styleNotice)
			return
		}
		u.text(0, y, fit("wasd move  g gather  p pick up  t talk  : command  q quit", w), styleDim)
	}
}

func (u *ui) text(x, y int, s string, st tcell.Style) {
	for i, r := range s {
		u.screen.SetContent(x+i, y, r, nil, st)
	}
}

func fit(s string, w int) string {
	if len(s) <= w {
		return s + strings.Repeat(" ", w-len(s))
	}
	if w <= 1 {
		return s[:w]
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
