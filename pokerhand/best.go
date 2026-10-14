package pokerhand

import (
	"slices"

	"videocasino/cards"
)

// Best returns the best hand of five that can be made from the cards given,
// of which there must be five to seven, and its score. The five are given
// in the order they lie among the cards.
func Best(cs []cards.Card) (best Hand, top Score) {
	n := len(cs)
	top = -1
	// Every way of leaving out all but five of them.
	for a := 0; a < n; a++ {
		for b := a + 1; b < n; b++ {
			for c := b + 1; c < n; c++ {
				for d := c + 1; d < n; d++ {
					for e := d + 1; e < n; e++ {
						h := Hand{cs[a], cs[b], cs[c], cs[d], cs[e]}
						if s := h.Score(); s > top {
							best, top = h, s
						}
					}
				}
			}
		}
	}
	return best, top
}

// Table holds the score of every hand of five, for counting a great many
// hands quickly. A hand is found in it by Index.
type Table []int32

// binom holds how many ways k things can be taken from n, for the numbers
// a hand of five needs.
var binom = func() (b [cards.DeckSize + 1][Size + 1]int) {
	for n := range b {
		b[n][0] = 1
		for k := 1; k <= Size && k <= n; k++ {
			b[n][k] = b[n-1][k-1] + b[n-1][k]
		}
	}
	return b
}()

// Hands is how many hands of five there are.
var Hands = binom[cards.DeckSize][Size]

// Index returns the place of a hand of five in a Table. The cards must be
// in order, lowest first: a < b < c < d < e is at C(a,1) + C(b,2) +
// C(c,3) + C(d,4) + C(e,5).
func Index(h Hand) int {
	return binom[h[0]][1] + binom[h[1]][2] + binom[h[2]][3] + binom[h[3]][4] + binom[h[4]][5]
}

// NewTable works out the score of every hand of five.
func NewTable() Table {
	t := make(Table, Hands)
	for e := range cards.Card(cards.DeckSize) {
		for d := range e {
			for c := range d {
				for b := range c {
					for a := range b {
						h := Hand{a, b, c, d, e}
						t[Index(h)] = int32(h.Score())
					}
				}
			}
		}
	}
	return t
}

// Score returns the score of a hand of five, in any order.
func (t Table) Score(h Hand) Score {
	slices.Sort(h[:])
	return Score(t[Index(h)])
}

// Best returns the score of the best hand of five that can be made from
// the cards given, of which there must be five to seven: what Best
// returns, found more quickly.
func (t Table) Best(cs []cards.Card) Score {
	sorted := slices.Clone(cs)
	slices.Sort(sorted)
	n := len(sorted)
	top := int32(-1)
	for a := 0; a < n; a++ {
		for b := a + 1; b < n; b++ {
			for c := b + 1; c < n; c++ {
				for d := c + 1; d < n; d++ {
					for e := d + 1; e < n; e++ {
						i := binom[sorted[a]][1] + binom[sorted[b]][2] + binom[sorted[c]][3] + binom[sorted[d]][4] + binom[sorted[e]][5]
						top = max(top, t[i])
					}
				}
			}
		}
	}
	return Score(top)
}
