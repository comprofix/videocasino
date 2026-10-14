package table

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"
	"slices"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/pokerhand"
)

// The chips a player can choose from.
var ChipChoices = []int{100, 500, 2500, 10_000, 100_000} // $1, $5, $25, $100 and $1,000

// MaxBet is the most that can be bet on the ante, and so on the blind, or
// on Trips.
const MaxBet = 500_000 // $5,000

// Bet is one of the bets a player puts chips on. The blind is not among
// them: it is always the same as the ante, and goes down with it. Nor is
// the play bet, which is made by betting during the hand.
type Bet int

const (
	Trips Bet = iota
	Ante
	Bets // how many there are
)

func (b Bet) String() string {
	return [...]string{"TRIPS", "ANTE"}[b]
}

// Result describes a hand once it is over.
type Result struct {
	Hole  [HoleSize]cards.Card
	Board [BoardSize]cards.Card
	// Dealer is the dealer's two cards. They are not dealt if the player
	// folded.
	Dealer  [HoleSize]cards.Card
	Outcome Outcome

	Player      pokerhand.Score // the player's best five of seven
	DealerScore pokerhand.Score // the dealer's, if the hand was played
	Qualified   bool            // the dealer qualified

	Stake     [Bets]int // what the hand was dealt for; the blind is the ante again
	Play      int       // the play bet, if it was made
	Paid      int       // what the ante, the blind and the play bet were paid, themselves included
	TripsPaid int       // what Trips was paid, itself included
}

// Staked is what was bet on the hand in all.
func (r *Result) Staked() int {
	return r.Stake[Trips] + 2*r.Stake[Ante] + r.Play
}

// Win is what the hand was paid in all.
func (r *Result) Win() int {
	return r.Paid + r.TripsPaid
}

// Machine is an Ultimate Texas Hold'em table. Chips are put on the ante,
// which puts the same on the blind, and on Trips, and a hand is dealt. The
// player then checks or bets as the board is turned: 4 or 3 antes before
// the flop, 2 on the flop, or 1 on the river, where the hand must be bet
// or folded. A bet ends the choosing: the rest of the board is turned and
// the hand played against the dealer's.
type Machine struct {
	Bank *bank.Bank
	Chip int // the chip that is put down or taken off

	// Bets is what is bet for the next hand, and Last the bets of the last.
	Bets [Bets]int
	Last [Bets]int

	// Round is the choice the waiting hand is at, or NoHand. Stake is the
	// bets it was dealt for, Hole the player's cards, and Board the board,
	// of which as many are turned as the round shows.
	//
	// Neither the board nor the dealer's cards are dealt until they are
	// turned, so there is never a hidden card to be learnt: they are drawn
	// then from the rest of the deck, which comes to the same thing as
	// having dealt them face down.
	Round Round
	Stake [Bets]int
	Hole  [HoleSize]cards.Card
	Board [BoardSize]cards.Card

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

// dealt returns the cards dealt so far: the player's and those of the
// board that are turned.
func (m *Machine) dealt() []cards.Card {
	return append(slices.Clone(m.Hole[:]), m.Board[:m.Round.Shown()]...)
}

// Total returns what is bet on the table for the next hand, the blind
// included.
func (m *Machine) Total() int {
	return m.Bets[Trips] + 2*m.Bets[Ante]
}

// LastTotal returns what was bet on the last hand before it was dealt.
func (m *Machine) LastTotal() int {
	return m.Last[Trips] + 2*m.Last[Ante]
}

// Needed returns what the bank must hold for the given bets to be dealt:
// the bets and the blind, and the largest play bet, so that the hand can
// always be bet.
func Needed(bets [Bets]int) int {
	return bets[Trips] + bets[Ante]*(2+MaxPlay)
}

// Place puts a chip on a bet. It reports false and puts nothing down if a
// hand is waiting, if the bet would then hold more than it may, or if the
// bank could not then cover the hand.
func (m *Machine) Place(b Bet) bool {
	bets := m.Bets
	bets[b] += m.Chip
	if m.Round != NoHand || bets[b] > MaxBet || Needed(bets) > m.Bank.Credits {
		return false
	}
	m.Bets = bets
	return true
}

// Remove takes a chip off a bet, or what is there if that is less. It
// reports false if nothing was bet on it, or a hand is waiting.
func (m *Machine) Remove(b Bet) bool {
	if m.Round != NoHand || m.Bets[b] == 0 {
		return false
	}
	m.Bets[b] -= min(m.Chip, m.Bets[b])
	return true
}

// Clear takes every chip off the table.
func (m *Machine) Clear() {
	if m.Round == NoHand {
		m.Bets = [Bets]int{}
	}
}

// Rebet puts the bets of the last hand back on the table, in place of any
// there now. It reports false and changes nothing if there was no last
// hand, a hand is waiting, or the bank could not cover them.
func (m *Machine) Rebet() bool {
	if m.Round != NoHand || m.LastTotal() == 0 || Needed(m.Last) > m.Bank.Credits {
		return false
	}
	m.Bets = m.Last
	return true
}

// CanDeal reports whether Deal would deal a hand: none is waiting, there is
// an ante, and the bank can cover the bets and the largest play bet.
func (m *Machine) CanDeal() bool {
	return m.Round == NoHand && m.Bets[Ante] > 0 && Needed(m.Bets) <= m.Bank.Credits
}

// Deal takes the bets and the blind from the bank and deals the player two
// cards. It reports false and deals nothing if CanDeal does.
func (m *Machine) Deal() bool {
	if !m.CanDeal() || !m.Bank.Take(m.Total()) {
		return false
	}
	m.stir()
	m.Stake, m.Last, m.Bets = m.Bets, m.Bets, [Bets]int{}
	m.draw(m.Hole[:])
	m.Board = [BoardSize]cards.Card{}
	m.Round = Preflop
	return true
}

// Check passes on betting, and turns the next of the board: the flop
// before it, and the turn and the river on it. On the river there is no
// checking. It reports false if there is no hand at a round to check in.
func (m *Machine) Check() bool {
	switch m.Round {
	case Preflop:
		m.draw(m.Board[:FlopSize], m.dealt()...)
		m.Round = Flop
	case Flop:
		m.draw(m.Board[FlopSize:], m.dealt()...)
		m.Round = River
	default:
		return false
	}
	return true
}

// CanBet reports whether a play bet of the given number of antes can be
// made: it is one the round allows, and the bank can cover it.
func (m *Machine) CanBet(antes int) bool {
	return slices.Contains(m.Round.Bets(), antes) && antes*m.Stake[Ante] <= m.Bank.Credits
}

// result begins the account of the hand that is waiting, with the board
// all turned.
func (m *Machine) result() Result {
	r := Result{Hole: m.Hole, Board: m.Board, Stake: m.Stake}
	_, r.Player = pokerhand.Best(seven(m.Hole, m.Board))
	r.TripsPaid = TripsPaid(r.Player, m.Stake[Trips])
	return r
}

// Bet makes a play bet of so many antes, turns the rest of the board,
// deals the dealer's cards and settles the hand. It reports false if the
// bet cannot be made.
func (m *Machine) Bet(antes int) (Result, bool) {
	if !m.CanBet(antes) || !m.Bank.Take(antes*m.Stake[Ante]) {
		return Result{}, false
	}
	shown := m.Round.Shown()
	m.draw(m.Board[shown:], m.dealt()...)
	m.Round = River // all turned

	r := m.result()
	r.Play = antes * m.Stake[Ante]
	m.draw(r.Dealer[:], m.dealt()...)
	_, r.DealerScore = pokerhand.Best(seven(r.Dealer, m.Board))
	r.Qualified = Qualifies(r.DealerScore)
	r.Outcome, r.Paid = Settle(r.Player, r.DealerScore, m.Stake[Ante], m.Stake[Ante], r.Play)
	m.Bank.Pay(r.Paid + r.TripsPaid)
	m.Round = NoHand
	return r, true
}

// Fold gives up the hand on the river, and the ante and the blind with it.
// Trips is still settled on the player's cards. It reports false if no
// hand is waiting on the river.
func (m *Machine) Fold() (Result, bool) {
	if m.Round != River {
		return Result{}, false
	}
	r := m.result()
	r.Outcome = Folded
	m.Bank.Pay(r.TripsPaid)
	m.Round = NoHand
	return r, true
}
