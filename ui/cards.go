package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"videocasino/cards"
)

// The size of a card at scale 1.
const (
	CardW = 120
	CardH = 168
)

// The colours of a playing card.
var (
	cardFace  = color.RGBA{250, 250, 244, 255}
	cardEdge  = color.RGBA{120, 120, 120, 255}
	cardRed   = color.RGBA{204, 16, 32, 255}
	cardBlack = color.RGBA{20, 20, 24, 255}
	backDark  = color.RGBA{140, 16, 28, 255}
	backLight = color.RGBA{200, 60, 70, 255}
)

var rankText = [...]string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}

// DrawCard draws the face of a card with its top left corner at (x, y): a
// white card with its rank and suit in the top corner and its rank again,
// large, at the bottom.
func DrawCard(dst *ebiten.Image, c cards.Card, x, y, scale float64) {
	s := scale
	FillRoundRect(dst, x, y, CardW*s, CardH*s, 10*s, cardEdge)
	FillRoundRect(dst, x+1.5*s, y+1.5*s, (CardW-3)*s, (CardH-3)*s, 9*s, cardFace)

	ink := color.Color(cardBlack)
	if c.Suit() == cards.Diamonds || c.Suit() == cards.Hearts {
		ink = cardRed
	}
	rank := rankText[c.Rank()-cards.Two]
	DrawText(dst, rank, x+27*s, y+3*s, 36*s, ink, AlignCenter, AlignStart)
	DrawSuit(dst, c.Suit(), x+27*s, y+62*s, 14*s, ink)

	// The ten is the one rank with two figures, and needs them smaller to
	// fit.
	size := 92.0
	if c.Rank() == cards.Ten {
		size = 74
	}
	DrawText(dst, rank, x+(CardW-10)*s, y+(CardH-3)*s, size*s, ink, AlignEnd, AlignEnd)
}

// DrawCardBack draws a card lying face down.
func DrawCardBack(dst *ebiten.Image, x, y, scale float64) {
	s := scale
	FillRoundRect(dst, x, y, CardW*s, CardH*s, 10*s, cardEdge)
	FillRoundRect(dst, x+1.5*s, y+1.5*s, (CardW-3)*s, (CardH-3)*s, 9*s, cardFace)
	FillRoundRect(dst, x+8*s, y+8*s, (CardW-16)*s, (CardH-16)*s, 5*s, backDark)

	// A lattice of diamonds.
	const cols, rows = 4, 6
	w, h := (CardW-28)*s/cols, (CardH-28)*s/rows
	for row := range rows {
		for col := range cols {
			cx := x + 14*s + (float64(col)+0.5)*w
			cy := y + 14*s + (float64(row)+0.5)*h
			FillPoly(dst, backLight, cx, cy-h/2, cx+w/2, cy, cx, cy+h/2, cx-w/2, cy)
			FillPoly(dst, backDark, cx, cy-h/4, cx+w/4, cy, cx, cy+h/4, cx-w/4, cy)
		}
	}
}

// DrawSuit draws a suit's symbol centred on (cx, cy). It reaches h above and
// below the centre.
func DrawSuit(dst *ebiten.Image, suit cards.Suit, cx, cy, h float64, c color.Color) {
	// x and y turn offsets from the centre, in units of h, into screen
	// positions.
	x := func(dx float64) float64 { return cx + dx*h }
	y := func(dy float64) float64 { return cy + dy*h }
	stem := func() {
		FillPoly(dst, c, x(0), y(0.1), x(0.38), y(1), x(-0.38), y(1))
	}

	switch suit {
	case cards.Diamonds:
		FillPoly(dst, c, x(0), y(-1), x(0.78), y(0), x(0), y(1), x(-0.78), y(0))

	case cards.Hearts:
		FillCircle(dst, x(-0.47), y(-0.42), 0.53*h, c)
		FillCircle(dst, x(0.47), y(-0.42), 0.53*h, c)
		FillPoly(dst, c, x(-0.96), y(-0.2), x(0.96), y(-0.2), x(0), y(1))

	case cards.Spades:
		FillCircle(dst, x(-0.47), y(0.2), 0.5*h, c)
		FillCircle(dst, x(0.47), y(0.2), 0.5*h, c)
		FillPoly(dst, c, x(-0.93), y(0), x(0.93), y(0), x(0), y(-1))
		stem()

	case cards.Clubs:
		FillCircle(dst, x(0), y(-0.52), 0.46*h, c)
		FillCircle(dst, x(-0.5), y(0.18), 0.46*h, c)
		FillCircle(dst, x(0.5), y(0.18), 0.46*h, c)
		FillCircle(dst, x(0), y(0), 0.3*h, c)
		stem()
	}
}
