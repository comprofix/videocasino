package table

import (
	"slices"

	"videocasino/cards"
)

// Saved is what is kept between sessions so that a game can be carried on:
// the bets, the shoe and any hand still to be played out. The credits are
// the bank's to keep.
type Saved struct {
	Bet     int
	SideBet int

	// The shoe: how many of each card are left, and how many more are to
	// be dealt before the cut card comes out. No order of cards is kept,
	// as there is none: each card is drawn when it is dealt.
	Shoe  [cards.DeckSize]int
	ToCut int

	Dealt    bool
	Shuffled bool
	Stake    int
	Side     int
	Dealer   []cards.Card
	Hands    []Hand
	Active   int
	Pair     PairKind
	SideWin  int
}

// copyHands returns hands that share no cards with the ones given.
func copyHands(hands []Hand) []Hand {
	out := make([]Hand, len(hands))
	for i, h := range hands {
		out[i] = h
		out[i].Cards = slices.Clone(h.Cards)
	}
	return out
}

// Save returns the game as it stands, for keeping. Only a hand still to be
// played out is kept: one that has been settled is over.
func (m *Machine) Save() Saved {
	s := Saved{Bet: m.Bet, SideBet: m.SideBet, Shoe: m.Left, ToCut: m.ToCut}
	if m.Dealt {
		s.Dealt = true
		s.Shuffled = m.Shuffled
		s.Stake, s.Side = m.Stake, m.Side
		s.Dealer = slices.Clone(m.Dealer)
		s.Hands = copyHands(m.Hands)
		s.Active = m.Active
		s.Pair, s.SideWin = m.Pair, m.SideWin
	}
	return s
}

// Restore puts a saved game back. Anything in it that could not have come
// from play is put right, as the save may have been damaged or edited: a
// bet falls back to the smallest, a hand that could not have been dealt is
// thrown away, and a shoe that could not have been left is shuffled.
func (m *Machine) Restore(s Saved) {
	m.Bet = BetChoices[0]
	if slices.Contains(BetChoices, s.Bet) {
		m.Bet = s.Bet
	}
	m.SideBet = SideChoices[0]
	if slices.Contains(SideChoices, s.SideBet) {
		m.SideBet = s.SideBet
	}

	m.Dealt, m.Shuffled, m.Stake, m.Side = false, false, 0, 0
	m.Dealer, m.Hands, m.Active = nil, nil, 0
	m.Pair, m.SideWin = NoPair, 0
	m.Last = Result{}
	if s.Dealt && validHand(s) {
		m.Dealt = true
		m.Shuffled = s.Shuffled
		m.Stake, m.Side = s.Stake, s.Side
		m.Dealer = slices.Clone(s.Dealer)
		m.Hands = copyHands(s.Hands)
		m.Active = s.Active
		m.Pair, m.SideWin = s.Pair, s.SideWin
	}

	// The cards on the table came out of the shoe.
	var table [cards.DeckSize]int
	for _, c := range m.Dealer {
		table[c]++
	}
	for _, h := range m.Hands {
		for _, c := range h.Cards {
			table[c]++
		}
	}
	if validShoe(s, table) {
		m.Left, m.ToCut = s.Shoe, s.ToCut
		return
	}
	m.shuffle()
	for c, k := range table {
		m.Left[c] -= k
		m.ToCut -= k
	}
}

// validShoe reports whether the shoe in a save is one that play could have
// left, given the cards on the table: no more of any card than six decks
// hold, and the cut card where a shuffle could have put it.
func validShoe(s Saved, table [cards.DeckSize]int) bool {
	left := 0
	for c, k := range s.Shoe {
		if k < 0 || k+table[c] > Decks {
			return false
		}
		left += k
	}
	// What lies behind the cut card is never dealt, so it is what the
	// shuffle left there: the cards left, less those still to come before
	// the cut card.
	behind := left - s.ToCut
	return behind >= ShoeSize-cutMax && behind <= ShoeSize-cutMin
}

// validHand reports whether the hand in a save is one that play could have
// left waiting: the dealer showing one card, and the player with a hand
// that can still take a card.
func validHand(s Saved) bool {
	if !slices.Contains(BetChoices, s.Stake) || !slices.Contains(SideChoices, s.Side) {
		return false
	}
	if len(s.Dealer) != 1 || len(s.Hands) < 1 || len(s.Hands) > 2 {
		return false
	}
	if s.Active < 0 || s.Active >= len(s.Hands) {
		return false
	}
	if s.Pair < NoPair || s.Pair > PerfectPair {
		return false
	}

	// No card can be on the table more often than it is in the shoe.
	var seen [cards.DeckSize]int
	count := func(dealt []cards.Card) bool {
		for _, c := range dealt {
			if !c.Valid() {
				return false
			}
			if seen[c]++; seen[c] > Decks {
				return false
			}
		}
		return true
	}
	if !count(s.Dealer) {
		return false
	}

	for i, h := range s.Hands {
		if !count(h.Cards) {
			return false
		}
		total, _ := Total(h.Cards)
		switch {
		case i < s.Active:
			// Played already: finished, at its own bet or doubled.
			if !h.Done || len(h.Cards) < 2 || h.Doubled != (h.Bet == 2*s.Stake) {
				return false
			}
			if !h.Doubled && h.Bet != s.Stake {
				return false
			}
		case i == s.Active:
			// Being played: at least two cards, and room for another.
			if h.Done || h.Doubled || h.Bet != s.Stake || len(h.Cards) < 2 || total >= 21 {
				return false
			}
		default:
			// Waiting its turn after a split, with the one card it began with.
			if h.Done || h.Doubled || h.Bet != s.Stake || len(h.Cards) != 1 {
				return false
			}
		}
	}
	return true
}
