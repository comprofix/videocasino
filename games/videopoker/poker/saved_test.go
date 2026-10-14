package poker

import (
	"encoding/json"
	"testing"
	"videocasino/bank"
	"videocasino/cards"
)

// TestSaveRestore carries a game over to another machine, by way of JSON as
// the saved game is kept, between the deal and the draw.
func TestSaveRestore(t *testing.T) {
	m := New(bank.New(), 1, 2)
	m.Coins, m.CoinValue = 4, 25
	m.Deal()
	m.ToggleHold(0)
	m.ToggleHold(3)

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
	if other.Save() != m.Save() {
		t.Errorf("restored %+v, want %+v", other.Save(), m.Save())
	}

	// The hand can be played out where it was left.
	r, ok := other.Draw()
	if !ok {
		t.Fatal("could not draw to the restored hand")
	}
	if r.Hand[0] != m.Hand[0] || r.Hand[3] != m.Hand[3] {
		t.Errorf("held cards of %v became %v", m.Hand, r.Hand)
	}
}

// TestRestoreBad checks that a save that could not have come from play is
// put right.
func TestRestoreBad(t *testing.T) {
	good := Saved{Coins: 5, CoinValue: 25, Dealt: true, Hand: hand("As Kd 7h 7c 2s"), Held: [HandSize]bool{2: true, 3: true}}
	m := New(bank.New(), 1, 2)
	m.Restore(good)
	if m.Save() != good {
		t.Fatalf("a good save was restored as %+v", m.Save())
	}

	tests := []struct {
		name  string
		spoil func(*Saved)
		check func(*Machine) bool
	}{
		{"no coins", func(s *Saved) { s.Coins = 0 }, func(m *Machine) bool { return m.Coins == 1 }},
		{"too many coins", func(s *Saved) { s.Coins = MaxCoins + 1 }, func(m *Machine) bool { return m.Coins == 1 }},
		{"a coin value not on offer", func(s *Saved) { s.CoinValue = 7 }, func(m *Machine) bool { return m.CoinValue == CoinValues[0] }},
		{"the same card twice", func(s *Saved) { s.Hand[1] = s.Hand[0] }, func(m *Machine) bool { return !m.Dealt && m.Held == [HandSize]bool{} }},
		{"a card past the end of the deck", func(s *Saved) { s.Hand[4] = cards.DeckSize }, func(m *Machine) bool { return !m.Dealt }},
		{"a card before the start of the deck", func(s *Saved) { s.Hand[4] = -1 }, func(m *Machine) bool { return !m.Dealt }},
		{"holds with no hand dealt", func(s *Saved) { s.Dealt = false }, func(m *Machine) bool { return m.Held == [HandSize]bool{} }},
	}
	for _, tt := range tests {
		s := good
		tt.spoil(&s)
		m := New(bank.New(), 1, 2)
		m.Restore(s)
		if !tt.check(m) {
			t.Errorf("%s: restored as %+v", tt.name, m.Save())
		}
		// Whatever was restored has to be playable.
		if m.Dealt {
			if _, ok := m.Draw(); !ok {
				t.Errorf("%s: could not draw", tt.name)
			}
		}
		m.Bank.Reset()
		if _, ok := m.Deal(); !ok {
			t.Errorf("%s: could not deal", tt.name)
		}
	}
}
