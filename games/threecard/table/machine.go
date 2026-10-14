package table

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"

	"videocasino/bank"
	"videocasino/cards"
)

// The chips a player can choose from.
var ChipChoices = []int{100, 500, 2500, 10_000, 100_000} // $1, $5, $25, $100 and $1,000

// MaxBet is the most that can be bet on the ante, or on Pair Plus.
const MaxBet = 500_000 // $5,000

// Bet is one of the bets a player puts chips on. The play bet is not among
// them: it is made by playing the hand, and is always the same as the ante.
type Bet int

const (
	PairPlus Bet = iota
	Ante
	Bets // how many there are
)

func (b Bet) String() string {
	return [...]string{"PAIR PLUS", "ANTE"}[b]
}

// Result describes a hand once it is over.
type Result struct {
	Player  Hand
	Dealer  Hand // not dealt if the player folded
	Outcome Outcome

	Stake    [Bets]int // what the hand was dealt for
	Play     int       // the play bet, if it was made
	PairPaid int       // what Pair Plus was paid, itself included
	Paid     int       // what the ante and the play bet were paid, themselves included
}

// Staked is what was bet on the hand in all.
func (r *Result) Staked() int {
	return r.Stake[PairPlus] + r.Stake[Ante] + r.Play
}

// Win is what the hand was paid in all.
func (r *Result) Win() int {
	return r.PairPaid + r.Paid
}

// Machine is a Three Card Poker table. Chips are put on the ante and on
// Pair Plus and a hand is dealt; the player then plays it against the
// dealer's, for a second bet the size of the ante, or folds it.
type Machine struct {
	Bank *bank.Bank
	Chip int // the chip that is put down or taken off

	// Bets is what is bet for the next hand, and Last the bets of the last.
	Bets [Bets]int
	Last [Bets]int

	// Dealt is set from the deal until the player plays or folds: the bets
	// have been taken and the hand is waiting. Stake is the bets it was
	// dealt for and Hand the player's cards.
	//
	// The dealer's cards are not dealt until the player has decided, so
	// there is never a hidden card to be learnt: they are drawn then from
	// the rest of the deck, which comes to the same thing as having dealt
	// them face down.
	Dealt bool
	Stake [Bets]int
	Hand  Hand

	// PairPaid is what Pair Plus was paid for the hand, itself included. It
	// is settled at the deal, on the player's own cards.
	PairPaid int

	// next is cards to be dealt before any are drawn at random. Only the
	// tests use it.
	next []cards.Card

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
	return &Machine{
		Bank: b,
		Chip: ChipChoices[0],
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

// draw deals a hand at random from a deck that the given cards have
// already left. Each hand is dealt from a whole deck, freshly shuffled.
func (m *Machine) draw(gone ...cards.Card) (h Hand) {
	var out [cards.DeckSize]bool
	for _, c := range gone {
		out[c] = true
	}
	left := cards.DeckSize - len(gone)
	for i := range h {
		if len(m.next) > 0 {
			h[i], m.next = m.next[0], m.next[1:]
		} else {
			// The nth of the cards still in the deck.
			n := m.rng.IntN(left)
			for c := range cards.Card(cards.DeckSize) {
				if out[c] {
					continue
				}
				if n == 0 {
					h[i] = c
					break
				}
				n--
			}
		}
		out[h[i]] = true
		left--
	}
	return h
}

// Total returns what is bet on the table for the next hand.
func (m *Machine) Total() int {
	return m.Bets[PairPlus] + m.Bets[Ante]
}

// LastTotal returns what was bet on the last hand, before it was played.
func (m *Machine) LastTotal() int {
	return m.Last[PairPlus] + m.Last[Ante]
}

// Needed returns what the bank must hold for the given bets to be dealt:
// the bets themselves, and the ante over again, so that the hand can always
// be played.
func Needed(bets [Bets]int) int {
	return bets[PairPlus] + 2*bets[Ante]
}

// Place puts a chip on a bet. It reports false and puts nothing down if a
// hand is waiting to be played, if the bet would then hold more than it
// may, or if the bank could not then cover the hand.
func (m *Machine) Place(b Bet) bool {
	bets := m.Bets
	bets[b] += m.Chip
	if m.Dealt || bets[b] > MaxBet || Needed(bets) > m.Bank.Credits {
		return false
	}
	m.Bets = bets
	return true
}

// Remove takes a chip off a bet, or what is there if that is less. It
// reports false if nothing was bet on it, or a hand is waiting.
func (m *Machine) Remove(b Bet) bool {
	if m.Dealt || m.Bets[b] == 0 {
		return false
	}
	m.Bets[b] -= min(m.Chip, m.Bets[b])
	return true
}

// Clear takes every chip off the table.
func (m *Machine) Clear() {
	if !m.Dealt {
		m.Bets = [Bets]int{}
	}
}

// Rebet puts the bets of the last hand back on the table, in place of any
// there now. It reports false and changes nothing if there was no last
// hand, a hand is waiting, or the bank could not cover them.
func (m *Machine) Rebet() bool {
	if m.Dealt || m.LastTotal() == 0 || Needed(m.Last) > m.Bank.Credits {
		return false
	}
	m.Bets = m.Last
	return true
}

// Deal takes the bets from the bank and deals the player a hand, and
// settles Pair Plus on it straight away. It reports false and deals nothing
// if a hand is waiting, if there is no ante, or if the bank could not cover
// the bets and the play bet to follow.
func (m *Machine) Deal() bool {
	if m.Dealt || m.Bets[Ante] == 0 || Needed(m.Bets) > m.Bank.Credits || !m.Bank.Take(m.Total()) {
		return false
	}
	m.stir()
	m.Stake, m.Last, m.Bets = m.Bets, m.Bets, [Bets]int{}
	m.Hand = m.draw()
	m.Dealt = true

	m.PairPaid = 0
	if pays := PairPlusPays[m.Hand.Category()]; pays > 0 {
		m.PairPaid = m.Stake[PairPlus] * (pays + 1)
		m.Bank.Pay(m.PairPaid)
	}
	return true
}

// result begins the account of the hand that is waiting.
func (m *Machine) result() Result {
	return Result{Player: m.Hand, Stake: m.Stake, PairPaid: m.PairPaid}
}

// Play makes the play bet, deals the dealer's hand and settles the ante and
// the play bet against it. It reports false if no hand is waiting.
func (m *Machine) Play() (Result, bool) {
	ante := m.Stake[Ante]
	if !m.Dealt || !m.Bank.Take(ante) {
		return Result{}, false
	}
	r := m.result()
	r.Play = ante
	r.Dealer = m.draw(m.Hand[:]...)
	var paid int
	r.Outcome, paid = Settle(r.Player.Score(), r.Dealer.Score())
	r.Paid = paid * ante
	m.Bank.Pay(r.Paid)
	m.Dealt = false
	return r, true
}

// Fold gives up the hand that is waiting, and the ante with it. Whatever
// Pair Plus paid at the deal is kept. It reports false if no hand is
// waiting.
func (m *Machine) Fold() (Result, bool) {
	if !m.Dealt {
		return Result{}, false
	}
	r := m.result()
	r.Outcome = Folded
	m.Dealt = false
	return r, true
}
