package slot

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"videocasino/games/slot/machine"
	"videocasino/palette"
	"videocasino/ui"
)

// The game's own colours, which the palette has no good match for.
var (
	reelBack = color.RGBA{16, 16, 24, 255}
	plumSkin = color.RGBA{120, 70, 220, 255}
	plumGlow = color.RGBA{175, 135, 255, 255}
	goldDark = color.RGBA{190, 110, 0, 255}
	chipFelt = color.RGBA{0, 0, 96, 255}
)

// The colour of each card symbol.
var cardInk = map[machine.Symbol]color.Color{
	machine.Ten:   palette.Seafoam,
	machine.Jack:  palette.Leaf,
	machine.Queen: palette.Flamingo,
	machine.King:  palette.Blue,
	machine.Ace:   palette.White,
}

var cardText = map[machine.Symbol]string{
	machine.Ten:   "10",
	machine.Jack:  "J",
	machine.Queen: "Q",
	machine.King:  "K",
	machine.Ace:   "A",
}

// drawSymbol draws a symbol centred on (cx, cy). At scale 1 it fills a
// 120x90 cell; the reels, paytable and menu use smaller scales.
func drawSymbol(dst *ebiten.Image, sym machine.Symbol, cx, cy, scale float64) {
	// x and y turn offsets from the centre into screen positions.
	x := func(dx float64) float64 { return cx + dx*scale }
	y := func(dy float64) float64 { return cy + dy*scale }
	s := scale

	switch sym {
	case machine.Ten, machine.Jack, machine.Queen, machine.King, machine.Ace:
		ui.DrawText(dst, cardText[sym], x(3), y(3), 64*s, palette.Dusk, ui.AlignCenter, ui.AlignCenter)
		ui.DrawText(dst, cardText[sym], x(0), y(0), 64*s, cardInk[sym], ui.AlignCenter, ui.AlignCenter)

	case machine.Cherry:
		ui.DrawLine(dst, x(-14), y(8), x(4), y(-32), 3*s, palette.Rust)
		ui.DrawLine(dst, x(14), y(10), x(4), y(-32), 3*s, palette.Rust)
		ui.FillCircle(dst, x(14), y(-30), 7*s, palette.Forest)
		ui.FillCircle(dst, x(-14), y(12), 16*s, palette.Crimson)
		ui.FillCircle(dst, x(14), y(14), 16*s, palette.Crimson)
		ui.FillCircle(dst, x(-19), y(7), 4*s, palette.Red)
		ui.FillCircle(dst, x(9), y(9), 4*s, palette.Red)

	case machine.Plum:
		ui.DrawLine(dst, x(4), y(-22), x(10), y(-38), 3*s, palette.Rust)
		ui.FillCircle(dst, x(18), y(-34), 7*s, palette.Forest)
		ui.FillCircle(dst, x(0), y(4), 28*s, plumSkin)
		ui.FillCircle(dst, x(-10), y(-7), 7*s, plumGlow)

	case machine.Bell:
		ui.FillCircle(dst, x(0), y(-30), 5*s, palette.Amber)
		ui.FillCircle(dst, x(0), y(28), 7*s, goldDark)
		ui.FillCircle(dst, x(0), y(-8), 20*s, palette.Amber)
		ui.FillPoly(dst, palette.Amber, x(-20), y(-8), x(20), y(-8), x(28), y(20), x(-28), y(20))
		ui.FillRect(dst, x(-32), y(18), 64*s, 7*s, goldDark)

	case machine.Dollar:
		// The S is built from straight strokes, as in the original.
		w := 6 * s
		ui.DrawLine(dst, x(-18), y(-30), x(18), y(-30), w, palette.Gold)
		ui.DrawLine(dst, x(-18), y(-33), x(-18), y(3), w, palette.Gold)
		ui.DrawLine(dst, x(18), y(-33), x(18), y(-22), w, palette.Gold)
		ui.DrawLine(dst, x(-18), y(0), x(18), y(0), w, palette.Gold)
		ui.DrawLine(dst, x(18), y(-3), x(18), y(33), w, palette.Gold)
		ui.DrawLine(dst, x(-18), y(30), x(18), y(30), w, palette.Gold)
		ui.DrawLine(dst, x(-18), y(33), x(-18), y(22), w, palette.Gold)
		ui.DrawLine(dst, x(-6), y(-40), x(-6), y(40), 3*s, palette.Gold)
		ui.DrawLine(dst, x(7), y(-40), x(7), y(40), 3*s, palette.Gold)

	case machine.Wild:
		// A jester's cap of three points, each tipped with a bell, over a
		// banner that says what the joker does.
		ui.FillPoly(dst, palette.Flamingo, x(-36), y(10), x(-8), y(10), x(-42), y(-26))
		ui.FillPoly(dst, palette.Flamingo, x(8), y(10), x(36), y(10), x(42), y(-26))
		ui.FillPoly(dst, palette.Leaf, x(-16), y(10), x(16), y(10), x(0), y(-38))
		ui.FillCircle(dst, x(-42), y(-28), 5*s, palette.Amber)
		ui.FillCircle(dst, x(42), y(-28), 5*s, palette.Amber)
		ui.FillCircle(dst, x(0), y(-40), 5*s, palette.Amber)
		ui.FillRect(dst, x(-38), y(8), 76*s, 7*s, palette.Amber)
		ui.FillRect(dst, x(-42), y(18), 84*s, 24*s, palette.White)
		ui.DrawText(dst, "WILD", x(0), y(30), 20*s, palette.Crimson, ui.AlignCenter, ui.AlignCenter)

	case machine.Scatter:
		drawDie(dst, x(-17), y(-10), 19*s, -0.22, 5)
		drawDie(dst, x(17), y(2), 19*s, 0.18, 3)
		ui.DrawText(dst, "FREE GAMES", x(0), y(36), 12*s, palette.Gold, ui.AlignCenter, ui.AlignCenter)

	case machine.Link:
		drawChip(dst, cx, cy, scale, "")
	}
}

// pips gives the spots on each face of a die, as positions on a 3x3 grid
// centred on the face.
var pips = map[int][][2]float64{
	3: {{-1, -1}, {0, 0}, {1, 1}},
	5: {{-1, -1}, {1, -1}, {0, 0}, {-1, 1}, {1, 1}},
}

// drawDie draws a die face centred on (cx, cy) and turned by angle radians.
// half is half the length of a side.
func drawDie(dst *ebiten.Image, cx, cy, half, angle float64, face int) {
	sin, cos := math.Sincos(angle)
	at := func(dx, dy float64) (float64, float64) {
		return cx + dx*cos - dy*sin, cy + dx*sin + dy*cos
	}
	square := func(h float64, c color.Color) {
		x0, y0 := at(-h, -h)
		x1, y1 := at(h, -h)
		x2, y2 := at(h, h)
		x3, y3 := at(-h, h)
		ui.FillPoly(dst, c, x0, y0, x1, y1, x2, y2, x3, y3)
	}
	square(half, palette.Fog)
	square(half*0.9, palette.White)
	for _, p := range pips[face] {
		px, py := at(p[0]*half*0.52, p[1]*half*0.52)
		ui.FillCircle(dst, px, py, half*0.17, palette.Ink)
	}
}

// drawChip draws a poker chip centred on (cx, cy) with a label in the
// middle: the credits or the jackpot it carries.
func drawChip(dst *ebiten.Image, cx, cy, scale float64, label string) {
	s := scale
	ui.FillCircle(dst, cx, cy, 39*s, palette.White)
	ui.FillCircle(dst, cx, cy, 37*s, palette.Crimson)
	// White blocks round the rim.
	for i := range 8 {
		sin, cos := math.Sincos(float64(i) * math.Pi / 4)
		ui.DrawLine(dst, cx+cos*28*s, cy+sin*28*s, cx+cos*37*s, cy+sin*37*s, 10*s, palette.White)
	}
	ui.FillCircle(dst, cx, cy, 27*s, palette.White)
	ui.FillCircle(dst, cx, cy, 24*s, chipFelt)

	size := 20.0
	switch {
	case len(label) > 5:
		size = 11
	case len(label) > 3:
		size = 13
	}
	ui.DrawText(dst, label, cx, cy, size*s, palette.Gold, ui.AlignCenter, ui.AlignCenter)
}
