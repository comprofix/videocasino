package table

import (
	"slices"

	"videocasino/cards"
)

// Saved is what is kept between sessions so that a game can be carried on:
// the chip, the bets, and a hand that is waiting, with the board as far as
// it is turned. The credits are the bank's to keep.
//
// The dealer's cards and the board still to be turned are never in it:
// they are not dealt until they are shown.
type Saved struct {
	Chip int
	Bets [Bets]int
	Last [Bets]int

	Round Round
	Stake [Bets]int
	Hole  [HoleSize]cards.Card
	Board []cards.Card // as many as are turned
}

// Save returns the game as it stands, for keeping. Only a hand still to be
// played is kept: one that has been settled is over.
func (m *Machine) Save() Saved {
	s := Saved{Chip: m.Chip, Bets: m.Bets, Last: m.Last}
	if m.Round != NoHand {
		s.Round, s.Stake, s.Hole = m.Round, m.Stake, m.Hole
		s.Board = slices.Clone(m.Board[:m.Round.Shown()])
	}
	return s
}

// validBets reports whether saved bets are ones that could have been made.
func validBets(bets [Bets]int) bool {
	for _, bet := range bets {
		if bet < 0 || bet > MaxBet || bet%ChipChoices[0] != 0 {
			return false
		}
	}
	return true
}

// validHand reports whether the hand in a save is one that play could have
// left waiting: dealt for an ante, at a round, with as many of the board
// turned as the round shows, and every card different.
func validHand(s Saved) bool {
	if !validBets(s.Stake) || s.Stake[Ante] == 0 || s.Round < Preflop || s.Round > River {
		return false
	}
	if len(s.Board) != s.Round.Shown() {
		return false
	}
	dealt := append(s.Hole[:], s.Board...)
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
	m.Bets, m.Last = [Bets]int{}, [Bets]int{}
	if validBets(s.Last) {
		m.Last = s.Last
	}

	m.Round, m.Stake, m.Hole, m.Board = NoHand, [Bets]int{}, [HoleSize]cards.Card{}, [BoardSize]cards.Card{}
	if s.Round != NoHand && validHand(s) {
		m.Round, m.Stake, m.Hole = s.Round, s.Stake, s.Hole
		copy(m.Board[:], s.Board)
		return // the table is clear while a hand is played
	}
	if validBets(s.Bets) {
		m.Bets = s.Bets
	}
}
