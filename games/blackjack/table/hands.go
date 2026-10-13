// Package table holds the Blackjack rules: the shoe, the hands, the dealer,
// what is paid, the Perfect Pairs side bet and the odds. The cards come from
// the cards package and the credits from the bank. It has no graphics so it
// can be tested on its own.
package table

import "videocasino/cards"

const (
	// Decks is the number of decks in the shoe. Hands are dealt from it one
	// after another until the cut card comes out, and then it is shuffled.
	Decks    = 6
	ShoeSize = Decks * cards.DeckSize

	// DealerStands is the total the dealer draws to. The dealer stands on
	// any 17, soft or hard.
	DealerStands = 17

	// A hand can be doubled only on a hard total from DoubleFrom to
	// DoubleTo.
	DoubleFrom = 9
	DoubleTo   = 11
)

// Value is what a card counts for: a ten or a picture is 10, and an ace is
// 1. Total counts an ace as 11 where that helps.
func Value(c cards.Card) int {
	switch r := c.Rank(); {
	case r == cards.Ace:
		return 1
	case r >= cards.Ten:
		return 10
	default:
		return int(r)
	}
}

// Total is what a hand adds up to, counting one ace as 11 if that does not
// take it past 21. It reports whether an ace is counted so: the hand is
// then soft, and cannot go bust on the next card.
func Total(hand []cards.Card) (total int, soft bool) {
	ace := false
	for _, c := range hand {
		v := Value(c)
		total += v
		ace = ace || v == 1
	}
	if ace && total+10 <= 21 {
		return total + 10, true
	}
	return total, false
}

// CanDoubleOn reports whether the rules let a hand be doubled: it is the
// first two cards, and they make a hard 9, 10 or 11. A hand with an ace in
// it is soft, and cannot be: an ace and an eight is a soft 19, not a 9.
func CanDoubleOn(hand []cards.Card) bool {
	total, soft := Total(hand)
	return len(hand) == 2 && !soft && total >= DoubleFrom && total <= DoubleTo
}

// IsBlackjack reports whether a hand is 21 in two cards. For the player
// that only counts on the two cards first dealt: after a split it is an
// ordinary 21.
func IsBlackjack(hand []cards.Card) bool {
	total, _ := Total(hand)
	return len(hand) == 2 && total == 21
}
