package table

import "videocasino/cards"

// The odds of the game, worked out from the rules rather than by playing:
// every hand the player can hold is set against every hand the dealer can
// then hold.

// combo is three cards of the deck taken together, with what they make.
type combo struct {
	mask  uint64 // a bit for each of its cards
	score Score
}

// combos returns every hand of three cards that can be dealt from a deck.
func combos() []combo {
	all := make([]combo, 0, 22100)
	for a := cards.Card(0); a < cards.DeckSize; a++ {
		for b := a + 1; b < cards.DeckSize; b++ {
			for c := b + 1; c < cards.DeckSize; c++ {
				all = append(all, combo{1<<a | 1<<b | 1<<c, Hand{a, b, c}.Score()})
			}
		}
	}
	return all
}

// Advice is what playing a hand is worth.
type Advice struct {
	// Play is what the player wins or loses on average by playing the
	// hand, for each credit of the ante. Folding always loses 1.
	Play float64
	// Best reports whether playing is better than folding.
	Best bool
}

// playNet adds up what playing a hand wins or loses, in antes, over every
// hand the dealer can hold, and returns it with the number of those hands.
func playNet(player combo, all []combo) (net, hands int) {
	for _, d := range all {
		if d.mask&player.mask != 0 {
			continue // the dealer cannot hold the player's cards
		}
		_, paid := Settle(player.score, d.score)
		net += paid - 2 // the ante and the play bet were staked
		hands++
	}
	return net, hands
}

// Advise weighs up playing a hand against folding it. It is exact: every
// hand the dealer can be dealt from the rest of the deck is counted.
func Advise(h Hand) Advice {
	player := combo{1<<h[0] | 1<<h[1] | 1<<h[2], h.Score()}
	net, hands := playNet(player, combos())
	return Advice{
		Play: float64(net) / float64(hands),
		Best: net > -hands,
	}
}

// Odds is the odds of the game for a player who plays every hand that is
// worth more played than folded.
type Odds struct {
	// AnteReturn is the fraction of what is staked on the ante and the play
	// bet together that is paid back over a long run.
	AnteReturn float64
	// AnteEdge is what is lost on average for each credit of the ante.
	AnteEdge float64
	// Played is the fraction of hands that are worth playing.
	Played float64
	// PairPlusReturn is the fraction of what is bet on Pair Plus that it
	// pays back.
	PairPlusReturn float64
	// Chance is how often the player is dealt a hand of each category.
	Chance [Categories]float64
	// Weakest is the worst hand that is worth playing.
	Weakest Score
}

// NewOdds works out the odds of the game.
func NewOdds() Odds {
	all := combos()
	var o Odds
	var net, staked, played, hands, pairPaid int
	var dealt [Categories]int
	o.Weakest = -1
	for _, p := range all {
		dealt[p.score.Category()]++
		if pays := PairPlusPays[p.score.Category()]; pays > 0 {
			pairPaid += pays + 1
		}

		play, n := playNet(p, all)
		hands += n
		if play > -n {
			net += play
			staked += 2 * n
			played++
			if o.Weakest < 0 || p.score < o.Weakest {
				o.Weakest = p.score
			}
		} else {
			net -= n // folded: the ante is lost
			staked += n
		}
	}
	o.AnteEdge = -float64(net) / float64(hands)
	o.AnteReturn = float64(staked+net) / float64(staked)
	o.Played = float64(played) / float64(len(all))
	o.PairPlusReturn = float64(pairPaid) / float64(len(all))
	for c, n := range dealt {
		o.Chance[c] = float64(n) / float64(len(all))
	}
	return o
}
