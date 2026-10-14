package poker

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"

	"videocasino/bank"
	"videocasino/cards"
)

// CoinValues is the worth in credits of the coins a player can choose to
// bet with.
var CoinValues = []int{25, 100, 500, 2500, 10_000} // 25c, $1, $5, $25 and $100

// Result describes a hand once the draw has been made.
type Result struct {
	Hand [HandSize]cards.Card
	Made Category
	Win  int // credits won
}

// Machine is the state of a game in progress. It has no credits of its own:
// bets are taken from the bank and wins paid into it.
type Machine struct {
	Table     *Paytable
	Bank      *bank.Bank
	Coins     int // coins bet on a hand, from 1 to MaxCoins
	CoinValue int // credits to the coin: one of CoinValues

	// Hand is the cards last dealt, and Held those the player is keeping.
	// Dealt is set between the deal and the draw: the bet has been taken
	// and the hand is waiting to be played out.
	Hand  [HandSize]cards.Card
	Held  [HandSize]bool
	Dealt bool

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

// New returns a machine that plays from the given bank. The seeds fix the
// sequence of cards, which the tests rely on.
func New(b *bank.Bank, seed1, seed2 uint64) *Machine {
	var seed [32]byte
	binary.LittleEndian.PutUint64(seed[:], seed1)
	binary.LittleEndian.PutUint64(seed[8:], seed2)
	return newMachine(b, seed)
}

// NewRandom returns a machine for play: one whose cards cannot be
// predicted. Its generator is a cryptographic one, seeded by the operating
// system and seeded afresh as play goes on.
func NewRandom(b *bank.Bank) *Machine {
	m := newMachine(b, freshSeed())
	m.reseedIn = reseedEvery
	return m
}

func newMachine(b *bank.Bank, seed [32]byte) *Machine {
	src := rand.NewChaCha8(seed)
	return &Machine{
		Table:     &NineSix,
		Bank:      b,
		Coins:     1,
		CoinValue: CoinValues[0],
		src:       src,
		rng:       rand.New(src),
	}
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

// Bet is the cost of one hand in credits.
func (m *Machine) Bet() int {
	return m.Coins * m.CoinValue
}

// pick takes a card at random out of deck.
func (m *Machine) pick(deck *[]cards.Card) cards.Card {
	d := *deck
	i := m.rng.IntN(len(d))
	c := d[i]
	d[i] = d[len(d)-1]
	*deck = d[:len(d)-1]
	return c
}

// Deal takes the bet and deals five cards from a full deck, with none of
// them held. It reports what they make as they stand. It reports false and
// changes nothing if there are too few credits, or if a hand has been dealt
// and is still to be drawn to.
//
// Coins and CoinValue must not be changed between the deal and the draw:
// the hand is paid at the bet it was dealt for.
func (m *Machine) Deal() (Category, bool) {
	if m.Dealt || !m.Bank.Take(m.Bet()) {
		return Nothing, false
	}
	m.stir()

	deck := make([]cards.Card, cards.DeckSize)
	for i := range deck {
		deck[i] = cards.Card(i)
	}
	for i := range m.Hand {
		m.Hand[i] = m.pick(&deck)
	}
	m.Held = [HandSize]bool{}
	m.Dealt = true
	return Evaluate(m.Hand), true
}

// ToggleHold holds the card at the given place, or lets it go if it is
// held. It does nothing unless a hand is waiting to be drawn to.
func (m *Machine) ToggleHold(i int) {
	if m.Dealt {
		m.Held[i] = !m.Held[i]
	}
}

// Draw replaces the cards that are not held and pays what the hand then
// makes. The new cards come from the 47 that were not dealt, so a card let
// go cannot come back. They are chosen only now, so nothing about them can
// be learnt beforehand. It reports false if no hand is waiting.
func (m *Machine) Draw() (Result, bool) {
	if !m.Dealt {
		return Result{}, false
	}
	var dealt [cards.DeckSize]bool
	for _, c := range m.Hand {
		dealt[c] = true
	}
	deck := make([]cards.Card, 0, cards.DeckSize-HandSize)
	for c := range cards.Card(cards.DeckSize) {
		if !dealt[c] {
			deck = append(deck, c)
		}
	}
	for i := range m.Hand {
		if !m.Held[i] {
			m.Hand[i] = m.pick(&deck)
		}
	}

	r := Result{Hand: m.Hand, Made: Evaluate(m.Hand)}
	r.Win = m.Table.Pay(r.Made, m.Coins) * m.CoinValue
	m.Bank.Pay(r.Win)
	m.Dealt = false
	return r, true
}
