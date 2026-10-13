package machine

// Saved is the part of a game that is kept from one session to the next:
// the bet, the two jackpot pots and any feature still to be played. The
// credits are the bank's to keep.
type Saved struct {
	Lines      int
	BetPerLine int
	FreeSpins  int
	Link       *Feature `json:",omitempty"`

	// The pots, in thousandths of a credit.
	MajorPot int
	GrandPot int
}

// Save returns the game as it stands, for keeping.
func (m *Machine) Save() Saved {
	s := Saved{
		Lines:      m.Lines,
		BetPerLine: m.BetPerLine,
		FreeSpins:  m.FreeSpins,
		MajorPot:   m.majorPot,
		GrandPot:   m.grandPot,
	}
	if m.Link != nil {
		link := *m.Link
		s.Link = &link
	}
	return s
}

// Restore puts a saved game back. Anything in it that could not have come
// from play is put right, as the save may have been damaged or edited: a
// bet falls back to the smallest, and a pot never drops below its start.
func (m *Machine) Restore(s Saved) {
	m.FreeSpins = max(s.FreeSpins, 0)
	m.majorPot = max(s.MajorPot, MajorStart*potUnit)
	m.grandPot = max(s.GrandPot, GrandStart*potUnit)

	m.Lines, m.BetPerLine = LineChoices[0], BetChoices[0]
	for _, n := range LineChoices {
		if n == s.Lines {
			m.Lines = n
		}
	}
	for _, n := range BetChoices {
		if n == s.BetPerLine {
			m.BetPerLine = n
		}
	}

	m.Link = nil
	if l := s.Link; l != nil && l.Count() >= LinkTrigger && l.Count() < Cells &&
		l.Respins >= 1 && l.Respins <= LinkRespins && l.Bet >= 1 && l.Bet <= MaxBet {
		link := *l
		m.Link = &link
	}
}
