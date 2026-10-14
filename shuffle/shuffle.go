// Package shuffle shows a shoe of cards being shuffled, for any card game
// that deals from one. It is for show: the shoe itself is in the shoe
// package, and is filled again whether the shuffle is watched or not.
//
// A game notes the tick at which its shuffle began, and from then on hands
// the number of ticks gone by to Falls, for the sound, and to Draw, until
// they come to Ticks.
package shuffle

import (
	"github.com/hajimehoshi/ebiten/v2"

	"videocasino/ui"
)

// How the shuffle goes: so many riffles, each of so many ticks, with a card
// falling every few ticks while the two halves run together.
const (
	riffles     = 4
	riffleTicks = 40
	riffleCards = 8   // cards that fall in a riffle, from each half in turn
	riffleFall  = 0.6 // the part of a riffle the cards fall for

	// Ticks is how long the shuffle takes.
	Ticks = riffles * riffleTicks
)

// Sound returns the sound of a card falling in a riffle: a quiet card.
func Sound() []byte {
	return ui.Tone(165, 28, 0.12, true)
}

// Falls reports whether a card falls at tick t of the shuffle, when its
// sound is due. One falls at even steps through the first part of each
// riffle.
func Falls(t int) bool {
	fall := int(riffleTicks * riffleFall)
	at := t % riffleTicks
	return t >= 0 && t < Ticks && at < fall && at%(fall/riffleCards) == 0
}

// Done returns how far through the shuffle is at tick t, from 0 to 1.
func Done(t int) float64 {
	return min(max(float64(t)/Ticks, 0), 1)
}

// Draw draws the shuffle as it stands at tick t, with its middle at
// (cx, cy) and the cards at the given scale: a riffle, done several times
// over. The cards lie in two halves, and fall from each in turn onto a pile
// between them, turning as they go. The pile is then cut in two again for
// the next riffle.
func Draw(dst *ebiten.Image, t int, cx, cy, scale float64) {
	const (
		apart = 120 // how far each half lies from the middle
		lift  = 26  // how high a card rises as it falls across
		turn  = 0.7 // how far a card is turned as it leaves its half
	)
	riffle := min(t/riffleTicks, riffles-1)
	at := float64(t-riffle*riffleTicks) / riffleTicks // how far through this riffle

	// pile draws a few cards lying squared up, with their middle at x.
	pile := func(x float64, n int) {
		for i := range n {
			ui.DrawCardBackTurned(dst, x-float64(i), cy-float64(i), scale, 0)
		}
	}

	// After the cards have fallen the pile is cut and the halves drawn
	// apart, ready for the next riffle. The last riffle leaves it whole.
	if at >= riffleFall {
		out := 0.0
		if riffle < riffles-1 {
			out = (at - riffleFall) / (1 - riffleFall) * apart
		}
		pile(cx-out, 3)
		pile(cx+out, 3)
		return
	}

	// The halves, the pile between them, and the cards on their way: each
	// sets off a little after the one before, from each half in turn.
	pile(cx-apart, 3)
	pile(cx+apart, 3)
	const span = 0.25 // the part of the fall one card takes
	for i := range riffleCards {
		start := float64(i) * (riffleFall - riffleFall*span) / (riffleCards - 1)
		u := (at - start) / (riffleFall * span)
		if u <= 0 {
			continue // still in its half
		}
		side := float64(1 - 2*(i%2)) // from the right, then the left
		if u >= 1 {
			// Landed: it lies on the pile, a little off square.
			ui.DrawCardBackTurned(dst, cx, cy-float64(i), scale, side*0.04)
			continue
		}
		x := cx + side*apart*(1-u)
		y := cy - float64(i) - lift*4*u*(1-u)
		ui.DrawCardBackTurned(dst, x, y, scale, side*turn*(1-u))
	}
}
