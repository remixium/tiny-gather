package term

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// TestColoursReachTheScreen checks that styles are applied to cells, not just
// defined. It looks for specific glyphs and asserts their colour, so a palette
// that silently falls back to the default would fail here.
func TestColoursReachTheScreen(t *testing.T) {
	screen := session(t, "painter", nil, 50*time.Millisecond)
	cells, w, h := screen.GetContents()

	type found struct {
		fg, bg tcell.Color
		attr   tcell.AttrMask
	}
	find := func(want rune) (found, bool) {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := cells[y*w+x]
				if len(c.Runes) > 0 && c.Runes[0] == want {
					fg, bg, attr := c.Style.Decompose()
					return found{fg, bg, attr}, true
				}
			}
		}
		return found{}, false
	}

	// The player is bold white.
	if f, ok := find('@'); !ok {
		t.Error("player not drawn")
	} else if f.fg != tcell.ColorWhite || f.attr&tcell.AttrBold == 0 {
		t.Errorf("player is fg=%v attr=%v, want bold white", f.fg, f.attr)
	}

	// The nearest chest in this fixture is blue, and the blue chest must be
	// blue: that is what "the blue chest" on screen means.
	if f, ok := find('='); !ok {
		t.Error("no chest drawn")
	} else if f.fg != tcell.ColorBlue {
		t.Errorf("chest is fg=%v, want blue", f.fg)
	}

	// Trees are bright green.
	if f, ok := find('T'); !ok {
		t.Error("no tree drawn")
	} else if f.fg != tcell.ColorLime {
		t.Errorf("tree is fg=%v, want lime", f.fg)
	}

	// The pond reads as water: a navy background, whether or not it is in
	// range (dimming keeps the colour, it just fades it).
	if f, ok := find('~'); !ok {
		t.Error("the pond is not drawn as water; landmark extents are not reaching the client")
	} else if f.bg != tcell.ColorNavy {
		t.Errorf("water has bg=%v, want navy", f.bg)
	}

	// House walls are distinguishable from generic walls.
	if f, ok := find('#'); !ok {
		t.Error("no wall drawn")
	} else if f.fg != tcell.ColorMaroon {
		t.Errorf("house wall is fg=%v, want maroon", f.fg)
	}

	// Seen floor is green, not grey: grey collapses to black on a
	// 16-colour terminal and the floor vanishes.
	if f, ok := find('.'); !ok {
		t.Error("no floor drawn")
	} else if f.fg != tcell.ColorGreen {
		t.Errorf("floor is fg=%v, want green", f.fg)
	}
}
