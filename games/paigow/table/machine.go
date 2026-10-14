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

// MaxBet is the most that can be bet on a hand.
const MaxBet = 500_000 // $5,000

// Result describes a hand once it is over.
type Result struct {
	Hand   [HandSize]cards.Card
	Player Setting
	Dealer [HandSize]cards.Card
	House  Setting // the dealer's hand, set the house way

	HighWon, LowWon bool // which of the player's hands beat the dealer's
	Outcome         Outcome

	Bet        int
	Paid       int // what the bet was paid, itself included
	Commission int // what the house kept of a win
}

// Machine is a Pai Gow Poker table. A bet is made and seven cards dealt;
// the player sets them as a high hand of five and a low hand of two, and
// plays them against the dealer's seven, set the house way.
type Machine struct {
	Bank *bank.Bank
	Chip int // the chip that is put down or taken off

	// Bet is what is bet on the next hand, and Last the bet of the last.
	Bet  int
	Last int

	// Dealt is set from the deal until the hand is played: the bet has been
	// taken and the hand is waiting to be set. Stake is the bet it was
	// dealt for and Hand the player's seven cards.
	//
	// The dealer's cards are not dealt until the player has set the hand,
	// so there is never a hidden card to be learnt: they are drawn then
	// from the rest of the deck, which comes to the same thing as having
	// dealt them face down.
	Dealt bool
	Stake int
	Hand  [HandSize]cards.Card

	// Ranker ranks the hands. Its zero value does.
	Ranker Ranker

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

// draw deals cards into hand at random from a deck of 53 that the given
// cards have already left. Each hand is dealt from a whole deck, freshly
// shuffled.
func draw(rng *rand.Rand, next *[]cards.Card, hand []cards.Card, gone ...cards.Card) {
	var out [DeckSize]bool
	for _, c := range gone {
		out[c] = true
	}
	left := DeckSize - len(gone)
	for i := range hand {
		if len(*next) > 0 {
			hand[i], *next = (*next)[0], (*next)[1:]
		} else {
			// The nth of the cards still in the deck.
			n := rng.IntN(left)
			for c := range cards.Card(DeckSize) {
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

// Place puts a chip on the bet. It reports false and puts nothing down if
// a hand is waiting, if the bet would then be more than it may, or if the
// bank could not cover it.
func (m *Machine) Place() bool {
	bet := m.Bet + m.Chip
	if m.Dealt || bet > MaxBet || bet > m.Bank.Credits {
		return false
	}
	m.Bet = bet
	return true
}

// Remove takes a chip off the bet, or what is there if that is less. It
// reports false if nothing was bet, or a hand is waiting.
func (m *Machine) Remove() bool {
	if m.Dealt || m.Bet == 0 {
		return false
	}
	m.Bet -= min(m.Chip, m.Bet)
	return true
}

// Clear takes the chips off the table.
func (m *Machine) Clear() {
	if !m.Dealt {
		m.Bet = 0
	}
}

// Rebet puts the bet of the last hand back on the table, in place of any
// there now. It reports false and changes nothing if there was no last
// hand, a hand is waiting, or the bank could not cover it.
func (m *Machine) Rebet() bool {
	if m.Dealt || m.Last == 0 || m.Last > m.Bank.Credits {
		return false
	}
	m.Bet = m.Last
	return true
}

// CanDeal reports whether Deal would deal a hand: none is waiting, there is
// a bet, and the bank can cover it.
func (m *Machine) CanDeal() bool {
	return !m.Dealt && m.Bet > 0 && m.Bet <= m.Bank.Credits
}

// Deal takes the bet from the bank and deals the player seven cards. It
// reports false and deals nothing if CanDeal does.
func (m *Machine) Deal() bool {
	if !m.CanDeal() || !m.Bank.Take(m.Bet) {
		return false
	}
	m.stir()
	m.Stake, m.Last, m.Bet = m.Bet, m.Bet, 0
	draw(m.rng, &m.next, m.Hand[:])
	m.Dealt = true
	return true
}

// Play sets the waiting hand with the given two cards low, deals the
// dealer's seven, sets them the house way and settles the bet. It reports
// false if no hand is waiting, or the setting is not one that can be made.
func (m *Machine) Play(low [LowSize]int) (Result, bool) {
	if !m.Dealt {
		return Result{}, false
	}
	player, ok := m.Ranker.Set(m.Hand, low)
	if !ok {
		return Result{}, false
	}
	r := Result{Hand: m.Hand, Player: player, Bet: m.Stake}
	draw(m.rng, &m.next, r.Dealer[:], m.Hand[:]...)
	r.House, _ = m.Ranker.Set(r.Dealer, m.Ranker.HouseWay(r.Dealer))
	r.HighWon, r.LowWon = player.High > r.House.High, player.LowS > r.House.LowS
	r.Outcome, r.Paid, r.Commission = Settle(player.High, player.LowS, r.House.High, r.House.LowS, m.Stake)
	m.Bank.Pay(r.Paid)
	m.Dealt = false
	return r, true
}
