// Package shoe holds the shoe that card games are dealt from: several decks
// together, dealt hand after hand down to a cut card. It has no graphics so
// it can be tested on its own.
package shoe

import (
	"math/rand/v2"

	"videocasino/cards"
)

// Shoe is the cards left to be dealt. A card that has been dealt stays out
// until the shoe is filled again.
//
// The cards are not kept in any order: each is drawn at random from what is
// left, which comes to the same thing as dealing from a shuffled shoe and
// leaves no card waiting to be learnt.
type Shoe struct {
	// Left is how many of each card are left.
	Left [cards.DeckSize]int
	// ToCut is how many more cards are to be dealt before the cut card
	// comes out. Once it has it is nought or less, by the number of cards
	// dealt since.
	ToCut int

	decks int

	// next is cards to be dealt before any are drawn at random. Only tests
	// use it.
	next []cards.Card
}

// New returns a shoe for the given number of decks. It is empty until it is
// filled.
func New(decks int) Shoe {
	return Shoe{decks: decks}
}

// Decks is the number of decks the shoe holds when it is full.
func (s *Shoe) Decks() int {
	return s.decks
}

// Size is the number of cards the shoe holds when it is full.
func (s *Shoe) Size() int {
	return s.decks * cards.DeckSize
}

// Fill puts every card back in the shoe, and the cut card in after the
// given number of them.
func (s *Shoe) Fill(toCut int) {
	for c := range s.Left {
		s.Left[c] = s.decks
	}
	s.ToCut = toCut
}

// CardsLeft is how many cards are left in the shoe.
func (s *Shoe) CardsLeft() int {
	n := 0
	for _, k := range s.Left {
		n += k
	}
	return n
}

// CutOut reports whether the cut card has come out.
func (s *Shoe) CutOut() bool {
	return s.ToCut <= 0
}

// Draw takes a card at random from what is left in the shoe. It is chosen
// only now, so nothing about it can be learnt beforehand.
func (s *Shoe) Draw(rng *rand.Rand) cards.Card {
	s.ToCut--
	if len(s.next) > 0 {
		c := s.next[0]
		s.next = s.next[1:]
		if s.Left[c] > 0 {
			s.Left[c]--
		}
		return c
	}
	i := rng.IntN(s.CardsLeft())
	for c, k := range s.Left {
		if i < k {
			s.Left[c]--
			return cards.Card(c)
		}
		i -= k
	}
	panic("shoe: the shoe is empty")
}

// Stack sets cards to be dealt, in order, before any more are drawn at
// random. It is for tests, which need to know what is dealt.
func (s *Shoe) Stack(next ...cards.Card) {
	s.next = next
}

// Stacked is how many of the cards set by Stack are still to be dealt.
func (s *Shoe) Stacked() int {
	return len(s.next)
}
