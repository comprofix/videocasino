package table

import (
	"encoding/json"
	"reflect"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
)

// splitGame returns a game saved in the middle of a split: the first hand
// being played and the second waiting its turn.
func splitGame() *Machine {
	m := stacked(50, 5, "8s 6h 8d 3c")
	m.Deal()
	m.Split()
	return m
}

// TestSaveRestore carries a game over to another machine, by way of JSON as
// the saved game is kept, in the middle of a hand.
func TestSaveRestore(t *testing.T) {
	m := splitGame()
	data, err := json.Marshal(m.Save())
	if err != nil {
		t.Fatal(err)
	}
	var s Saved
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	other := New(bank.New(), 8, 9)
	other.Restore(s)
	if !reflect.DeepEqual(other.Save(), m.Save()) {
		t.Fatalf("restored %+v, want %+v", other.Save(), m.Save())
	}

	// The restored game shares no cards with the one saved.
	other.Hands[0].Cards[0] = cards.New(cards.Two, cards.Clubs)
	if m.Hands[0].Cards[0] != cards.New(cards.Eight, cards.Spades) {
		t.Error("the restored game shares its cards with the saved one")
	}
	other.Restore(s)

	// The hand can be played out where it was left.
	other.next = deck("Tc 2h Kd 9s")
	other.Hit()   // 21 on the first hand
	other.Stand() // 10 on the second, and the dealer goes bust
	if other.Dealt || other.Last.Paid != 200 {
		t.Errorf("played out the restored hand for %d: %+v", other.Last.Paid, other.Last)
	}

	// A game with no hand waiting keeps only the bets and the shoe.
	m = stacked(100, 25, "Ts 7h 9s Td")
	m.Deal()
	m.Stand()
	got := m.Save()
	if got.Dealt || got.Bet != 100 || got.SideBet != 25 || len(got.Hands) != 0 || len(got.Dealer) != 0 || got.Stake != 0 {
		t.Errorf("saved %+v after the hand, want only the bets and the shoe", got)
	}
	if got.Shoe != m.Shoe || got.ToCut != m.ToCut {
		t.Errorf("saved a shoe of %v cut at %d, want %v cut at %d", got.Shoe, got.ToCut, m.Shoe, m.ToCut)
	}
}

// TestRestoreBad checks that a save that could not have come from play is
// put right.
func TestRestoreBad(t *testing.T) {
	good := splitGame().Save()
	m := New(bank.New(), 1, 2)
	m.Restore(good)
	if !reflect.DeepEqual(m.Save(), good) {
		t.Fatalf("a good save was restored as %+v", m.Save())
	}

	twos := make([]cards.Card, Decks+1)
	tests := []struct {
		name  string
		spoil func(*Saved)
	}{
		{"a stake not on offer", func(s *Saved) { s.Stake = 7 }},
		{"a side bet not on offer", func(s *Saved) { s.Side = 7 }},
		{"a dealer with two cards", func(s *Saved) { s.Dealer = deck("6h Kd") }},
		{"no dealer", func(s *Saved) { s.Dealer = nil }},
		{"no hands", func(s *Saved) { s.Hands = nil }},
		{"three hands", func(s *Saved) { s.Hands = append(s.Hands, s.Hands[1]) }},
		{"no hand being played", func(s *Saved) { s.Active = 2 }},
		{"a hand before the first", func(s *Saved) { s.Active = -1 }},
		{"a card past the end of the deck", func(s *Saved) { s.Hands[0].Cards[1] = cards.DeckSize }},
		{"a card before the start of the deck", func(s *Saved) { s.Dealer[0] = -1 }},
		{"a card more often than the shoe has it", func(s *Saved) { s.Hands[0].Cards = twos }},
		{"a finished hand being played", func(s *Saved) { s.Hands[0].Done = true }},
		{"a hand of 21 being played", func(s *Saved) { s.Hands[0].Cards = deck("8s 3c Td") }},
		{"a bust hand being played", func(s *Saved) { s.Hands[0].Cards = deck("8s 9c Td") }},
		{"a hand of one card being played", func(s *Saved) { s.Hands[0].Cards = deck("8s") }},
		{"a hand at the wrong bet", func(s *Saved) { s.Hands[0].Bet = 20 }},
		{"a doubled hand being played", func(s *Saved) { s.Hands[0].Doubled = true }},
		{"a waiting hand with two cards", func(s *Saved) { s.Hands[1].Cards = deck("8d 2h") }},
		{"an unfinished hand already played", func(s *Saved) { s.Active = 1; s.Hands[1].Cards = deck("8d 2h") }},
		{"a pair that is not one", func(s *Saved) { s.Pair = 9 }},
	}
	for _, tt := range tests {
		s := splitGame().Save()
		tt.spoil(&s)
		m := New(bank.New(), 1, 2)
		m.Restore(s)
		if m.Dealt || len(m.Hands) != 0 || len(m.Dealer) != 0 {
			t.Errorf("%s: restored a hand: %+v", tt.name, m.Save())
		}
		// The bets are kept, and whatever was restored has to be playable.
		if m.Bet != 50 || m.SideBet != 5 {
			t.Errorf("%s: restored bets of %d and %d", tt.name, m.Bet, m.SideBet)
		}
		if !m.Deal() {
			t.Errorf("%s: could not deal", tt.name)
		}
	}

	// Bets that are not on offer fall back to the smallest.
	m = New(bank.New(), 1, 2)
	m.Restore(Saved{Bet: 7, SideBet: 3})
	if m.Bet != BetChoices[0] || m.SideBet != SideChoices[0] {
		t.Errorf("restored bets of %d and %d, want the smallest", m.Bet, m.SideBet)
	}
}

// TestRestoreShoe checks that a shoe that play could not have left is
// shuffled, less the cards on the table, and that a good one is kept.
func TestRestoreShoe(t *testing.T) {
	good := splitGame().Save()
	eight := cards.New(cards.Eight, cards.Spades) // on the table
	tests := []struct {
		name  string
		spoil func(*Saved)
	}{
		{"fewer than none of a card", func(s *Saved) { s.Shoe[0] = -1 }},
		{"more of a card than the decks hold", func(s *Saved) { s.Shoe[0] = Decks + 1 }},
		{"a card on the table that never left the shoe", func(s *Saved) { s.Shoe[eight] = Decks }},
		{"an empty shoe", func(s *Saved) { s.Shoe = [cards.DeckSize]int{} }},
		{"a cut card past the end", func(s *Saved) { s.ToCut = ShoeSize }},
		{"a cut card too near the front", func(s *Saved) { s.ToCut -= ShoeSize / 2 }},
	}
	for _, tt := range tests {
		s := splitGame().Save()
		tt.spoil(&s)
		m := New(bank.New(), 1, 2)
		m.Restore(s)
		// The hand is kept, and its four cards are out of the new shoe.
		if !m.Dealt || len(m.Hands) != 2 {
			t.Errorf("%s: the hand was thrown away with the shoe", tt.name)
		}
		if left := m.CardsLeft(); left != ShoeSize-4 || m.Shoe[eight] != Decks-1 {
			t.Errorf("%s: %d cards left with %d eights of spades, want %d and %d", tt.name, left, m.Shoe[eight], ShoeSize-4, Decks-1)
		}
		if cut := m.ToCut + 4; cut < cutMin || cut > cutMax {
			t.Errorf("%s: the cut card was put %d cards in", tt.name, cut)
		}
		if !m.Hit() {
			t.Errorf("%s: could not play on", tt.name)
		}
	}

	// A shoe well run down is kept as it is.
	m := New(bank.New(), 1, 2)
	m.Restore(good)
	if m.Shoe != good.Shoe || m.ToCut != good.ToCut {
		t.Errorf("a good shoe was not kept")
	}
}
