package cards

import "testing"

func TestEveryCard(t *testing.T) {
	// Each rank and suit makes a card of its own, and the card gives them
	// back.
	var seen [DeckSize]bool
	for r := Two; r <= Ace; r++ {
		for s := Clubs; s <= Spades; s++ {
			c := New(r, s)
			if !c.Valid() {
				t.Fatalf("New(%d, %d) = %d, not a card in the deck", r, s, c)
			}
			if seen[c] {
				t.Errorf("New(%d, %d) = %d, a card already made", r, s, c)
			}
			seen[c] = true
			if c.Rank() != r || c.Suit() != s {
				t.Errorf("card %d is rank %d suit %d, want rank %d suit %d", c, c.Rank(), c.Suit(), r, s)
			}
		}
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		card Card
		want string
	}{
		{New(Two, Clubs), "2c"},
		{New(Ten, Diamonds), "Td"},
		{New(Queen, Hearts), "Qh"},
		{New(Ace, Spades), "As"},
	}
	for _, tt := range tests {
		if got := tt.card.String(); got != tt.want {
			t.Errorf("card %d is named %q, want %q", tt.card, got, tt.want)
		}
	}
}

func TestValid(t *testing.T) {
	if New(Two, Clubs) != 0 || New(Ace, Spades) != DeckSize-1 {
		t.Error("the deck does not run from the two of clubs to the ace of spades")
	}
	for _, c := range []Card{-1, DeckSize} {
		if c.Valid() {
			t.Errorf("card %d counted as one in the deck", c)
		}
	}
}
