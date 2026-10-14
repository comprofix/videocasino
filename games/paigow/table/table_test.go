package table

import (
	"strings"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/pokerhand"
)

// deck makes cards from names such as "As Kd 4c", with "Jk" the joker.
func deck(t *testing.T, names string) []cards.Card {
	t.Helper()
	var cs []cards.Card
	for _, f := range strings.Fields(names) {
		if f == "Jk" {
			cs = append(cs, Joker)
			continue
		}
		r := strings.IndexByte("23456789TJQKA", f[0])
		s := strings.IndexByte("cdhs", f[1])
		if len(f) != 2 || r < 0 || s < 0 {
			t.Fatalf("%q is not a card", f)
		}
		cs = append(cs, cards.New(cards.Two+cards.Rank(r), cards.Suit(s)))
	}
	return cs
}

func hand7(t *testing.T, names string) [HandSize]cards.Card {
	t.Helper()
	return [HandSize]cards.Card(deck(t, names))
}

func TestHigh(t *testing.T) {
	tests := []struct {
		hand string
		name string
	}{
		{"As Ad Ah Ac Jk", "FIVE ACES"},
		{"Jk Ks Qs Js Ts", "ROYAL FLUSH"},
		{"Jk Kd 9h 7c 5s", "ACE KING HIGH"},
		{"Jk Ad 9h 7c 5s", "PAIR OF ACES"},
		{"Jk 2h 3c 4d 5s", "STRAIGHT TO THE SIX"},
		{"Jk 2h 9h 5h Kh", "FLUSH, ACE HIGH"},
		{"Jk Ah 9h 5h Kh", "FLUSH, ACE HIGH"},
		{"Jk Kd Kh Ks Kc", "FOUR KINGS"},
		{"Jk 9s 8s 7s 6s", "STRAIGHT FLUSH TO THE TEN"},
		{"Jk 9d 9h 7c 7s", "TWO PAIR, NINES AND SEVENS"},
		{"Qs Qd 9h 7c 2s", "PAIR OF QUEENS"},
	}
	var r Ranker
	for _, tt := range tests {
		if got := r.High([HighSize]cards.Card(deck(t, tt.hand))); got.String() != tt.name {
			t.Errorf("%s is %q, want %q", tt.hand, got, tt.name)
		}
	}
	// The joker makes the best flush it can: ace, king and queen high.
	a := r.High([HighSize]cards.Card(deck(t, "Jk Ah 9h 5h Kh")))
	b := r.High([HighSize]cards.Card(deck(t, "Ah Kh Jh 9h 5h")))
	if a <= b {
		t.Error("the joker did not make the best flush")
	}
	// Five aces beat a royal flush.
	if FiveAces <= r.High([HighSize]cards.Card(deck(t, "As Ks Qs Js Ts"))) {
		t.Error("five aces do not beat a royal flush")
	}
	// The table ranks as the hand alone does.
	fast := NewRanker()
	for _, tt := range tests {
		h := [HighSize]cards.Card(deck(t, tt.hand))
		if fast.High(h) != r.High(h) {
			t.Errorf("the table ranks %s differently", tt.hand)
		}
	}
}

func TestLow(t *testing.T) {
	tests := []struct {
		low  string
		name string
		pair bool
	}{
		{"Jk Ad", "PAIR OF ACES", true},
		{"Kd 2h", "KING TWO", false},
		{"2h Kd", "KING TWO", false},
		{"Jk Qd", "ACE QUEEN", false},
		{"7c 7d", "PAIR OF SEVENS", true},
	}
	for _, tt := range tests {
		cs := deck(t, tt.low)
		l := LowOf(cs[0], cs[1])
		if l.String() != tt.name || l.Pair() != tt.pair {
			t.Errorf("%s is %q, pair %v", tt.low, l, l.Pair())
		}
	}
	// Any pair beats any two cards that make nothing.
	if LowOf(deck(t, "2c 2d")[0], deck(t, "2c 2d")[1]) <= LowOf(deck(t, "Ac Kd")[0], deck(t, "Ac Kd")[1]) {
		t.Error("a pair of twos does not beat ace, king")
	}
}

func TestFouls(t *testing.T) {
	var r Ranker
	tests := []struct {
		high, low string
		fouls     bool
	}{
		{"9s 9d 7h 4c 2s", "9h 9c", false}, // the same pair, with more beside it
		{"9s 9d 7h 4c 2s", "Th Tc", true},
		{"8s 7d 6h 4c 2s", "Kh 3c", true},
		{"As 7d 6h 4c 2s", "Kh Qc", false},
		{"As Qd 6h 4c 2s", "Ah Kc", true},
		{"2s 3d 4h 5c 6s", "Ah Ac", false},
	}
	for _, tt := range tests {
		lows := deck(t, tt.low)
		if got := Fouls(r.High([HighSize]cards.Card(deck(t, tt.high))), LowOf(lows[0], lows[1])); got != tt.fouls {
			t.Errorf("%s over %s fouls: %v, want %v", tt.high, tt.low, got, tt.fouls)
		}
	}
}

func TestSettle(t *testing.T) {
	var r Ranker
	hi := func(s string) High { return r.High([HighSize]cards.Card(deck(t, s))) }
	lo := func(s string) Low { cs := deck(t, s); return LowOf(cs[0], cs[1]) }
	tests := []struct {
		high, low, dHigh, dLow string
		outcome                Outcome
		paid, commission       int // for a bet of 1000
	}{
		{"Ks Kd 7h 4c 2s", "Ah Qc", "Qs Qd 7h 4c 2s", "Ac Jd", Won, 1950, 50},
		{"Ks Kd 7h 4c 2s", "Ah Qc", "As Ad 7h 4c 2s", "Ac Jd", Pushed, 1000, 0},
		{"Ks Kd 7h 4c 2s", "Kh Qc", "As Ad 7h 4c 2s", "Ac Jd", Lost, 0, 0},
		// A copy goes to the dealer: a tied low hand loses it.
		{"Ks Kd 7h 4c 2s", "Ah Qc", "Qs Qd 7h 4c 2s", "Ad Qh", Pushed, 1000, 0},
		{"Ks Kd 7h 4c 2s", "Ah Qc", "Kh Kc 7d 4s 2c", "Ad Qh", Lost, 0, 0},
	}
	for _, tt := range tests {
		o, paid, com := Settle(hi(tt.high), lo(tt.low), hi(tt.dHigh), lo(tt.dLow), 1000)
		if o != tt.outcome || paid != tt.paid || com != tt.commission {
			t.Errorf("%s / %s against %s / %s: %v paying %d less %d", tt.high, tt.low, tt.dHigh, tt.dLow, o, paid, com)
		}
	}
	// The commission on the smallest chip is still a whole number of cents.
	if _, paid, com := Settle(hi("Ks Kd 7h 4c 2s"), lo("Ah Qc"), hi("Qs Qd 7h 4c 2s"), lo("Ac Jd"), 100); paid != 195 || com != 5 {
		t.Errorf("a $1 win pays %d less %d", paid, com)
	}
}

// TestEverySetting checks that every hand can be set, and set the house
// way, without fouling.
func TestEverySetting(t *testing.T) {
	m := New(bank.New(), 7, 8)
	for range 20_000 {
		var h [HandSize]cards.Card
		draw(m.rng, &m.next, h[:])
		low := m.Ranker.HouseWay(h)
		if _, ok := m.Ranker.Set(h, low); !ok {
			t.Fatalf("the house way sets %v with %v low, which fouls", h, low)
		}
	}
}

func TestPlay(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 10_000 // $100
	if !m.Place() || m.Bet != 10_000 {
		t.Fatalf("the bet is %d", m.Bet)
	}
	m.next = deck(t, "Ks Kd 7h 4c 2s Ah Qc  Qs Qd 7d 4s 3c Ac Jd")
	if !m.Deal() || !m.Dealt || b.Credits != bank.StartCredits-10_000 {
		t.Fatalf("after the deal: dealt %v, %d credits", m.Dealt, b.Credits)
	}
	if m.Place() || m.Deal() {
		t.Error("bet or dealt with a hand waiting")
	}
	if _, ok := m.Play([LowSize]int{0, 1}); ok {
		t.Error("played a hand set to foul: the kings low")
	}
	if _, ok := m.Play([LowSize]int{3, 3}); ok {
		t.Error("played a hand with one card low twice")
	}
	r, ok := m.Play([LowSize]int{5, 6})
	if !ok || m.Dealt {
		t.Fatal("the hand was not played")
	}
	// Kings over ace, queen beats queens over ace, jack.
	if r.Outcome != Won || !r.HighWon || !r.LowWon || r.Paid != 19_500 || r.Commission != 500 {
		t.Errorf("the hand came out %+v", r)
	}
	if b.Credits != bank.StartCredits-10_000+19_500 {
		t.Errorf("the bank has %d credits", b.Credits)
	}
	if _, ok := m.Play([LowSize]int{5, 6}); ok {
		t.Error("played with no hand")
	}
}

func TestBets(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	if m.Rebet() || m.Deal() {
		t.Error("made a bet or dealt with nothing")
	}
	b.Credits = 10_000_000
	m.Chip = 100_000
	for range 5 {
		m.Place()
	}
	if m.Place() || m.Bet != MaxBet {
		t.Errorf("the bet went to %d", m.Bet)
	}
	m.Remove()
	m.Chip = 500
	m.Remove()
	if m.Bet != MaxBet-100_500 {
		t.Errorf("after taking two chips off the bet is %d", m.Bet)
	}
	m.Clear()
	b.Credits = 99
	m.Chip = 100
	if m.Place() {
		t.Error("bet 100 with 99 in the bank")
	}
	b.Credits = 100
	if !m.Place() || !m.Deal() {
		t.Fatal("could not bet the whole bank")
	}
	m.Play(m.Ranker.HouseWay(m.Hand))
	b.Credits = 100_000
	if !m.Rebet() || m.Bet != 100 {
		t.Errorf("the rebet is %d", m.Bet)
	}
}

func TestSaveAndRestore(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 2500
	m.Place()
	m.next = deck(t, "Jk Kd 9h 7c 5s 3d 2h")
	m.Deal()
	s := m.Save()

	again := New(b, 3, 4)
	again.Restore(s)
	if again.Save() != s || !again.Dealt || again.Hand != m.Hand {
		t.Errorf("restored %+v, want %+v", again.Save(), s)
	}

	bad := s
	bad.Hand[1] = bad.Hand[0]
	again.Restore(bad)
	if again.Dealt {
		t.Error("restored a hand with a card twice")
	}
	bad = s
	bad.Hand[2] = DeckSize
	again.Restore(bad)
	if again.Dealt {
		t.Error("restored a hand with a card not in the deck")
	}
	bad = s
	bad.Chip, bad.Last, bad.Dealt, bad.Bet = 7, MaxBet+100, false, 150
	again.Restore(bad)
	if again.Chip != ChipChoices[0] || again.Last != 0 || again.Bet != 0 {
		t.Errorf("restored a chip of %d, last %d, bet %d", again.Chip, again.Last, again.Bet)
	}
}

// TestFiveAcesCategory checks that five aces count above every category.
func TestFiveAcesCategory(t *testing.T) {
	if FiveAces.Category() <= pokerhand.RoyalFlush {
		t.Error("five aces do not count above a royal flush")
	}
}
