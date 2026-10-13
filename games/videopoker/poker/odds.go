package poker

import (
	"cmp"
	"math/bits"
	"slices"
	"sync"
	"videocasino/cards"
)

// The long-run odds of the game, worked out from the paytable and the rules
// rather than by playing. Video poker's return depends on how well it is
// played, so the figures here are for perfect play: for every hand that can
// be dealt, every way of holding it is weighed up and the best one taken.
//
// Weighing up one way of holding a hand means looking at every draw that
// could follow, up to a million and a half of them. Doing that afresh for
// each of the 2,598,960 hands would take hours. Instead every hand is
// looked at once and counted against each set of cards within it, which
// gives, for any set of up to five cards, what all the hands containing it
// pay between them. What a hold is worth then comes from a few of those
// totals.

// choose[n][k] is the number of ways of choosing k cards from n.
var choose = func() (c [cards.DeckSize + 1][HandSize + 1]int) {
	for n := range c {
		c[n][0] = 1
		for k := 1; k <= HandSize && k <= n; k++ {
			c[n][k] = c[n-1][k-1] + c[n-1][k]
		}
	}
	return c
}()

const (
	numHolds = 1 << HandSize // the ways of holding a hand
	holdAll  = numHolds - 1
)

// sets describes the 32 sets of cards that can be picked out of one hand.
// A set is numbered by its cards: bit i is set if it has the hand's card i.
// The hand must be in ascending order.
type sets struct {
	size [numHolds]int // how many cards are in the set

	// index tells the set apart from every other set of its size in the
	// deck, counting from zero with no gaps.
	index [numHolds]int
}

func setsOf(hand [HandSize]cards.Card) (s sets) {
	for set := 1; set < numHolds; set++ {
		// The set is a smaller one with its highest card added.
		top := bits.Len(uint(set)) - 1
		rest := set &^ (1 << top)
		s.size[set] = s.size[rest] + 1
		s.index[set] = s.index[rest] + choose[hand[top]][s.size[set]]
	}
	return s
}

// eachHand calls f with every hand that can be dealt, its cards in
// ascending order.
func eachHand(f func([HandSize]cards.Card)) {
	var h [HandSize]cards.Card
	for h[4] = 4; h[4] < cards.DeckSize; h[4]++ {
		for h[3] = 3; h[3] < h[4]; h[3]++ {
			for h[2] = 2; h[2] < h[3]; h[2]++ {
				for h[1] = 1; h[1] < h[2]; h[1]++ {
					for h[0] = 0; h[0] < h[1]; h[0]++ {
						f(h)
					}
				}
			}
		}
	}
}

// tally counts the hands that can be made from each set of cards. It
// depends only on the rules of poker, not on what the hands pay.
type tally struct {
	// with[k][i] is how many of the hands containing set i of size k make
	// each category, for sets of fewer than five cards. Hands that make
	// nothing are left out.
	with [HandSize][][NumCategories]int32

	// made[i] is what five-card set i makes.
	made []uint8
}

// tallied is worked out the first time it is needed, which takes a moment.
var tallied = sync.OnceValue(func() *tally {
	t := &tally{made: make([]uint8, choose[cards.DeckSize][HandSize])}
	for k := range t.with {
		t.with[k] = make([][NumCategories]int32, choose[cards.DeckSize][k])
	}
	eachHand(func(hand [HandSize]cards.Card) {
		s := setsOf(hand)
		made := Evaluate(hand)
		t.made[s.index[holdAll]] = uint8(made)
		if made == Nothing {
			return
		}
		for set := range holdAll {
			t.with[s.size[set]][s.index[set]][made]++
		}
	})
	return t
})

// Odds is the odds of one paytable at one bet. The bet matters because a
// royal flush pays more for each coin when the most coins are bet.
type Odds struct {
	coins int

	// worth[k][i] is what all the hands containing set i of size k pay
	// between them, in coins.
	worth [HandSize + 1][]int32
}

// NewOdds works out the odds of a paytable when the given number of coins
// is bet on each hand.
func NewOdds(p *Paytable, coins int) *Odds {
	t := tallied()
	o := &Odds{coins: coins}
	var pay [NumCategories]int
	for c := range pay {
		pay[c] = p.Pay(Category(c), coins)
	}
	for k := range t.with {
		o.worth[k] = make([]int32, len(t.with[k]))
		for i := range t.with[k] {
			total := 0
			for c, n := range t.with[k][i] {
				total += int(n) * pay[c]
			}
			o.worth[k][i] = int32(total)
		}
	}
	o.worth[HandSize] = make([]int32, len(t.made))
	for i, c := range t.made {
		o.worth[HandSize][i] = int32(pay[c])
	}
	return o
}

// draws is how many different draws can follow a hold of the given number
// of cards.
func draws(held int) int {
	return choose[cards.DeckSize-HandSize][HandSize-held]
}

// best returns the best way to hold a hand, given its sets, and what that
// is worth: the average payout in coins over every draw that can follow.
func (o *Odds) best(s *sets) (hold int, value float64) {
	// To begin with, total[set] is what every hand containing the set
	// pays. That counts hands with the other cards of this hand in them,
	// and those cards will have been thrown away. Taking the hands with
	// each of them out in turn leaves only what the draws can make.
	var total [numHolds]int
	for set := range total {
		total[set] = int(o.worth[s.size[set]][s.index[set]])
	}
	for i := range HandSize {
		card := 1 << i
		for set := range total {
			if set&card == 0 {
				total[set] -= total[set|card]
			}
		}
	}

	// Going down from holding everything means that, of two holds worth
	// the same, the one keeping more cards is chosen.
	value = -1
	for set := holdAll; set >= 0; set-- {
		if v := float64(total[set]) / float64(draws(s.size[set])); v > value {
			hold, value = set, v
		}
	}
	return hold, value
}

// Best returns the cards to hold to make the most of a hand, and what the
// hand is then worth: the average payout in coins.
func (o *Odds) Best(hand [HandSize]cards.Card) (held [HandSize]bool, value float64) {
	// The sets want the cards in order, so sort them and remember where
	// each one came from.
	order := [HandSize]int{0, 1, 2, 3, 4}
	slices.SortFunc(order[:], func(a, b int) int { return cmp.Compare(hand[a], hand[b]) })
	var sorted [HandSize]cards.Card
	for i, at := range order {
		sorted[i] = hand[at]
	}
	s := setsOf(sorted)
	hold, value := o.best(&s)
	for i, at := range order {
		held[at] = hold>>i&1 == 1
	}
	return held, value
}

// Return is the fraction of what is bet that is paid back over the long
// run to a player who always makes the best hold.
func (o *Odds) Return() float64 {
	total := 0.0
	eachHand(func(hand [HandSize]cards.Card) {
		s := setsOf(hand)
		_, value := o.best(&s)
		total += value
	})
	return total / float64(choose[cards.DeckSize][HandSize]*o.coins)
}

// Frequencies is how often a hand ends up as each category, after the
// draw, for a player who always makes the best hold.
func (o *Odds) Frequencies() (freq [NumCategories]float64) {
	t := tallied()
	eachHand(func(hand [HandSize]cards.Card) {
		s := setsOf(hand)
		hold, _ := o.best(&s)

		// Count what the draws make, as best does for what they pay, but
		// for this one hold: only the sets containing it come into it.
		// Each card that the set has and the hold has not swaps the sign.
		var n [NumCategories]int
		for set := hold; ; set = (set + 1) | hold {
			sign := 1
			if (s.size[set]-s.size[hold])%2 == 1 {
				sign = -1
			}
			if set == holdAll {
				n[t.made[s.index[set]]] += sign
				break
			}
			for c, count := range t.with[s.size[set]][s.index[set]] {
				n[c] += sign * int(count)
			}
		}
		for c := JacksOrBetter; c < NumCategories; c++ {
			freq[c] += float64(n[c]) / float64(draws(s.size[hold]))
		}
	})

	freq[Nothing] = 1
	for c := JacksOrBetter; c < NumCategories; c++ {
		freq[c] /= float64(choose[cards.DeckSize][HandSize])
		freq[Nothing] -= freq[c]
	}
	return freq
}
