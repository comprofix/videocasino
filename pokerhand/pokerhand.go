// Package pokerhand ranks hands of five cards as poker does, fully: not
// only what a hand makes, but which of two hands that make the same thing
// is the better, by its ranks and kickers. It is for any card game played
// against the dealer's hand. It has no graphics so it can be tested on its
// own.
package pokerhand

import (
	"slices"

	"videocasino/cards"
)

// Size is the number of cards in a hand.
const Size = 5

// Hand is a hand of five cards.
type Hand [Size]cards.Card

// Category is what a hand makes, from the lowest to the highest. A royal
// flush is a straight flush to the ace, given a category of its own as
// games pay it apart.
type Category int

const (
	HighCard Category = iota
	Pair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
	RoyalFlush
	Categories // how many there are
)

func (c Category) String() string {
	return [...]string{
		"HIGH CARD", "PAIR", "TWO PAIR", "THREE OF A KIND", "STRAIGHT",
		"FLUSH", "FULL HOUSE", "FOUR OF A KIND", "STRAIGHT FLUSH", "ROYAL FLUSH",
	}[c]
}

// Score is how good a hand is: the better of two hands has the higher, and
// two hands that tie have the same. It holds the category and then, four
// bits each, the ranks that settle hands within it, those that count first
// first.
type Score int

const (
	rankBits  = 4
	rankSlots = Size // ranks held after the category
)

func score(c Category, ranks ...cards.Rank) Score {
	s := int(c)
	for i := range rankSlots {
		s <<= rankBits
		if i < len(ranks) {
			s |= int(ranks[i])
		}
	}
	return Score(s)
}

// Category returns what a hand with this score makes.
func (s Score) Category() Category {
	return Category(int(s) >> (rankBits * rankSlots))
}

// Rank returns the nth rank that settles a hand with this score, from 0:
// the rank of the pair, the three or the four first, the top of a straight,
// or the highest card.
func (s Score) Rank(n int) cards.Rank {
	return cards.Rank(int(s) >> (rankBits * (rankSlots - 1 - n)) & (1<<rankBits - 1))
}

// Score works out how good the hand is.
func (h Hand) Score() Score {
	// How many of each rank there are, and whether the suits all match.
	var count [cards.Ace + 1]int
	flush := true
	for _, c := range h {
		count[c.Rank()]++
		if c.Suit() != h[0].Suit() {
			flush = false
		}
	}

	// The ranks in the order they count: the most of a kind first, and the
	// higher first among those as many.
	var held [Size]cards.Rank
	ranks := held[:0]
	for n := 4; n >= 1; n-- {
		for r := cards.Ace; r >= cards.Two; r-- {
			if count[r] == n {
				ranks = append(ranks, r)
			}
		}
	}

	// Five ranks in a row make a straight. The ace is high, but also makes
	// the lowest straight, ace to five: that one runs only to the five.
	straight := false
	if len(ranks) == Size {
		top := ranks[0]
		switch {
		case ranks[0]-ranks[4] == 4:
			straight = true
		case slices.Equal(ranks, []cards.Rank{cards.Ace, cards.Five, cards.Four, cards.Three, cards.Two}):
			straight, top = true, cards.Five
		}
		switch {
		case straight && flush && top == cards.Ace:
			return score(RoyalFlush, top)
		case straight && flush:
			return score(StraightFlush, top)
		case flush:
			return score(Flush, ranks...)
		case straight:
			return score(Straight, top)
		}
		return score(HighCard, ranks...)
	}

	switch first, second := count[ranks[0]], count[ranks[1]]; {
	case first == 4:
		return score(FourOfAKind, ranks...)
	case first == 3 && second == 2:
		return score(FullHouse, ranks...)
	case first == 3:
		return score(ThreeOfAKind, ranks...)
	case first == 2 && second == 2:
		return score(TwoPair, ranks...)
	}
	return score(Pair, ranks...)
}

// Category returns what the hand makes.
func (h Hand) Category() Category {
	return h.Score().Category()
}

// Least returns a score that every hand making the given category, with
// the given ranks counting first, reaches, and that no lesser hand does:
// every hand of an ace and a king scores at least Least(HighCard, Ace,
// King), and every hand of an ace and a queen less. It is not itself the
// score of a hand, as the ranks not given count as nothing.
func Least(c Category, ranks ...cards.Rank) Score {
	return score(c, ranks...)
}

var (
	rankNames = [...]string{"TWO", "THREE", "FOUR", "FIVE", "SIX", "SEVEN", "EIGHT", "NINE", "TEN", "JACK", "QUEEN", "KING", "ACE"}
	manyNames = [...]string{"TWOS", "THREES", "FOURS", "FIVES", "SIXES", "SEVENS", "EIGHTS", "NINES", "TENS", "JACKS", "QUEENS", "KINGS", "ACES"}
)

// String names the hand a score stands for, such as "PAIR OF SEVENS" or
// "ACE KING HIGH". A hand that makes nothing is named by its two highest
// cards, as those are what count against a dealer who needs an ace and a
// king.
func (s Score) String() string {
	one := func(n int) string { return rankNames[s.Rank(n)-cards.Two] }
	many := func(n int) string { return manyNames[s.Rank(n)-cards.Two] }
	switch s.Category() {
	case RoyalFlush:
		return "ROYAL FLUSH"
	case StraightFlush:
		return "STRAIGHT FLUSH TO THE " + one(0)
	case FourOfAKind:
		return "FOUR " + many(0)
	case FullHouse:
		return many(0) + " FULL OF " + many(1)
	case Flush:
		return "FLUSH, " + one(0) + " HIGH"
	case Straight:
		return "STRAIGHT TO THE " + one(0)
	case ThreeOfAKind:
		return "THREE " + many(0)
	case TwoPair:
		return "TWO PAIR, " + many(0) + " AND " + many(1)
	case Pair:
		return "PAIR OF " + many(0)
	}
	return one(0) + " " + one(1) + " HIGH"
}
