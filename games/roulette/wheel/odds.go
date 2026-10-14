package wheel

// The long-run odds of the game, worked out from the wheel and what each bet
// pays rather than by playing.

// Return is the fraction of what is bet on the spot that it pays back over
// a long run: each of the wheel's pockets comes up as often as any other,
// and the bet pays on the ones it covers.
func (s Spot) Return() float64 {
	return float64(len(s.Numbers)*(s.Pays()+1)) / Pockets
}

// Return is the fraction of what is bet that the game pays back over a long
// run. Every bet on the table pays back the same, so it does not depend on
// how the game is played; the tests check that they do.
func Return() float64 {
	return Spots[0].Return()
}
