// Package poker holds the video poker rules: the hands, the paytable, the
// bets and the odds. The cards come from the cards package and the credits
// from the bank. It has no graphics so it can be tested on its own.
package poker

import (
	"math/bits"

	"videocasino/cards"
)

// HandSize is the number of cards in a hand.
const HandSize = 5

// Category is what a hand of five cards makes. They run from the lowest
// paying to the highest.
type Category int

const (
	Nothing Category = iota
	JacksOrBetter
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
	RoyalFlush
	NumCategories
)

func (c Category) String() string {
	return [...]string{
		"Nothing", "Jacks or Better", "Two Pair", "Three of a Kind",
		"Straight", "Flush", "Full House", "Four of a Kind",
		"Straight Flush", "Royal Flush",
	}[c]
}

// Sets of ranks, one bit for each, with the two at bit 0.
const (
	wheelRanks = 1<<(cards.Ace-cards.Two) | 0b1111 // ace to five, the one straight with a low ace
	royalRanks = 0b11111 << (cards.Ten - cards.Two)
	highRanks  = 1 << (cards.Jack - cards.Two) // a pair of these or better pays
)

// Evaluate returns what a hand makes. A pair lower than jacks makes
// nothing.
func Evaluate(hand [HandSize]cards.Card) Category {
	var count [cards.NumRanks]int
	var ranks, pairs uint // the ranks in the hand, and those it has twice
	most := 0             // the most cards of one rank
	flush := true
	for _, c := range hand {
		r := c / cards.NumSuits
		count[r]++
		ranks |= 1 << r
		if count[r] == 2 {
			pairs |= 1 << r
		}
		most = max(most, count[r])
		if c.Suit() != hand[0].Suit() {
			flush = false
		}
	}

	switch bits.OnesCount(ranks) {
	case 2:
		if most == 4 {
			return FourOfAKind
		}
		return FullHouse
	case 3:
		if most == 3 {
			return ThreeOfAKind
		}
		return TwoPair
	case 4:
		if pairs >= highRanks {
			return JacksOrBetter
		}
		return Nothing
	}

	// Five different ranks make a straight if they are next to each other.
	straight := ranks == wheelRanks || ranks>>bits.TrailingZeros(ranks) == 0b11111
	switch {
	case flush && ranks == royalRanks:
		return RoyalFlush
	case flush && straight:
		return StraightFlush
	case flush:
		return Flush
	case straight:
		return Straight
	}
	return Nothing
}
