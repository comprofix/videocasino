package table

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"

	"videocasino/bank"
	"videocasino/cards"

	// The odds have a shoe of their own, counted by value, so the package
	// goes by another name here.
	cardshoe "videocasino/shoe"
)

// BetChoices is the bets a player can choose from. They are all even, so
// that a blackjack, which pays 3 to 2, always pays whole credits.
var BetChoices = []int{10, 20, 50, 100, 500}

// SideChoices is the Perfect Pairs bets a player can choose from. Nothing
// is bet on it at 0.
var SideChoices = []int{0, 5, 10, 25, 50}

// Hand is one of the player's hands: the one dealt, or one of the two it
// was split into.
type Hand struct {
	Cards   []cards.Card
	Bet     int  // credits staked on it: twice the bet once it is doubled
	Doubled bool // it was doubled, and took one card for it
	Done    bool // it can take no more cards
}

// Total is what the hand adds up to, and whether it is soft.
func (h *Hand) Total() (total int, soft bool) {
	return Total(h.Cards)
}

// Bust reports whether the hand has gone past 21.
func (h *Hand) Bust() bool {
	total, _ := h.Total()
	return total > 21
}

// Outcome is how a hand ended.
type Outcome int

const (
	Lost Outcome = iota
	Pushed
	Won
	WonBlackjack // 21 in the two cards first dealt, paid 3 to 2
)

// Result describes a hand once it has been settled.
type Result struct {
	Dealer          []cards.Card
	DealerBlackjack bool
	Outcomes        []Outcome // one for each of the player's hands
	Staked          int       // credits staked on the hands
	Paid            int       // credits paid back for them, stakes included
}

// Machine is the state of a game in progress. It has no credits of its own:
// bets are taken from the bank and wins paid into it.
type Machine struct {
	Bank    *bank.Bank
	Bet     int // credits to be bet on the next hand: one of BetChoices
	SideBet int // credits to be bet on Perfect Pairs: one of SideChoices

	// Dealt is set from the deal until the hand is settled: the bets have
	// been taken and the hand is waiting to be played out. Stake and Side
	// are the bets it was dealt for, which Bet and SideBet may since have
	// moved away from.
	Dealt bool
	Stake int
	Side  int

	// Dealer is the dealer's cards. While the player is playing there is
	// only the one that shows: the dealer takes the rest when the player
	// has finished, so there is never a hidden card to be learnt.
	Dealer []cards.Card

	// Hands is the player's hands: one, or two after a split. Active is
	// the one being played. They stay as they ended until the next deal.
	Hands  []Hand
	Active int

	// Pair is what the first two cards made for Perfect Pairs, and SideWin
	// what that paid on top of the side bet. It is settled at the deal.
	Pair    PairKind
	SideWin int

	// Last is how the hand last played was settled.
	Last Result

	// The shoe the hands are dealt from, one after another, so a card that
	// has been played stays out until the next shuffle.
	//
	// Once the cut card has come out, the hand being played is finished and
	// the shoe is shuffled before the next. Shuffled is set while the hand
	// on the table is the first from a fresh shoe.
	cardshoe.Shoe
	Shuffled bool

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
	m := &Machine{
		Bank: b,
		Bet:  BetChoices[0],
		Shoe: cardshoe.New(Decks),
		src:  src,
		rng:  rand.New(src),
	}
	m.shuffle()
	return m
}

// The cut card is put in at random between two thirds and five sixths of
// the way through the shoe, so between one deck and two are never dealt.
const (
	cutMin = ShoeSize * 2 / 3
	cutMax = ShoeSize * 5 / 6
)

// shuffle fills the shoe again and puts the cut card in.
func (m *Machine) shuffle() {
	m.Fill(cutMin + m.rng.IntN(cutMax-cutMin+1))
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

// draw takes a card at random from what is left in the shoe. It is chosen
// only now, so nothing about it can be learnt beforehand.
func (m *Machine) draw() cards.Card {
	return m.Draw(m.rng)
}

// hand returns the hand being played, or nil if there is none.
func (m *Machine) hand() *Hand {
	if !m.Dealt || m.Active >= len(m.Hands) {
		return nil
	}
	return &m.Hands[m.Active]
}

// Deal takes the bet and the side bet, deals two cards to the player and
// one to the dealer, and settles Perfect Pairs straight away. If the cut
// card came out during the last hand the shoe is shuffled first. A blackjack
// needs no playing, so the hand is settled at once too. It reports false
// and changes nothing if there are too few credits, or if a hand has been
// dealt and is still to be played out.
func (m *Machine) Deal() bool {
	if m.Dealt || !m.Bank.Take(m.Bet+m.SideBet) {
		return false
	}
	m.stir()
	if m.CutOut() {
		m.shuffle()
	}
	m.Shuffled = m.CardsLeft() == ShoeSize
	m.Stake, m.Side = m.Bet, m.SideBet
	m.Dealer, m.Hands, m.Active = nil, nil, 0
	m.Last = Result{}
	m.Dealt = true

	first := m.draw()
	m.Hands = []Hand{{Cards: []cards.Card{first}, Bet: m.Stake}}
	m.Dealer = []cards.Card{m.draw()}
	second := m.draw()
	m.Hands[0].Cards = append(m.Hands[0].Cards, second)

	m.Pair, m.SideWin = PairOf(first, second), 0
	if m.Side > 0 && m.Pair != NoPair {
		m.SideWin = m.Side * PairPays[m.Pair]
		m.Bank.Pay(m.Side + m.SideWin)
	}

	if IsBlackjack(m.Hands[0].Cards) {
		m.Hands[0].Done = true
	}
	m.advance()
	return true
}

// Hit deals another card to the hand being played. A hand that reaches 21
// or goes bust is finished. It reports false if no hand is being played.
func (m *Machine) Hit() bool {
	h := m.hand()
	if h == nil {
		return false
	}
	h.Cards = append(h.Cards, m.draw())
	if total, _ := h.Total(); total >= 21 {
		h.Done = true
		m.advance()
	}
	return true
}

// Stand finishes the hand being played as it is. It reports false if no
// hand is being played.
func (m *Machine) Stand() bool {
	h := m.hand()
	if h == nil {
		return false
	}
	h.Done = true
	m.advance()
	return true
}

// CanDouble reports whether the hand being played can be doubled: its
// first two cards make a hard 9, 10 or 11, and the bank can cover the bet a
// second time. A hand can be doubled after a split.
func (m *Machine) CanDouble() bool {
	h := m.hand()
	return h != nil && CanDoubleOn(h.Cards) && m.Bank.Credits >= m.Stake
}

// Double takes the bet a second time and deals the hand one more card,
// which finishes it. It reports false and changes nothing if the hand
// cannot be doubled.
func (m *Machine) Double() bool {
	if !m.CanDouble() || !m.Bank.Take(m.Stake) {
		return false
	}
	h := m.hand()
	h.Bet += m.Stake
	h.Doubled = true
	h.Cards = append(h.Cards, m.draw())
	h.Done = true
	m.advance()
	return true
}

// CanSplit reports whether the hand can be split: it is the hand first
// dealt, its two cards count the same, and the bank can cover the bet a
// second time. Any two cards worth ten can be split, such as a king and a
// queen. A hand can be split only once.
func (m *Machine) CanSplit() bool {
	h := m.hand()
	return h != nil && len(m.Hands) == 1 && len(h.Cards) == 2 &&
		Value(h.Cards[0]) == Value(h.Cards[1]) && m.Bank.Credits >= m.Stake
}

// Split takes the bet a second time and makes two hands of the two cards,
// each to be played in turn. Split aces are dealt one card each and that
// is all they get. It reports false and changes nothing if the hand cannot
// be split.
func (m *Machine) Split() bool {
	if !m.CanSplit() || !m.Bank.Take(m.Stake) {
		return false
	}
	pair := m.Hands[0].Cards
	m.Hands = []Hand{
		{Cards: []cards.Card{pair[0]}, Bet: m.Stake},
		{Cards: []cards.Card{pair[1]}, Bet: m.Stake},
	}
	m.Active = 0
	if Value(pair[0]) == 1 {
		for i := range m.Hands {
			h := &m.Hands[i]
			h.Cards = append(h.Cards, m.draw())
			h.Done = true
		}
	}
	m.advance()
	return true
}

// advance moves on to the next hand that can still take cards. A split
// hand is dealt its second card when its turn comes. When there are no
// hands left to play the dealer plays and everything is settled.
func (m *Machine) advance() {
	for m.Active < len(m.Hands) {
		h := &m.Hands[m.Active]
		if len(h.Cards) == 1 {
			h.Cards = append(h.Cards, m.draw())
			if total, _ := h.Total(); total == 21 {
				h.Done = true
			}
		}
		if !h.Done {
			return
		}
		m.Active++
	}
	m.finish()
}

// finish plays the dealer's hand and settles the player's.
//
// The dealer has no hidden card, so a dealer's blackjack can only turn up
// after the player has played. It costs the player the bet first made and
// no more: whatever was added by doubling or splitting is returned, even on
// a hand that went bust. That leaves the player exactly as well off as when
// the dealer looks for a blackjack before the player plays.
func (m *Machine) finish() {
	natural := len(m.Hands) == 1 && IsBlackjack(m.Hands[0].Cards)
	staked, alive := 0, false
	for i := range m.Hands {
		staked += m.Hands[i].Bet
		alive = alive || !m.Hands[i].Bust()
	}
	up := Value(m.Dealer[0])
	couldBlackjack := up == 1 || up == 10

	switch {
	case natural:
		// Only a blackjack of the dealer's own can matter.
		if couldBlackjack {
			m.Dealer = append(m.Dealer, m.draw())
		}
	case !alive:
		// Every hand is lost already. The dealer draws only if a blackjack
		// would give something back.
		if couldBlackjack && staked > m.Stake {
			m.Dealer = append(m.Dealer, m.draw())
		}
	default:
		for {
			if total, _ := Total(m.Dealer); total >= DealerStands {
				break
			}
			m.Dealer = append(m.Dealer, m.draw())
		}
	}

	r := Result{
		Dealer:          m.Dealer,
		DealerBlackjack: IsBlackjack(m.Dealer),
		Outcomes:        make([]Outcome, len(m.Hands)),
		Staked:          staked,
	}
	dealer, _ := Total(m.Dealer)
	switch {
	case natural && r.DealerBlackjack:
		r.Outcomes[0] = Pushed
		r.Paid = m.Stake
	case natural:
		r.Outcomes[0] = WonBlackjack
		r.Paid = m.Stake + m.Stake*3/2
	case r.DealerBlackjack:
		// Every hand loses, but only the first bet is kept.
		r.Paid = staked - m.Stake
	default:
		for i := range m.Hands {
			h := &m.Hands[i]
			total, _ := h.Total()
			switch {
			case total > 21:
				r.Outcomes[i] = Lost
			case dealer > 21 || total > dealer:
				r.Outcomes[i] = Won
				r.Paid += 2 * h.Bet
			case total == dealer:
				r.Outcomes[i] = Pushed
				r.Paid += h.Bet
			}
		}
	}

	m.Bank.Pay(r.Paid)
	m.Last = r
	m.Dealt = false
}
