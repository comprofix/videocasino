package table

import "slices"

// Saved is what is kept between sessions so that a game can be carried on:
// the chip, the bets, and a hand that has been dealt but not yet played or
// folded. The credits are the bank's to keep.
//
// The dealer's cards are never in it: they are not dealt until the player
// has decided.
type Saved struct {
	Chip int
	Bets [Bets]int
	Last [Bets]int

	Dealt    bool
	Stake    [Bets]int
	Hand     Hand
	PairPaid int
}

// Save returns the game as it stands, for keeping. Only a hand still to be
// played is kept: one that has been settled is over.
func (m *Machine) Save() Saved {
	s := Saved{Chip: m.Chip, Bets: m.Bets, Last: m.Last}
	if m.Dealt {
		s.Dealt = true
		s.Stake, s.Hand, s.PairPaid = m.Stake, m.Hand, m.PairPaid
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
// left waiting: three different cards, dealt for an ante, with Pair Plus
// paid what those cards pay.
func validHand(s Saved) bool {
	if !validBets(s.Stake) || s.Stake[Ante] == 0 {
		return false
	}
	for i, c := range s.Hand {
		if !c.Valid() || slices.Contains(s.Hand[:i], c) {
			return false
		}
	}
	paid := 0
	if pays := PairPlusPays[s.Hand.Category()]; pays > 0 {
		paid = s.Stake[PairPlus] * (pays + 1)
	}
	return s.PairPaid == paid
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

	m.Dealt, m.Stake, m.Hand, m.PairPaid = false, [Bets]int{}, Hand{}, 0
	if s.Dealt && validHand(s) {
		m.Dealt = true
		m.Stake, m.Hand, m.PairPaid = s.Stake, s.Hand, s.PairPaid
		return // the table is clear while a hand is played
	}
	if validBets(s.Bets) {
		m.Bets = s.Bets
	}
}
