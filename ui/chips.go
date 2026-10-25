package ui

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"videocasino/palette"
)

// ChipR is the size of a chip lying on a table.
const ChipR = 11

// The colours of the chips, from the smallest a game has to the largest, as
// casino chips go: white, red, green, black and yellow. The ink is what the
// amount is written in on each.
var (
	chipFills = []color.Color{palette.White, palette.Crimson, palette.Forest, palette.Ink, palette.Gold}
	chipInks  = []color.Color{palette.Ink, palette.White, palette.White, palette.White, palette.Ink}
)

// chipText writes an amount of credits, a cent each, short enough to fit on
// a chip: whole dollars up to $999, then thousands, such as 2.5K.
func chipText(amount int) string {
	dollars := amount / 100
	switch {
	case dollars < 1000:
		return fmt.Sprintf("$%d", dollars)
	case dollars%1000 == 0:
		return fmt.Sprintf("%dK", dollars/1000)
	}
	return fmt.Sprintf("%.1fK", float64(dollars)/1000)
}

// DrawChip draws what is bet in one place as a single chip with the amount
// on it, in the colour of the largest of the game's chips that makes part
// of it. A lit chip has a bright rim, to mark a bet that has won.
func DrawChip(dst *ebiten.Image, x, y float64, amount int, chips []int, lit bool) {
	fill, ink := chipFills[0], chipInks[0]
	for i, n := range chips {
		if amount >= n && i < len(chipFills) {
			fill, ink = chipFills[i], chipInks[i]
		}
	}
	rim := color.Color(palette.Fog)
	if lit {
		rim = palette.Gold
	}
	FillCircle(dst, x, y, ChipR, palette.Ink)
	FillCircle(dst, x, y, ChipR-1, rim)
	FillCircle(dst, x, y, ChipR-3, fill)
	FrameCircle(dst, x, y, ChipR-3, 1, palette.Dusk)
	DrawText(dst, chipText(amount), x, y, 8, ink, AlignCenter, AlignCenter)
}
