package table

import "videocasino/cards"

// The odds of the game from a freshly shuffled shoe, worked out from the
// rules rather than by playing: every way the six cards of a hand can come
// out of the shoe is followed, each weighed by how likely it is.

// Odds is the chances of a hand dealt from a full shoe.
type Odds struct {
	PlayerWins float64
	BankerWins float64
	Tied       float64
	// BankerSix is the chance the banker wins with a six, which is part of
	// BankerWins.
	BankerSix float64
	// Pair is the chance that a hand's first two cards are a pair. It is the
	// same for either hand.
	Pair float64
}

// NewOdds works out the odds of a hand dealt from a full shoe.
//
// Only what a card counts matters to which hand wins, so the shoe is
// counted by value: there are four times as many cards that count nothing
// as there are of any other value.
func NewOdds() Odds {
	var left [10]int
	for c := range cards.Card(cards.DeckSize) {
		left[Value(c)] += Decks
	}
	n := ShoeSize

	var o Odds
	// take follows each value the next card can have, with its chance.
	take := func(chance float64, then func(v int, chance float64)) {
		for v, k := range left {
			if k == 0 {
				continue
			}
			left[v]--
			n--
			then(v, chance*float64(k)/float64(n+1))
			left[v]++
			n++
		}
	}
	settle := func(player, banker int, chance float64) {
		switch {
		case player > banker:
			o.PlayerWins += chance
		case banker > player:
			o.BankerWins += chance
			if banker == BankerSix {
				o.BankerSix += chance
			}
		default:
			o.Tied += chance
		}
	}
	// banker plays the banker's hand out once the player's is done.
	banker := func(player, bank, third int, chance float64) {
		if !BankerDraws(bank, third) {
			settle(player, bank, chance)
			return
		}
		take(chance, func(v int, chance float64) {
			settle(player, (bank+v)%10, chance)
		})
	}

	take(1, func(p1 int, chance float64) {
		take(chance, func(b1 int, chance float64) {
			take(chance, func(p2 int, chance float64) {
				take(chance, func(b2 int, chance float64) {
					player, bank := (p1+p2)%10, (b1+b2)%10
					switch {
					case player >= Natural || bank >= Natural:
						settle(player, bank, chance)
					case PlayerDraws(player):
						take(chance, func(p3 int, chance float64) {
							banker((player+p3)%10, bank, p3, chance)
						})
					default:
						banker(player, bank, -1, chance)
					}
				})
			})
		})
	})

	// A pair: any first card, then another of its rank from what is left.
	// The other hand's first card comes between them, but as nothing is
	// known of it the chance is the same as if it did not.
	o.Pair = float64(Decks*cards.NumSuits-1) / float64(ShoeSize-1)
	return o
}

// Return is the fraction of what is bet on b that it pays back over a long
// run of hands, each from a full shoe.
func (o Odds) Return(b Bet) float64 {
	win := float64(Pays[b] + 1)
	switch b {
	case Player:
		return o.PlayerWins*win + o.Tied
	case Banker:
		// A win with a six pays half, and the bet back.
		half := 1 + float64(Pays[b])/2
		return (o.BankerWins-o.BankerSix)*win + o.BankerSix*half + o.Tied
	case Tie:
		return o.Tied * win
	}
	return o.Pair * win
}
