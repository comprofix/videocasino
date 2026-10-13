package table

import "videocasino/cards"

// PairKind is what the player's first two cards make for the Perfect Pairs
// side bet. A pair here is two cards of the same rank: a king and a queen
// are not one, though they can be split.
type PairKind int

const (
	NoPair       PairKind = iota
	MixedPair             // the same rank, one red and one black
	ColouredPair          // the same rank and colour, but different suits
	PerfectPair           // the same card twice
)

func (k PairKind) String() string {
	return [...]string{"No Pair", "Mixed Pair", "Coloured Pair", "Perfect Pair"}[k]
}

// PairPays is what each pair pays for every credit of the side bet, on top
// of the bet itself, which is returned.
var PairPays = [...]int{
	MixedPair:    6,
	ColouredPair: 12,
	PerfectPair:  25,
}

// red reports whether a card is a diamond or a heart.
func red(c cards.Card) bool {
	return c.Suit() == cards.Diamonds || c.Suit() == cards.Hearts
}

// PairOf returns the pair two cards make.
func PairOf(a, b cards.Card) PairKind {
	switch {
	case a.Rank() != b.Rank():
		return NoPair
	case a == b:
		return PerfectPair
	case red(a) == red(b):
		return ColouredPair
	}
	return MixedPair
}

// PairChances is how often the first two cards from a freshly shuffled
// shoe make each pair. As the shoe runs down the chances move with what is
// left in it. It is counted from every two cards that can be dealt.
func PairChances() (chance [PerfectPair + 1]float64) {
	for a := range cards.Card(cards.DeckSize) {
		for b := range cards.Card(cards.DeckSize) {
			// The second card comes from what the first has left.
			second := Decks
			if a == b {
				second--
			}
			ways := Decks * second
			chance[PairOf(a, b)] += float64(ways) / float64(ShoeSize*(ShoeSize-1))
		}
	}
	return chance
}

// PairReturn is the fraction of what is bet on Perfect Pairs that is paid
// back on a hand from a freshly shuffled shoe.
func PairReturn() float64 {
	total := 0.0
	for kind, chance := range PairChances() {
		if PairKind(kind) != NoPair {
			total += chance * float64(PairPays[kind]+1)
		}
	}
	return total
}
