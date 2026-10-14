package table

import (
	"slices"

	"videocasino/cards"
)

// Saved is what is kept between sessions so that a game can be carried on:
// the chip, the bets on the table, the bets of the last hand, how the hands
// of this shoe went, and the shoe. A hand is dealt and settled all at once,
// so there is never one waiting. The credits are the bank's to keep.
type Saved struct {
	Chip    int
	Bets    [Bets]int
	Last    [Bets]int
	History []Outcome `json:",omitempty"`

	// The shoe: how many of each card are left, and whether its last hand
	// has been dealt. No order of cards is kept, as there is none: each
	// card is drawn when it is dealt. The cards that were burnt are simply
	// gone from it. Where the cut card stands follows from how many cards
	// are left, as it is always the same number from the end.
	Shoe     [cards.DeckSize]int
	Finished bool
}

// Save returns the game as it stands, for keeping.
func (m *Machine) Save() Saved {
	return Saved{
		Chip:     m.Chip,
		Bets:     m.Bets,
		Last:     m.Last,
		History:  slices.Clone(m.History),
		Shoe:     m.Left,
		Finished: m.Finished,
	}
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

// Restore puts a saved game back. Anything in it that could not have come
// from play is put right, as the save may have been damaged or edited: the
// chip falls back to the smallest, bets that could not have been made are
// cleared, and a shoe that could not have been left is shuffled.
func (m *Machine) Restore(s Saved) {
	m.Chip = ChipChoices[0]
	if slices.Contains(ChipChoices, s.Chip) {
		m.Chip = s.Chip
	}
	m.Bets, m.Last = [Bets]int{}, [Bets]int{}
	if validBets(s.Bets) {
		m.Bets = s.Bets
	}
	if validBets(s.Last) {
		m.Last = s.Last
	}

	if !validShoe(s) {
		m.shuffle()
		return
	}
	m.Left = s.Shoe
	left := m.CardsLeft()
	m.ToCut = left - CutBehind
	m.Fresh = left == ShoeSize
	m.Finished = s.Finished

	m.History = nil
	for _, o := range s.History {
		if o >= PlayerWins && o <= Tied && len(m.History) < HistoryLen {
			m.History = append(m.History, o)
		}
	}
}

// validShoe reports whether the shoe in a save is one that play could have
// left: no more of any card than eight decks hold, enough cards for the
// hands still to come, and its last hand dealt only if the cut card is out.
func validShoe(s Saved) bool {
	left := 0
	for _, k := range s.Shoe {
		if k < 0 || k > Decks {
			return false
		}
		left += k
	}
	// A full shoe has had no hand dealt from it, and one that has been
	// burnt has lost at least two cards. The fewest that can be left is
	// after the cut card comes out with the first card of a hand of six,
	// and the one hand more is six cards too.
	const mostToAHand = 6
	switch {
	case left > ShoeSize, left == ShoeSize-1:
		return false
	case left == ShoeSize:
		return !s.Finished
	case s.Finished:
		return left <= CutBehind && left >= CutBehind+1-2*mostToAHand
	}
	return left >= CutBehind+1-mostToAHand
}
