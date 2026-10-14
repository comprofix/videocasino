package wheel

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"
	"slices"

	"videocasino/bank"
)

// The chips a player can choose from.
var ChipChoices = []int{10, 50, 100, 500, 1000}

const (
	// MaxSpot is the most that can be bet on one spot.
	MaxSpot = 5000
	// HistoryLen is how many of the last numbers are remembered.
	HistoryLen = 10
)

// Machine is a roulette table. Chips are put on its spots and the wheel is
// spun: the bets are taken from the bank at the spin, and the winning ones
// paid into it.
type Machine struct {
	Bank *bank.Bank
	Chip int // the chip that is put down or taken off

	// Bets is what is bet on each spot for the next spin, in the order of
	// Spots, and Last the bets of the last spin.
	Bets []int
	Last []int

	History []int // the numbers that have come up, the newest first

	// Every chance in the game is drawn from rng, which reads from src.
	// reseedIn counts the spins until src is seeded afresh; it stays at
	// zero for a machine made with fixed seeds, which is never reseeded.
	src      *rand.ChaCha8
	rng      *rand.Rand
	reseedIn int
}

// reseedEvery is how many spins a machine made by NewRandom plays between
// fresh seeds.
const reseedEvery = 100

// New returns a table playing from the given bank, whose spins follow from
// the two seeds. It is for tests, which need the same spins every time.
func New(b *bank.Bank, seed1, seed2 uint64) *Machine {
	var seed [32]byte
	binary.LittleEndian.PutUint64(seed[:], seed1)
	binary.LittleEndian.PutUint64(seed[8:], seed2)
	return newMachine(b, seed)
}

// NewRandom returns a table playing from the given bank, seeded by the
// operating system and seeded afresh from time to time.
func NewRandom(b *bank.Bank) *Machine {
	m := newMachine(b, freshSeed())
	m.reseedIn = reseedEvery
	return m
}

func newMachine(b *bank.Bank, seed [32]byte) *Machine {
	src := rand.NewChaCha8(seed)
	return &Machine{
		Bank: b,
		Chip: ChipChoices[0],
		Bets: make([]int, len(Spots)),
		Last: make([]int, len(Spots)),
		src:  src,
		rng:  rand.New(src),
	}
}

// freshSeed asks the operating system for a seed.
func freshSeed() [32]byte {
	var seed [32]byte
	crand.Read(seed[:]) // never fails: the program stops if it cannot be read
	return seed
}

// stir counts a spin towards the next fresh seed, and takes one when it is
// due.
func (m *Machine) stir() {
	if m.reseedIn == 0 {
		return
	}
	m.reseedIn--
	if m.reseedIn == 0 {
		m.src.Seed(freshSeed())
		m.reseedIn = reseedEvery
	}
}

func sum(bets []int) int {
	total := 0
	for _, bet := range bets {
		total += bet
	}
	return total
}

// Total returns what is bet on the table for the next spin.
func (m *Machine) Total() int {
	return sum(m.Bets)
}

// LastTotal returns what was bet on the last spin.
func (m *Machine) LastTotal() int {
	return sum(m.Last)
}

// Place puts a chip on a spot. It reports false and puts nothing down if
// the spot would then hold more than it may, or the table more than the
// bank has.
func (m *Machine) Place(spot int) bool {
	if m.Bets[spot]+m.Chip > MaxSpot || m.Total()+m.Chip > m.Bank.Credits {
		return false
	}
	m.Bets[spot] += m.Chip
	return true
}

// Remove takes a chip off a spot, or what is there if that is less. It
// reports false if nothing was bet on it.
func (m *Machine) Remove(spot int) bool {
	if m.Bets[spot] == 0 {
		return false
	}
	m.Bets[spot] -= min(m.Chip, m.Bets[spot])
	return true
}

// Clear takes every chip off the table.
func (m *Machine) Clear() {
	clear(m.Bets)
}

// Rebet puts the bets of the last spin back on the table, in place of any
// there now. It reports false and changes nothing if there was no last
// spin, or the bank has too little for them.
func (m *Machine) Rebet() bool {
	if total := m.LastTotal(); total == 0 || total > m.Bank.Credits {
		return false
	}
	copy(m.Bets, m.Last)
	return true
}

// Result is the outcome of a spin.
type Result struct {
	Number int
	Bet    int // what was bet on the table
	Win    int // what was paid, with the winning bets themselves handed back
}

// Spin takes the bets from the bank, spins the wheel and pays the winning
// bets. The table is then clear, and the bets are remembered as the last
// ones. It reports false and spins nothing if nothing is bet or the bank
// has too little.
func (m *Machine) Spin() (Result, bool) {
	total := m.Total()
	if total == 0 || !m.Bank.Take(total) {
		return Result{}, false
	}
	r := Result{Number: m.rng.IntN(Pockets), Bet: total}
	r.Win = Paid(m.Bets, r.Number)
	m.Bank.Pay(r.Win)

	m.Bets, m.Last = m.Last, m.Bets
	clear(m.Bets)
	m.History = slices.Insert(m.History, 0, r.Number)
	m.History = m.History[:min(len(m.History), HistoryLen)]
	m.stir()
	return r, true
}
