package poker

import (
	"slices"

	"videocasino/cards"
)

// Saved is what is kept between sessions so that a game can be carried on:
// the bet and any hand still to be drawn to. The credits are the bank's to
// keep.
type Saved struct {
	Coins     int
	CoinValue int

	Dealt bool
	Hand  [HandSize]cards.Card
	Held  [HandSize]bool
}

// Save returns the game as it stands, for keeping.
func (m *Machine) Save() Saved {
	return Saved{
		Coins:     m.Coins,
		CoinValue: m.CoinValue,
		Dealt:     m.Dealt,
		Hand:      m.Hand,
		Held:      m.Held,
	}
}

// Restore puts a saved game back. Anything in it that could not have come
// from play is put right, as the save may have been damaged or edited: a
// bet falls back to the smallest, and a hand that is not five different
// cards is thrown away.
func (m *Machine) Restore(s Saved) {
	m.Coins = 1
	if s.Coins >= 1 && s.Coins <= MaxCoins {
		m.Coins = s.Coins
	}
	m.CoinValue = CoinValues[0]
	if slices.Contains(CoinValues, s.CoinValue) {
		m.CoinValue = s.CoinValue
	}

	m.Hand, m.Held, m.Dealt = [HandSize]cards.Card{}, [HandSize]bool{}, false
	if s.Dealt && validHand(s.Hand) {
		m.Hand, m.Held, m.Dealt = s.Hand, s.Held, true
	}
}

// validHand reports whether a hand is five different cards from the deck.
func validHand(hand [HandSize]cards.Card) bool {
	var seen [cards.DeckSize]bool
	for _, c := range hand {
		if !c.Valid() || seen[c] {
			return false
		}
		seen[c] = true
	}
	return true
}
