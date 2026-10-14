package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// DrawShoe draws a shoe of cards as a box that empties from the front as
// they are dealt, so that it shows how far through the shoe is, not a
// count. full is the part of the cards still in it, from 0 to 1, and cut
// where the cut card stands, as a part of the whole shoe in from the front
// of what is left; a cut card that is not in the shoe is given as less than
// nought. The note is written underneath.
func DrawShoe(dst *ebiten.Image, b Button, full, cut float64, edge color.Color, note string, ink color.Color) {
	DrawText(dst, "SHOE", b.X+b.W/2, b.Y-17, 11, LightGray, AlignCenter, AlignStart)
	FillRect(dst, b.X, b.Y, b.W, b.H, Black)

	// The cards are dealt from the left, so those left fill the right.
	left := full * (b.W - 4)
	FillRect(dst, b.X+b.W-2-left, b.Y+2, left, b.H-4, LightGray)
	if cut >= 0 {
		at := b.X + b.W - 2 - left + cut*(b.W-4)
		FillRect(dst, at-1, b.Y-3, 3, b.H+6, Yellow)
	}
	FrameRect(dst, b.X, b.Y, b.W, b.H, 2, edge)
	DrawText(dst, note, b.X+b.W/2, b.Y+b.H+6, 10, ink, AlignCenter, AlignStart)
}

// DrawCutCard draws the cut card with its top left corner at (x, y): a
// plain yellow card, as it lies on the table once it has come out of the
// shoe.
func DrawCutCard(dst *ebiten.Image, x, y, scale float64) {
	w, h := CardW*scale, CardH*scale
	FillRoundRect(dst, x, y, w, h, 10*scale, DarkGray)
	FillRoundRect(dst, x+1.5, y+1.5, w-3, h-3, 9*scale, Yellow)
	DrawText(dst, "CUT", x+w/2, y+h/2, 30*scale, Black, AlignCenter, AlignCenter)
}
