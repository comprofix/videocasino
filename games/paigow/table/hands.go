// Package table holds the Pai Gow Poker rules: the deck with its joker,
// how high and low hands rank, the house way the dealer sets a hand by,
// and the bet. Five-card hands are ranked by the pokerhand package, and
// the credits come from the bank. It has no graphics so it can be tested
// on its own.
package table

import (
	"slices"

	"videocasino/cards"
	"videocasino/pokerhand"
)

// The deck is the 52 cards and a joker, which follows them.
const (
	Joker    cards.Card = cards.DeckSize
	DeckSize            = cards.DeckSize + 1
)

// The hands: seven cards each, set as a high hand of five and a low hand
// of two.
const (
	HandSize = 7
	HighSize = 5
	LowSize  = 2
)

// Valid reports whether c is one of the 53 cards.
func Valid(c cards.Card) bool {
	return c >= 0 && c <= Joker
}

// rankOf returns the rank a card counts as for pairs and the like: the
// joker counts as an ace.
func rankOf(c cards.Card) cards.Rank {
	if c == Joker {
		return cards.Ace
	}
	return c.Rank()
}

// Name names a card in two letters, such as "Td", or "Jk" for the joker.
func Name(c cards.Card) string {
	if c == Joker {
		return "Jk"
	}
	return c.String()
}

// High is how good a high hand is: the better of two has the higher.
// Below five aces it is the pokerhand score of the hand, with the joker
// counted as it does best.
type High int32

// FiveAces is four aces and the joker, the best high hand of all.
const FiveAces = High(int32(pokerhand.Categories) << 20)

// String names a high hand, such as "PAIR OF SEVENS".
func (h High) String() string {
	if h == FiveAces {
		return "FIVE ACES"
	}
	return pokerhand.Score(h).String()
}

// Category returns what a high hand makes. Five aces makes more than a
// royal flush, as pokerhand.Categories.
func (h High) Category() pokerhand.Category {
	if h == FiveAces {
		return pokerhand.Categories
	}
	return pokerhand.Score(h).Category()
}

// Ranker ranks high hands. Its zero value ranks each hand on its own; one
// made with a score table ranks a great many quickly, for counting.
type Ranker struct {
	t pokerhand.Table
}

// NewRanker returns a ranker with a score table, which takes a moment to
// make.
func NewRanker() Ranker {
	return Ranker{t: pokerhand.NewTable()}
}

func (r Ranker) five(h pokerhand.Hand) pokerhand.Score {
	if r.t != nil {
		return r.t.Score(h)
	}
	return h.Score()
}

// High scores a high hand. The joker counts as an ace, or as any card that
// makes a straight, a flush or a straight flush.
func (r Ranker) High(h [HighSize]cards.Card) High {
	j := slices.Index(h[:], Joker)
	if j < 0 {
		return High(r.five(pokerhand.Hand(h)))
	}
	var four [HighSize - 1]cards.Card
	n, aces := 0, 0
	for i, c := range h {
		if i != j {
			four[n] = c
			n++
			if c.Rank() == cards.Ace {
				aces++
			}
		}
	}
	if aces == 4 {
		return FiveAces
	}
	held := func(c cards.Card) bool { return slices.Contains(four[:], c) }
	with := func(c cards.Card) pokerhand.Score {
		return r.five(pokerhand.Hand{four[0], four[1], four[2], four[3], c})
	}

	// The suit of a flush the joker could complete, if the four share one.
	flush := cards.Suit(-1)
	if four[0].Suit() == four[1].Suit() && four[1].Suit() == four[2].Suit() && four[2].Suit() == four[3].Suit() {
		flush = four[0].Suit()
	}

	// As an ace, of a suit that makes no flush.
	best := pokerhand.Score(-1)
	for s := range cards.Suit(cards.NumSuits) {
		if ace := cards.New(cards.Ace, s); s != flush && !held(ace) {
			best = with(ace)
			break
		}
	}
	// As whatever makes a straight, a flush or a straight flush, and only
	// those: never a full house or four of a kind. For each rank a card of
	// the flush's suit is tried, if there is a flush to make, and one of
	// another suit, which is enough for a straight.
	completes := func(s pokerhand.Score) bool {
		switch s.Category() {
		case pokerhand.Straight, pokerhand.Flush, pokerhand.StraightFlush, pokerhand.RoyalFlush:
			return true
		}
		return false
	}
	for rank := cards.Two; rank <= cards.Ace; rank++ {
		if flush >= 0 {
			if c := cards.New(rank, flush); !held(c) && completes(with(c)) {
				best = max(best, with(c))
			}
		}
		for s := range cards.Suit(cards.NumSuits) {
			if c := cards.New(rank, s); s != flush && !held(c) {
				if completes(with(c)) {
					best = max(best, with(c))
				}
				break
			}
		}
	}
	return High(best)
}

// Low is how good a low hand of two is: a pair, or two cards that make
// nothing. The better of two has the higher.
type Low int

// LowOf scores a low hand. The joker counts as an ace.
func LowOf(a, b cards.Card) Low {
	hi, lo := rankOf(a), rankOf(b)
	if hi < lo {
		hi, lo = lo, hi
	}
	if hi == lo {
		return Low(1<<8 | int(hi))
	}
	return Low(int(hi)<<4 | int(lo))
}

// Pair reports whether a low hand is a pair.
func (l Low) Pair() bool {
	return l >= 1<<8
}

var (
	rankNames = [...]string{"TWO", "THREE", "FOUR", "FIVE", "SIX", "SEVEN", "EIGHT", "NINE", "TEN", "JACK", "QUEEN", "KING", "ACE"}
	manyNames = [...]string{"TWOS", "THREES", "FOURS", "FIVES", "SIXES", "SEVENS", "EIGHTS", "NINES", "TENS", "JACKS", "QUEENS", "KINGS", "ACES"}
)

// String names a low hand, such as "PAIR OF SEVENS" or "ACE QUEEN".
func (l Low) String() string {
	if l.Pair() {
		return "PAIR OF " + manyNames[int(l)&0xff-int(cards.Two)]
	}
	hi, lo := int(l)>>4, int(l)&0xf
	return rankNames[hi-int(cards.Two)] + " " + rankNames[lo-int(cards.Two)]
}

// Fouls reports whether a high hand fails to outrank the low hand set
// beside it, which is not allowed.
func Fouls(high High, low Low) bool {
	var least pokerhand.Score
	if low.Pair() {
		least = pokerhand.Least(pokerhand.Pair, cards.Rank(int(low)&0xff))
	} else {
		least = pokerhand.Least(pokerhand.HighCard, cards.Rank(int(low)>>4), cards.Rank(int(low)&0xf))
	}
	return high <= High(least)
}

// Outcome is how the bet went.
type Outcome int

const (
	Lost   Outcome = iota // both of the dealer's hands were at least as good
	Pushed                // one hand each
	Won                   // both of the player's hands were better
)

// Commission is the part of a win the house keeps, in hundredths.
const Commission = 5

// Settle returns how a bet went, and what it was paid, itself included,
// with what was kept as commission. Each of the player's hands must beat
// the dealer's to win it: a hand that ties, a copy, goes to the dealer.
func Settle(high High, low Low, dealerHigh High, dealerLow Low, bet int) (o Outcome, paid, commission int) {
	switch wins := b2i(high > dealerHigh) + b2i(low > dealerLow); wins {
	case 2:
		commission = bet * Commission / 100
		return Won, 2*bet - commission, commission
	case 1:
		return Pushed, bet, 0
	}
	return Lost, 0, 0
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Setting is a hand set as high and low: which two of the seven go low.
type Setting struct {
	Low  [LowSize]int // the places of the two in the hand, in order
	High High
	LowS Low
}

// Split returns the cards of a hand as set: the high hand and the low.
func Split(hand [HandSize]cards.Card, low [LowSize]int) (high [HighSize]cards.Card, lowCards [LowSize]cards.Card) {
	n := 0
	for i, c := range hand {
		if i == low[0] || i == low[1] {
			continue
		}
		high[n] = c
		n++
	}
	return high, [LowSize]cards.Card{hand[low[0]], hand[low[1]]}
}

// Set scores a hand set with the given two low. It reports false if those
// are not two different places in the hand, or the setting fouls.
func (r Ranker) Set(hand [HandSize]cards.Card, low [LowSize]int) (Setting, bool) {
	if low[0] > low[1] {
		low[0], low[1] = low[1], low[0]
	}
	if low[0] < 0 || low[1] >= HandSize || low[0] == low[1] {
		return Setting{}, false
	}
	high, lows := Split(hand, low)
	s := Setting{Low: low, High: r.High(high), LowS: LowOf(lows[0], lows[1])}
	return s, !Fouls(s.High, s.LowS)
}

// settings returns every way of setting a hand that does not foul.
func (r Ranker) settings(hand [HandSize]cards.Card) []Setting {
	all := make([]Setting, 0, 21)
	for i := range HandSize {
		for j := i + 1; j < HandSize; j++ {
			if s, ok := r.Set(hand, [LowSize]int{i, j}); ok {
				all = append(all, s)
			}
		}
	}
	return all
}
