package ui

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// ChipR is the size of a chip lying on a table.
const ChipR = 11

// The colours of the chips, from the smallest a game has to the largest.
var chipInks = []color.Color{LightBlue, LightRed, Green, Magenta, Gold}

// DrawChip draws what is bet in one place as a single chip with the amount
// on it, in the colour of the largest of the game's chips that makes part
// of it. A lit chip has a bright rim, to mark a bet that has won.
func DrawChip(dst *ebiten.Image, x, y float64, amount int, chips []int, lit bool) {
	ink := chipInks[0]
	for i, n := range chips {
		if amount >= n && i < len(chipInks) {
			ink = chipInks[i]
		}
	}
	rim, text := color.Color(White), color.Color(White)
	if lit {
		rim = Yellow
	}
	if ink == Gold {
		text = Black
	}
	FillCircle(dst, x, y, ChipR, Black)
	FillCircle(dst, x, y, ChipR-1, rim)
	FillCircle(dst, x, y, ChipR-3, ink)
	DrawText(dst, fmt.Sprint(amount), x, y, 8, text, AlignCenter, AlignCenter)
}
