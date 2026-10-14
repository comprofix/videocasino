package shoe

import (
	"math/rand/v2"
	"testing"

	"videocasino/cards"
)

func TestShoe(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	s := New(2)
	if s.Decks() != 2 || s.Size() != 2*cards.DeckSize || s.CardsLeft() != 0 {
		t.Fatalf("a new shoe of 2 decks: %d decks, room for %d, %d cards", s.Decks(), s.Size(), s.CardsLeft())
	}
	s.Fill(100)
	if s.CardsLeft() != s.Size() || s.CutOut() {
		t.Fatalf("a full shoe has %d cards, cut card out %v", s.CardsLeft(), s.CutOut())
	}

	// Every card comes out as often as the shoe holds it, and the cut card
	// after the number of cards it was put in behind.
	var dealt [cards.DeckSize]int
	for i := range s.Size() {
		if s.CutOut() != (i >= 100) {
			t.Fatalf("after %d cards the cut card is out: %v", i, s.CutOut())
		}
		c := s.Draw(rng)
		if !c.Valid() {
			t.Fatalf("drew %d, which is not a card", c)
		}
		dealt[c]++
		if s.CardsLeft() != s.Size()-i-1 || s.ToCut != 100-i-1 {
			t.Fatalf("after %d cards: %d left, cut card in %d", i+1, s.CardsLeft(), s.ToCut)
		}
	}
	for c, k := range dealt {
		if k != 2 {
			t.Errorf("%v was dealt %d times from 2 decks", cards.Card(c), k)
		}
	}

	// A fresh fill puts them all back.
	s.Fill(60)
	if s.CardsLeft() != s.Size() || s.ToCut != 60 {
		t.Errorf("filled again: %d cards, cut card in %d", s.CardsLeft(), s.ToCut)
	}
}

func TestStack(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	s := New(1)
	s.Fill(40)
	ace, king := cards.New(cards.Ace, cards.Spades), cards.New(cards.King, cards.Hearts)
	s.Stack(ace, king, ace)
	if s.Stacked() != 3 {
		t.Fatalf("%d cards are stacked", s.Stacked())
	}
	for i, want := range []cards.Card{ace, king, ace} {
		if got := s.Draw(rng); got != want {
			t.Errorf("card %d was %v, want %v", i+1, got, want)
		}
	}
	// The stacked cards came out of the shoe, as far as it had them: the
	// second ace was not there to take.
	if s.Stacked() != 0 || s.Left[ace] != 0 || s.Left[king] != 0 || s.CardsLeft() != cards.DeckSize-2 || s.ToCut != 37 {
		t.Errorf("after the stack: %d stacked, %d aces, %d kings, %d cards, cut card in %d",
			s.Stacked(), s.Left[ace], s.Left[king], s.CardsLeft(), s.ToCut)
	}
	// The rest are drawn from what is left.
	for range 50 {
		if c := s.Draw(rng); c == ace || c == king {
			t.Fatalf("drew %v again from one deck", c)
		}
	}
}
