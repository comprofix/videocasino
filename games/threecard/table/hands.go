// Package table holds the Three Card Poker rules: how hands of three cards
// rank, the bets and what they pay, and the odds. The credits come from the
// bank. It has no graphics so it can be tested on its own.
package table

import (
	"slices"

	"videocasino/cards"
)

// HandSize is the number of cards in a hand.
const HandSize = 3

// Hand is a hand of three cards.
type Hand [HandSize]cards.Card

// Category is what a hand makes, from the lowest to the highest. A straight
// beats a flush: with three cards it is the harder to make.
type Category int

const (
	HighCard Category = iota
	Pair
	Flush
	Straight
	ThreeOfAKind
	StraightFlush
	Categories // how many there are
)

func (c Category) String() string {
	return [...]string{"HIGH CARD", "PAIR", "FLUSH", "STRAIGHT", "THREE OF A KIND", "STRAIGHT FLUSH"}[c]
}

// Score is how good a hand is: the better of two hands has the higher, and
// two hands that tie have the same.
type Score int

// scoreBase is one more than the highest rank, so that a category and the
// ranks that settle hands within it can be packed into a score.
const scoreBase = int(cards.Ace) + 1

func score(c Category, a, b, d cards.Rank) Score {
	return Score(((int(c)*scoreBase+int(a))*scoreBase+int(b))*scoreBase + int(d))
}

// Category returns what a hand with this score makes.
func (s Score) Category() Category {
	return Category(int(s) / (scoreBase * scoreBase * scoreBase))
}

// Rank returns the rank that counts first for a hand with this score: the
// rank of a pair or of three of a kind, the top of a straight, or the
// highest card.
func (s Score) Rank() cards.Rank {
	return cards.Rank(int(s) / (scoreBase * scoreBase) % scoreBase)
}

// Score works out how good the hand is.
func (h Hand) Score() Score {
	r := [HandSize]cards.Rank{h[0].Rank(), h[1].Rank(), h[2].Rank()}
	slices.Sort(r[:])
	lo, mid, hi := r[0], r[1], r[2]

	flush := h[0].Suit() == h[1].Suit() && h[1].Suit() == h[2].Suit()
	// The ace is high, but also makes the lowest straight, with a two and a
	// three: that one runs only to the three.
	straight, top := hi-mid == 1 && mid-lo == 1, hi
	if lo == cards.Two && mid == cards.Three && hi == cards.Ace {
		straight, top = true, cards.Three
	}

	switch {
	case straight && flush:
		return score(StraightFlush, top, 0, 0)
	case lo == hi:
		return score(ThreeOfAKind, hi, 0, 0)
	case straight:
		return score(Straight, top, 0, 0)
	case flush:
		return score(Flush, hi, mid, lo)
	case lo == mid:
		return score(Pair, lo, hi, 0)
	case mid == hi:
		return score(Pair, hi, lo, 0)
	}
	return score(HighCard, hi, mid, lo)
}

// Category returns what the hand makes.
func (h Hand) Category() Category {
	return h.Score().Category()
}

// DealerNeeds is the rank the dealer's highest card must reach for a hand
// that makes nothing to qualify.
const DealerNeeds = cards.Queen

// Qualifies reports whether a dealer's hand with this score is good enough
// to play: a queen high or better.
func (s Score) Qualifies() bool {
	// Every hand that makes something scores more than any that does not.
	return s >= score(HighCard, DealerNeeds, 0, 0)
}

var (
	rankNames = [...]string{"TWO", "THREE", "FOUR", "FIVE", "SIX", "SEVEN", "EIGHT", "NINE", "TEN", "JACK", "QUEEN", "KING", "ACE"}
	manyNames = [...]string{"TWOS", "THREES", "FOURS", "FIVES", "SIXES", "SEVENS", "EIGHTS", "NINES", "TENS", "JACKS", "QUEENS", "KINGS", "ACES"}
)

// String names the hand a score stands for, such as "PAIR OF SEVENS".
func (s Score) String() string {
	one, many := rankNames[s.Rank()-cards.Two], manyNames[s.Rank()-cards.Two]
	switch s.Category() {
	case StraightFlush:
		return "STRAIGHT FLUSH TO THE " + one
	case ThreeOfAKind:
		return "THREE " + many
	case Straight:
		return "STRAIGHT TO THE " + one
	case Flush:
		return "FLUSH, " + one + " HIGH"
	case Pair:
		return "PAIR OF " + many
	}
	return one + " HIGH"
}

// AnteBonus is what the ante is paid for each credit bet, on top of
// whatever else it wins or loses, when the player plays a hand of each
// category. It is paid whatever the dealer holds.
var AnteBonus = [Categories]int{
	Straight:      1,
	ThreeOfAKind:  4,
	StraightFlush: 5,
}

// PairPlusPays is what the Pair Plus bet pays for each credit bet, on top
// of the bet itself, on a hand of each category. It is paid on the player's
// own three cards, whatever the dealer holds.
var PairPlusPays = [Categories]int{
	Pair:          1,
	Flush:         4,
	Straight:      6,
	ThreeOfAKind:  30,
	StraightFlush: 40,
}

// Outcome is how the ante and the play bet went.
type Outcome int

const (
	Folded       Outcome = iota // the player gave up the ante
	Lost                        // the dealer qualified and had the better hand
	Pushed                      // the dealer qualified and the hands tied
	Won                         // the dealer qualified and the player's hand was better
	NotQualified                // the dealer did not qualify
)

// Settle returns how a hand that was played went against the dealer's, and
// what the ante and the play bet together are paid for each credit of the
// ante, themselves included and with any ante bonus.
//
// If the dealer does not qualify the ante wins and the play bet is handed
// back. If the dealer does, both bets win or lose together.
func Settle(player, dealer Score) (o Outcome, paid int) {
	paid = AnteBonus[player.Category()]
	switch {
	case !dealer.Qualifies():
		return NotQualified, paid + 3
	case player > dealer:
		return Won, paid + 4
	case player == dealer:
		return Pushed, paid + 2
	}
	return Lost, paid
}
