package table

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"
	"slices"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/shoe"
)

// The chips a player can choose from. They are all even, so that a bet on
// the banker's hand that is paid half is always paid whole credits.
var ChipChoices = []int{10, 50, 100, 500, 1000}

const (
	// MaxBet is the most that can be bet on any one of the bets.
	MaxBet = 5000
	// HistoryLen is how many of the last hands are remembered.
	HistoryLen = 16
)

// Result describes a hand that has been dealt and settled.
type Result struct {
	Player, Banker []cards.Card
	Outcome        Outcome

	Bets [Bets]int // what was bet
	Paid [Bets]int // what each bet was paid, with itself handed back
	Bet  int       // what was bet in all
	Win  int       // what was paid in all

	// Burn is the card turned up after a shuffle and Burnt how many cards
	// were then burnt for it, if this hand was the first from a new shoe:
	// Burnt is nought if it was not.
	Burn  cards.Card
	Burnt int

	// CutAfter is how many of the hand's cards had been dealt when the cut
	// card came out, counted in the order they were dealt, or -1 if it did
	// not come out during the hand.
	CutAfter int
}

// Order returns the cards of a hand in the order they were dealt: the
// player's first, the banker's first, and the same again, then the third
// cards.
func (r *Result) Order() []cards.Card {
	var order []cards.Card
	for i := range 3 {
		if i < len(r.Player) {
			order = append(order, r.Player[i])
		}
		if i < len(r.Banker) {
			order = append(order, r.Banker[i])
		}
	}
	return order
}

// Machine is a baccarat table. Chips are put on its bets and a hand is
// dealt: the bets are taken from the bank at the deal, and the winning ones
// paid into it. Nobody plays the hands, so a hand is dealt and settled all
// at once.
type Machine struct {
	Bank *bank.Bank
	Chip int // the chip that is put down or taken off

	// Bets is what is bet for the next hand, and Last the bets of the last.
	Bets [Bets]int
	Last [Bets]int

	// History is how the hands dealt from this shoe went, the newest first.
	History []Outcome

	// The shoe the hands are dealt from. After a shuffle the first card is
	// turned up and as many cards burnt as it counts, before the first hand
	// is dealt. When the cut card comes out the hand is finished and one
	// more dealt, and then the shoe is shuffled.
	//
	// Fresh is set while the shoe is full, with its cards still to be
	// burnt, and Finished once its last hand has been dealt.
	shoe.Shoe
	Fresh    bool
	Finished bool

	// Every card is drawn from rng, which reads from src. reseedIn counts
	// the hands until src is seeded afresh; it stays at zero for a machine
	// made with fixed seeds, which is never reseeded.
	src      *rand.ChaCha8
	rng      *rand.Rand
	reseedIn int
}

// reseedEvery is how many hands a machine made by NewRandom plays between
// fresh seeds.
const reseedEvery = 100

// New returns a table that plays from the given bank. The seeds fix the
// sequence of cards, which the tests rely on.
func New(b *bank.Bank, seed1, seed2 uint64) *Machine {
	var seed [32]byte
	binary.LittleEndian.PutUint64(seed[:], seed1)
	binary.LittleEndian.PutUint64(seed[8:], seed2)
	return newMachine(b, seed)
}

// NewRandom returns a table for play: one whose cards cannot be predicted.
// Its generator is a cryptographic one, seeded by the operating system and
// seeded afresh as play goes on.
func NewRandom(b *bank.Bank) *Machine {
	m := newMachine(b, freshSeed())
	m.reseedIn = reseedEvery
	return m
}

func newMachine(b *bank.Bank, seed [32]byte) *Machine {
	src := rand.NewChaCha8(seed)
	m := &Machine{
		Bank: b,
		Chip: ChipChoices[0],
		Shoe: shoe.New(Decks),
		src:  src,
		rng:  rand.New(src),
	}
	m.shuffle()
	return m
}

// shuffle fills the shoe again and puts the cut card in. The hands of the
// old shoe are forgotten, as they say nothing of the new one.
func (m *Machine) shuffle() {
	m.Fill(ShoeSize - CutBehind)
	m.Fresh, m.Finished = true, false
	m.History = nil
}

// freshSeed asks the operating system for a seed.
func freshSeed() [32]byte {
	var seed [32]byte
	crand.Read(seed[:]) // never fails: the program stops if it cannot be read
	return seed
}

// stir counts a hand towards the next fresh seed, and takes one when it is
// due.
func (m *Machine) stir() {
	if m.reseedIn == 0 {
		return
	}
	if m.reseedIn--; m.reseedIn == 0 {
		m.src.Seed(freshSeed())
		m.reseedIn = reseedEvery
	}
}

func sum(bets [Bets]int) int {
	total := 0
	for _, bet := range bets {
		total += bet
	}
	return total
}

// Total returns what is bet on the table for the next hand.
func (m *Machine) Total() int {
	return sum(m.Bets)
}

// LastTotal returns what was bet on the last hand.
func (m *Machine) LastTotal() int {
	return sum(m.Last)
}

// Place puts a chip on a bet. It reports false and puts nothing down if
// the bet would then hold more than it may, or the table more than the bank
// has.
func (m *Machine) Place(b Bet) bool {
	if m.Bets[b]+m.Chip > MaxBet || m.Total()+m.Chip > m.Bank.Credits {
		return false
	}
	m.Bets[b] += m.Chip
	return true
}

// Remove takes a chip off a bet, or what is there if that is less. It
// reports false if nothing was bet on it.
func (m *Machine) Remove(b Bet) bool {
	if m.Bets[b] == 0 {
		return false
	}
	m.Bets[b] -= min(m.Chip, m.Bets[b])
	return true
}

// Clear takes every chip off the table.
func (m *Machine) Clear() {
	m.Bets = [Bets]int{}
}

// Rebet puts the bets of the last hand back on the table, in place of any
// there now. It reports false and changes nothing if there was no last
// hand, or the bank has too little for them.
func (m *Machine) Rebet() bool {
	if total := m.LastTotal(); total == 0 || total > m.Bank.Credits {
		return false
	}
	m.Bets = m.Last
	return true
}

// Deal takes the bets from the bank, deals a hand and pays the winning
// bets. If the last hand of the shoe has been dealt the shoe is shuffled
// first, and the first cards of a new shoe are burnt. The table is then
// clear, and the bets are remembered as the last ones. It reports false and
// deals nothing if nothing is bet or the bank has too little.
func (m *Machine) Deal() (Result, bool) {
	total := m.Total()
	if total == 0 || !m.Bank.Take(total) {
		return Result{}, false
	}
	m.stir()
	if m.Finished {
		m.shuffle()
	}
	draw := func() cards.Card { return m.Draw(m.rng) }

	r := Result{Bets: m.Bets, Bet: total}
	if m.Fresh {
		r.Burn = draw()
		r.Burnt = BurnCount(r.Burn)
		for range r.Burnt {
			draw()
		}
		m.Fresh = false
	}

	// If the cut card is out already, this is the one hand more.
	last, toCut := m.CutOut(), m.ToCut
	r.Player, r.Banker = Play(draw)
	r.Outcome = OutcomeOf(r.Player, r.Banker)
	r.CutAfter = -1
	if !last && m.CutOut() {
		r.CutAfter = toCut
	}
	m.Finished = last

	r.Paid = Paid(m.Bets, r.Player, r.Banker)
	r.Win = sum(r.Paid)
	m.Bank.Pay(r.Win)

	m.Last, m.Bets = m.Bets, [Bets]int{}
	m.History = slices.Insert(m.History, 0, r.Outcome)
	m.History = m.History[:min(len(m.History), HistoryLen)]
	return r, true
}
