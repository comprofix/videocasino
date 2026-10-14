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

// MaxBet is the most that can be bet on the ante.
const MaxBet = 500_000 // $5,000

// Result describes a hand once it is over.
type Result struct {
	Player Hand
	// Dealer is the dealer's hand, with the card that was face up last.
	// Only that card is dealt if the player folded.
	Dealer  Hand
	Outcome Outcome

	Ante  int // what the hand was dealt for
	Raise int // the raise, if it was made
	Paid  int // what the ante and the raise were paid, themselves included
}

// Staked is what was bet on the hand in all.
func (r *Result) Staked() int {
	return r.Ante + r.Raise
}

// Machine is a Caribbean Stud table. Chips are put on the ante and a hand
// is dealt, with one of the dealer's cards face up; the player then raises,
// for twice the ante, and plays against the dealer's hand, or folds.
type Machine struct {
	Bank *bank.Bank
	Chip int // the chip that is put down or taken off

	// Ante is what is bet for the next hand, and Last the bet of the last.
	Ante int
	Last int

	// Dealt is set from the deal until the player raises or folds: the ante
	// has been taken and the hand is waiting. Stake is the ante it was dealt
	// for, Hand the player's cards and Up the dealer's card that is face up.
	//
	// The dealer's other cards are not dealt until the player has decided,
	// so there is never a hidden card to be learnt: they are drawn then from
	// the rest of the deck, which comes to the same thing as having dealt
	// them face down.
	Dealt bool
	Stake int
	Hand  Hand
	Up    cards.Card

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

// draw deals cards into hand at random from a deck that the given cards
// have already left. Each hand is dealt from a whole deck, freshly
// shuffled.
func (m *Machine) draw(hand []cards.Card, gone ...cards.Card) {
	var out [cards.DeckSize]bool
	for _, c := range gone {
		out[c] = true
	}
	left := cards.DeckSize - len(gone)
	for i := range hand {
		if len(m.next) > 0 {
			hand[i], m.next = m.next[0], m.next[1:]
		} else {
			// The nth of the cards still in the deck.
			n := m.rng.IntN(left)
			for c := range cards.Card(cards.DeckSize) {
				if out[c] {
					continue
				}
				if n == 0 {
					hand[i] = c
					break
				}
				n--
			}
		}
		out[hand[i]] = true
		left--
	}
}

// Needed returns what the bank must hold for an ante to be dealt: the ante
// itself, and the raise to follow, so that the hand can always be raised.
func Needed(ante int) int {
	return ante * (1 + RaiseSize)
}

// Place puts a chip on the ante. It reports false and puts nothing down if
// a hand is waiting to be played, if the ante would then be more than it
// may, or if the bank could not then cover the hand.
func (m *Machine) Place() bool {
	ante := m.Ante + m.Chip
	if m.Dealt || ante > MaxBet || Needed(ante) > m.Bank.Credits {
		return false
	}
	m.Ante = ante
	return true
}

// Remove takes a chip off the ante, or what is there if that is less. It
// reports false if nothing was bet, or a hand is waiting.
func (m *Machine) Remove() bool {
	if m.Dealt || m.Ante == 0 {
		return false
	}
	m.Ante -= min(m.Chip, m.Ante)
	return true
}

// Clear takes the chips off the table.
func (m *Machine) Clear() {
	if !m.Dealt {
		m.Ante = 0
	}
}

// Rebet puts the bet of the last hand back on the table, in place of any
// there now. It reports false and changes nothing if there was no last
// hand, a hand is waiting, or the bank could not cover it.
func (m *Machine) Rebet() bool {
	if m.Dealt || m.Last == 0 || Needed(m.Last) > m.Bank.Credits {
		return false
	}
	m.Ante = m.Last
	return true
}

// CanDeal reports whether Deal would deal a hand: none is waiting, there is
// an ante, and the bank can cover it and the raise.
func (m *Machine) CanDeal() bool {
	return !m.Dealt && m.Ante > 0 && Needed(m.Ante) <= m.Bank.Credits
}

// Deal takes the ante from the bank and deals the player a hand, and the
// dealer the card that is face up. It reports false and deals nothing if
// CanDeal does.
func (m *Machine) Deal() bool {
	if !m.CanDeal() || !m.Bank.Take(m.Ante) {
		return false
	}
	m.stir()
	m.Stake, m.Last, m.Ante = m.Ante, m.Ante, 0
	m.draw(m.Hand[:])
	var up [1]cards.Card
	m.draw(up[:], m.Hand[:]...)
	m.Up = up[0]
	m.Dealt = true
	return true
}

// result begins the account of the hand that is waiting.
func (m *Machine) result() Result {
	r := Result{Player: m.Hand, Ante: m.Stake}
	r.Dealer[HandSize-1] = m.Up
	return r
}

// Raise makes the raise, deals the rest of the dealer's hand and settles
// the ante and the raise against it. It reports false if no hand is
// waiting, or the bank cannot cover the raise.
func (m *Machine) Raise() (Result, bool) {
	raise := m.Stake * RaiseSize
	if !m.Dealt || !m.Bank.Take(raise) {
		return Result{}, false
	}
	r := m.result()
	r.Raise = raise
	m.draw(r.Dealer[:HandSize-1], append(m.Hand[:], m.Up)...)
	var paid int
	r.Outcome, paid = Settle(r.Player.Score(), r.Dealer.Score())
	r.Paid = paid * m.Stake
	m.Bank.Pay(r.Paid)
	m.Dealt = false
	return r, true
}

// Fold gives up the hand that is waiting, and the ante with it. It reports
// false if no hand is waiting.
func (m *Machine) Fold() (Result, bool) {
	if !m.Dealt {
		return Result{}, false
	}
	r := m.result()
	r.Outcome = Folded
	m.Dealt = false
	return r, true
}
