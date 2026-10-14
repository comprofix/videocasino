package table

import (
	"videocasino/cards"
	"videocasino/pokerhand"
)

// The house way: how the dealer sets a hand, and how the HOUSE WAY button
// sets the player's. The rules, in the order they are tried:
//
//  1. No pair: the highest card high, the next two low.
//  2. One pair: the pair high, the two highest of the rest low.
//  3. Two pairs: split them, the lower pair low, if the higher is aces or
//     jacks to kings. If it is 7s to 10s, split unless an ace can be
//     played low; if both are 6s or lower, split unless an ace or a king
//     can. Not split, both stay high with the best two others low.
//  4. Three pairs: the highest pair low.
//  5. Three of a kind: high, with the two highest of the rest low; three
//     aces are split, a pair high and an ace low with the best other. With
//     two threes of a kind a pair of the higher goes low.
//  6. A straight, a flush or a straight flush is kept high, with no pair,
//     with one pair if the pair can go low (or else as one pair), and with
//     three of a kind, the best two others low. With two or three pairs
//     the pairs are played instead. A straight flush counts before a
//     flush, and a flush before a straight.
//  7. A full house is split, the pair low; with a second pair, the higher
//     pair goes low.
//  8. Four of a kind: 2s to 6s are kept high; 7s to 10s are split unless an
//     ace or a king can go low; jacks to kings are split unless an ace can;
//     aces are split unless a pair of 7s or better can go low. With a pair,
//     or three of a kind, a pair of those goes low and the four stay high.
//  9. Five aces: three high and a pair of aces low, unless a pair of kings
//     can go low.
//  10. Where a rule leaves a choice, the best low hand is taken, and then
//     the best high. A hand is never set to foul.
//
// The joker counts as an ace throughout, but where it makes a straight or
// a flush rule 6 sees it.

// HouseWay returns the places of the two cards the house way sets low.
func (r Ranker) HouseWay(hand [HandSize]cards.Card) [LowSize]int {
	all := r.settings(hand)

	var count [cards.Ace + 1]int
	for _, c := range hand {
		count[rankOf(c)]++
	}
	// The ranks there are four, three and two of, highest first, and the
	// cards on their own.
	var quads, trips, pairs []cards.Rank
	var single [cards.Ace + 1]bool
	for rank := cards.Ace; rank >= cards.Two; rank-- {
		switch count[rank] {
		case 4:
			quads = append(quads, rank)
		case 3:
			trips = append(trips, rank)
		case 2:
			pairs = append(pairs, rank)
		case 1:
			single[rank] = true
		}
	}

	// lowRanks returns the ranks of a setting's two low cards.
	lowRanks := func(s Setting) (cards.Rank, cards.Rank) {
		return rankOf(hand[s.Low[0]]), rankOf(hand[s.Low[1]])
	}
	lowPairOf := func(rank cards.Rank) func(Setting) bool {
		return func(s Setting) bool {
			a, b := lowRanks(s)
			return a == rank && b == rank
		}
	}
	// pick returns the best setting the rule allows: the best low hand,
	// then the best high. With none allowed it falls back on the best of
	// all, which no hand should come to.
	var pick func(allow func(Setting) bool) [LowSize]int
	pick = func(allow func(Setting) bool) [LowSize]int {
		best := -1
		for i, s := range all {
			if allow != nil && !allow(s) {
				continue
			}
			if best < 0 || s.LowS > all[best].LowS || (s.LowS == all[best].LowS && s.High > all[best].High) {
				best = i
			}
		}
		if best < 0 {
			if allow == nil {
				return [LowSize]int{0, 1} // there is always a setting; this is never reached
			}
			return pick(nil)
		}
		return all[best].Low
	}
	// pickStraight returns the best setting with a straight, a flush or a
	// straight flush high, that allow also allows: the best of those
	// first, then the best low hand, then the best high.
	pickStraight := func(allow func(Setting) bool) ([LowSize]int, bool) {
		best := -1
		for i, s := range all {
			cat := s.High.Category()
			if cat < pokerhand.Straight || cat > pokerhand.RoyalFlush || (allow != nil && !allow(s)) {
				continue
			}
			if best < 0 {
				best = i
				continue
			}
			b := all[best]
			bc := b.High.Category()
			if cat > bc || (cat == bc && (s.LowS > b.LowS || (s.LowS == b.LowS && s.High > b.High))) {
				best = i
			}
		}
		if best < 0 {
			return [LowSize]int{}, false
		}
		return all[best].Low, true
	}
	// lowAvoids allows the settings with neither low card of the given
	// ranks: they stay together in the high hand.
	lowAvoids := func(ranks ...cards.Rank) func(Setting) bool {
		return func(s Setting) bool {
			a, b := lowRanks(s)
			for _, rank := range ranks {
				if a == rank || b == rank {
					return false
				}
			}
			return true
		}
	}

	switch {
	// Rule 9: five aces.
	case count[cards.Ace] == 5:
		if count[cards.King] >= 2 {
			return pick(lowPairOf(cards.King))
		}
		return pick(lowPairOf(cards.Ace))

	// Rule 8: four of a kind.
	case len(quads) > 0:
		q := quads[0]
		other := cards.Rank(0) // the highest rank of a pair or three beside the four
		if len(trips) > 0 {
			other = trips[0]
		}
		if len(pairs) > 0 && pairs[0] > other {
			other = pairs[0]
		}
		switch {
		case q == cards.Ace && other >= cards.Seven:
			return pick(lowPairOf(other))
		case q == cards.Ace:
			return pick(lowPairOf(cards.Ace))
		case other != 0:
			return pick(lowPairOf(other))
		}
		keep := q <= cards.Six ||
			(q <= cards.Ten && (single[cards.Ace] || single[cards.King])) ||
			(q <= cards.King && single[cards.Ace])
		if keep {
			return pick(lowAvoids(q))
		}
		return pick(lowPairOf(q))

	// Rule 7: a full house; and rule 5 with two threes of a kind.
	case len(trips) > 0 && len(pairs) > 0:
		return pick(lowPairOf(pairs[0]))
	case len(trips) == 2:
		return pick(lowPairOf(trips[0]))

	// Rule 4: three pairs.
	case len(pairs) >= 3:
		return pick(lowPairOf(pairs[0]))

	// Rule 3: two pairs, whatever straight or flush there may be.
	case len(pairs) == 2:
		hi, lo := pairs[0], pairs[1]
		split := hi >= cards.Jack ||
			(hi >= cards.Seven && !single[cards.Ace]) ||
			(hi <= cards.Six && !single[cards.Ace] && !single[cards.King])
		if split {
			return pick(lowPairOf(lo))
		}
		return pick(lowAvoids(hi, lo))

	// Rules 6 and 5: three of a kind and nothing paired beside it.
	case len(trips) == 1:
		if low, ok := pickStraight(nil); ok {
			return low
		}
		if trips[0] == cards.Ace {
			// A pair of aces high, the third ace low with the best other.
			return pick(func(s Setting) bool {
				a, b := lowRanks(s)
				return (a == cards.Ace) != (b == cards.Ace)
			})
		}
		return pick(lowAvoids(trips[0]))

	// Rules 6 and 2: one pair.
	case len(pairs) == 1:
		if low, ok := pickStraight(func(s Setting) bool { return s.LowS.Pair() }); ok {
			return low
		}
		return pick(lowAvoids(pairs[0]))
	}

	// Rules 6 and 1: no pair.
	if low, ok := pickStraight(nil); ok {
		return low
	}
	return pick(nil)
}
