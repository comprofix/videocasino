package ui

import (
	"image/color"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"videocasino/palette"
)

// Pressed reports whether any of the keys has just been pressed.
func Pressed(keys ...ebiten.Key) bool {
	for _, k := range keys {
		if inpututil.IsKeyJustPressed(k) {
			return true
		}
	}
	return false
}

// Dismissed reports whether the player has asked to leave a screen that is
// only there to be read, such as a game's help: with ENTER, SPACE or ESC, or
// a click anywhere.
func Dismissed() bool {
	return Pressed(ebiten.KeyEnter, ebiten.KeyNumpadEnter, ebiten.KeySpace, ebiten.KeyEscape) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
}

// Step returns the choice one place up or down from current, stopping at
// either end of the list.
func Step(choices []int, current, by int) int {
	i := slices.Index(choices, current) + by
	return choices[min(max(i, 0), len(choices)-1)]
}

// Button is a clickable rectangle.
type Button struct{ X, Y, W, H float64 }

func (b Button) Hit(x, y int) bool {
	fx, fy := float64(x), float64(y)
	return fx >= b.X && fx < b.X+b.W && fy >= b.Y && fy < b.Y+b.H
}

// DrawButton draws a button with one or two lines of text. A selected
// button is shown lit, and a locked one dimmed.
func DrawButton(dst *ebiten.Image, b Button, selected, locked bool, size float64, lines ...string) {
	fill, edge, ink := color.Color(palette.Cobalt), color.Color(palette.Blue), color.Color(palette.White)
	switch {
	case selected:
		fill, edge, ink = palette.Gold, palette.White, palette.Ink
	case locked:
		fill, edge, ink = palette.Ink, palette.Dusk, palette.Dusk
	}
	FillRect(dst, b.X, b.Y, b.W, b.H, fill)
	FrameRect(dst, b.X, b.Y, b.W, b.H, 2, edge)

	step := size + 3
	y := b.Y + b.H/2 - step*float64(len(lines)-1)/2
	for _, l := range lines {
		DrawText(dst, l, b.X+b.W/2, y, size, ink, AlignCenter, AlignCenter)
		y += step
	}
}

// DrawCoinButton draws the button that inserts a coin, which looks the same
// wherever it is.
func DrawCoinButton(dst *ebiten.Image, b Button, size float64, label string) {
	FillRect(dst, b.X, b.Y, b.W, b.H, palette.Amber)
	FrameRect(dst, b.X, b.Y, b.W, b.H, 2, palette.Gold)
	DrawText(dst, label, b.X+b.W/2, b.Y+b.H/2, size, palette.Ink, AlignCenter, AlignCenter)
}

// DrawResetButton draws the button that resets the credits.
func DrawResetButton(dst *ebiten.Image, b Button, size float64) {
	FillRect(dst, b.X, b.Y, b.W, b.H, palette.Crimson)
	FrameRect(dst, b.X, b.Y, b.W, b.H, 2, palette.Red)
	DrawText(dst, "RESET", b.X+b.W/2, b.Y+b.H/2, size, palette.White, AlignCenter, AlignCenter)
}
