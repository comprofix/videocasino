package table

import (
	"slices"

	"videocasino/cards"
)

// Saved is what is kept between sessions so that a game can be carried on:
// the chip, the bets, and a hand that has been dealt but not yet played.
// The credits are the bank's to keep.
//
// The dealer's cards are never in it: they are not dealt until the hand is
// played.
type Saved struct {
	Chip int
	Bet  int
	Last int

	Dealt bool
	Stake int
	Hand  [HandSize]cards.Card
}

// Save returns the game as it stands, for keeping. Only a hand still to be
// played is kept: one that has been settled is over.
func (m *Machine) Save() Saved {
	s := Saved{Chip: m.Chip, Bet: m.Bet, Last: m.Last}
	if m.Dealt {
		s.Dealt, s.Stake, s.Hand = true, m.Stake, m.Hand
	}
	return s
}

// validBet reports whether a saved bet is one that could have been made.
func validBet(bet int) bool {
	return bet >= 0 && bet <= MaxBet && bet%ChipChoices[0] == 0
}

// validHand reports whether the hand in a save is one that play could have
// left waiting: seven different cards of the 53, dealt for a bet.
func validHand(s Saved) bool {
	if !validBet(s.Stake) || s.Stake == 0 {
		return false
	}
	for i, c := range s.Hand {
		if !Valid(c) || slices.Contains(s.Hand[:i], c) {
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
	m.Bet, m.Last = 0, 0
	if validBet(s.Last) {
		m.Last = s.Last
	}

	m.Dealt, m.Stake, m.Hand = false, 0, [HandSize]cards.Card{}
	if s.Dealt && validHand(s) {
		m.Dealt, m.Stake, m.Hand = true, s.Stake, s.Hand
		return // the table is clear while a hand is played
	}
	if validBet(s.Bet) {
		m.Bet = s.Bet
	}
}
