package table

import (
	"videocasino/cards"
	"videocasino/pokerhand"
)

// The odds of the game, worked out from the rules rather than by playing:
// a hand is set against every hand the dealer can hold with the card that
// is face up.

// dealerHands is how many hands the dealer can hold once the player's five
// cards and the dealer's face-up card are known: the other four of the
// dealer's, from the 46 cards left.
const dealerHands = 46 * 45 * 44 * 43 / (4 * 3 * 2)

// Advice is what raising a hand is worth.
type Advice struct {
	// Raise is what the player wins or loses on average by raising, for
	// each credit of the ante. Folding always loses 1.
	Raise float64
	// Best reports whether raising is better than folding.
	Best bool
}

// raiseNet returns what raising wins or loses, in antes, against one hand
// of the dealer's: the ante and the raise were staked.
func raiseNet(player, dealer pokerhand.Score) int {
	_, paid := Settle(player, dealer)
	return paid - (1 + RaiseSize)
}

// raiseTotal adds up what raising a hand wins or loses, in antes, over
// every hand the dealer can hold with the given card face up.
func raiseTotal(h Hand, up cards.Card) int {
	var out [cards.DeckSize]bool
	for _, c := range h {
		out[c] = true
	}
	out[up] = true
	rest := make([]cards.Card, 0, cards.DeckSize-HandSize-1)
	for c := range cards.Card(cards.DeckSize) {
		if !out[c] {
			rest = append(rest, c)
		}
	}

	player := h.Score()
	dealer := Hand{HandSize - 1: up}
	net := 0
	for a := range rest {
		dealer[0] = rest[a]
		for b := a + 1; b < len(rest); b++ {
			dealer[1] = rest[b]
			for c := b + 1; c < len(rest); c++ {
				dealer[2] = rest[c]
				for d := c + 1; d < len(rest); d++ {
					dealer[3] = rest[d]
					net += raiseNet(player, dealer.Score())
				}
			}
		}
	}
	return net
}

// Advise weighs up raising a hand against folding it, with the dealer's
// card that is face up. It is exact: every hand the dealer can be dealt
// from the rest of the deck is counted.
func Advise(h Hand, up cards.Card) Advice {
	net := raiseTotal(h, up)
	return Advice{
		Raise: float64(net) / dealerHands,
		Best:  net > -dealerHands,
	}
}

// What the game pays back to a player who raises every hand that is worth
// more raised than folded. Every hand the player can be dealt is set
// against every card the dealer can show, and every hand the dealer can
// then hold: about 2 x 10^13 deals in all, which takes most of a minute to
// count, so it is counted by TestReturn only when asked:
//
//	go test ./games/caribbean/table -run TestReturn -long -timeout 0
//
// and the totals it gives are kept here, whole, so the figures are exact.
const (
	// playerHands is how many hands of five the player can be dealt, and
	// upCards how many cards the dealer can then show.
	playerHands = 2_598_960
	upCards     = cards.DeckSize - HandSize

	// deals is how many ways a hand can be dealt in all.
	deals = playerHands * upCards * dealerHands

	// returnNet is what the player wins over every deal, in antes: it is
	// less than nothing. returnStaked is what is staked over them, the
	// raises included.
	returnNet    = -1_041_372_912_372
	returnStaked = 40_753_679_602_680

	// raisedHands is how many of the player's hands, each with each card
	// the dealer could show, are worth raising.
	raisedHands = 63_794_004
)

// Edge returns what the player loses on average for each credit of the
// ante.
func Edge() float64 {
	return -float64(returnNet) / deals
}

// Return returns the fraction of what is staked on the ante and the raise
// together that is paid back over a long run.
func Return() float64 {
	return float64(returnStaked+returnNet) / float64(returnStaked)
}

// Raised returns the fraction of hands that are worth raising.
func Raised() float64 {
	return float64(raisedHands) / (playerHands * upCards)
}
