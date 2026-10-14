package table

import (
	"slices"

	"videocasino/cards"
)

// Saved is what is kept between sessions so that a game can be carried on:
// the chip, the bets, and a hand that has been dealt but not yet raised or
// folded, with the dealer's card that is face up. The credits are the
// bank's to keep.
//
// The dealer's other cards are never in it: they are not dealt until the
// player has decided.
type Saved struct {
	Chip int
	Ante int
	Last int

	Dealt bool
	Stake int
	Hand  Hand
	Up    cards.Card
}

// Save returns the game as it stands, for keeping. Only a hand still to be
// played is kept: one that has been settled is over.
func (m *Machine) Save() Saved {
	s := Saved{Chip: m.Chip, Ante: m.Ante, Last: m.Last}
	if m.Dealt {
		s.Dealt = true
		s.Stake, s.Hand, s.Up = m.Stake, m.Hand, m.Up
	}
	return s
}

// validBet reports whether a saved bet is one that could have been made.
func validBet(bet int) bool {
	return bet >= 0 && bet <= MaxBet && bet%ChipChoices[0] == 0
}

// validHand reports whether the hand in a save is one that play could have
// left waiting: six different cards, dealt for an ante.
func validHand(s Saved) bool {
	if !validBet(s.Stake) || s.Stake == 0 {
		return false
	}
	dealt := append(s.Hand[:], s.Up)
	for i, c := range dealt {
		if !c.Valid() || slices.Contains(dealt[:i], c) {
			return false
		}
	}
	return true
}

// Restore puts a saved game back. Anything in it that could not have come
// from play is put right, as the save may have been damaged or edited: the
// chip falls back to the smallest, bets that could not have been made are
// cleared, and a hand that could not have been dealt is thrown away.
func (m *Machine) Restore(s Saved) {
	m.Chip = ChipChoices[0]
	if slices.Contains(ChipChoices, s.Chip) {
		m.Chip = s.Chip
	}
	m.Ante, m.Last = 0, 0
	if validBet(s.Last) {
		m.Last = s.Last
	}

	m.Dealt, m.Stake, m.Hand, m.Up = false, 0, Hand{}, 0
	if s.Dealt && validHand(s) {
		m.Dealt = true
		m.Stake, m.Hand, m.Up = s.Stake, s.Hand, s.Up
		return // the table is clear while a hand is played
	}
	if validBet(s.Ante) {
		m.Ante = s.Ante
	}
}
