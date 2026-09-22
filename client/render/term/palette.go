package term

import (
	"github.com/gdamore/tcell/v2"

	"github.com/vadremix/tiny-gather/pkg/protocol"
)

// The palette uses only the sixteen ANSI colours.
//
// Named colours beyond those need 256-colour support, and on a terminal
// without it tcell substitutes the nearest basic colour: dark grey becomes
// black, and the floor disappears into the background. Anything that should
// read as "further away" uses the Dim attribute rather than a darker colour,
// for the same reason.
var (
	styleBase = tcell.StyleDefault

	// Terrain.
	styleFloor = styleBase.Foreground(tcell.ColorGreen)
	styleWater = styleBase.Foreground(tcell.ColorAqua).Background(tcell.ColorNavy)
	styleWall  = styleBase.Foreground(tcell.ColorSilver)
	styleHouse = styleBase.Foreground(tcell.ColorMaroon).Bold(true)
	styleFence = styleBase.Foreground(tcell.ColorOlive)

	// Entities.
	styleSelf   = styleBase.Foreground(tcell.ColorWhite).Bold(true)
	stylePlayer = styleBase.Foreground(tcell.ColorAqua).Bold(true)
	styleTree   = styleBase.Foreground(tcell.ColorLime).Bold(true)
	styleRock   = styleBase.Foreground(tcell.ColorSilver).Bold(true)
	styleStock  = styleBase.Foreground(tcell.ColorYellow).Bold(true)
	styleItem   = styleBase.Foreground(tcell.ColorFuchsia).Bold(true)
	styleLabel  = styleBase.Foreground(tcell.ColorOlive)

	// Panel.
	styleTitle  = styleBase.Foreground(tcell.ColorYellow).Bold(true)
	styleMuted  = styleBase.Foreground(tcell.ColorGray)
	styleNotice = styleBase.Foreground(tcell.ColorYellow)
	stylePrompt = styleBase.Foreground(tcell.ColorBlack).Background(tcell.ColorSilver)

	// Log.
	styleChat = styleBase.Foreground(tcell.ColorAqua)
	styleGood = styleBase.Foreground(tcell.ColorLime)
	styleBad  = styleBase.Foreground(tcell.ColorRed)
	styleWarn = styleBase.Foreground(tcell.ColorYellow)
)

// chestStyle colours a container by its declared colour, so "the blue chest"
// on screen is the blue chest in the observation.
func chestStyle(color string) tcell.Style {
	var c tcell.Color
	switch color {
	case "blue":
		c = tcell.ColorBlue
	case "red":
		c = tcell.ColorRed
	case "green":
		c = tcell.ColorGreen
	case "yellow":
		c = tcell.ColorYellow
	case "purple":
		c = tcell.ColorPurple
	case "grey", "gray":
		c = tcell.ColorGray
	default:
		c = tcell.ColorWhite
	}
	return styleBase.Foreground(c).Bold(true)
}

// itemStyle colours an item name wherever it appears in the panel.
func itemStyle(it protocol.Item) tcell.Style {
	switch it {
	case protocol.Wood:
		return styleBase.Foreground(tcell.ColorOlive)
	case protocol.Ore:
		return styleBase.Foreground(tcell.ColorSilver)
	default:
		return styleBase
	}
}

// objectStyle is the glyph and colour for a non-player entity.
func objectStyle(o protocol.ObjectView) (rune, tcell.Style) {
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

// terrainStyle is the glyph and colour for an impassable tile, decided by
// which landmark it belongs to. Every blocked tile is just "blocked" on the
// wire; the landmark extents are what let the pond look like water.
func terrainStyle(p protocol.Pos, landmarks []protocol.LandmarkView) (rune, tcell.Style) {
	for _, l := range landmarks {
		if !l.Contains(p) {
			continue
		}
		switch l.Name {
		case "pond":
			return '~', styleWater
		case "house":
			return '#', styleHouse
		case "fence":
			return '|', styleFence
		}
	}
	return '#', styleWall
}

// faded is how a style looks beyond perception range.
func faded(st tcell.Style) tcell.Style { return st.Dim(true) }
