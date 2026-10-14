package wheel

import "slices"

// Stake is what is bet on one spot. The spot is its place in Spots.
type Stake struct {
	Spot   int
	Amount int
}

// Saved is the part of a game that is kept from one session to the next:
// the chip, the bets on the table, the bets of the last spin and the numbers
// that have come up. The credits are the bank's to keep.
type Saved struct {
	Chip    int
	Bets    []Stake `json:",omitempty"`
	Last    []Stake `json:",omitempty"`
	History []int   `json:",omitempty"`
}

// stakes lists the spots that have something bet on them.
func stakes(bets []int) []Stake {
	var s []Stake
	for i, bet := range bets {
		if bet > 0 {
			s = append(s, Stake{i, bet})
		}
	}
	return s
}

// Save returns the game as it stands, for keeping.
func (m *Machine) Save() Saved {
	return Saved{
		Chip:    m.Chip,
		Bets:    stakes(m.Bets),
		Last:    stakes(m.Last),
		History: slices.Clone(m.History),
	}
}

// restore lays saved bets out spot by spot, leaving out any that could not
// have been made.
func restore(bets []int, saved []Stake) {
	clear(bets)
	for _, s := range saved {
		if s.Spot >= 0 && s.Spot < len(bets) && bets[s.Spot] == 0 &&
			s.Amount > 0 && s.Amount <= MaxSpot && s.Amount%ChipChoices[0] == 0 {
			bets[s.Spot] = s.Amount
		}
	}
}

// Restore puts a saved game back. Anything in it that could not have come
// from play is put right, as the save may have been damaged or edited: the
// chip falls back to the smallest, and a bet or a number that could not be
// is left out.
func (m *Machine) Restore(s Saved) {
	m.Chip = ChipChoices[0]
	if slices.Contains(ChipChoices, s.Chip) {
		m.Chip = s.Chip
	}
	restore(m.Bets, s.Bets)
	restore(m.Last, s.Last)

	m.History = nil
	for _, n := range s.History {
		if n >= 0 && n < Pockets && len(m.History) < HistoryLen {
			m.History = append(m.History, n)
		}
	}
}
