package table

import (
	"math"
	"strings"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/pokerhand"
)

// card makes a card from a name such as "As".
func card(t *testing.T, name string) cards.Card {
	t.Helper()
	r := strings.IndexByte("23456789TJQKA", name[0])
	s := strings.IndexByte("cdhs", name[1])
	if len(name) != 2 || r < 0 || s < 0 {
		t.Fatalf("%q is not a card", name)
	}
	return cards.New(cards.Two+cards.Rank(r), cards.Suit(s))
}

// hand makes a hand from names such as "As Kd 4c 3h 2s".
func hand(t *testing.T, names string) Hand {
	t.Helper()
	var h Hand
	fields := strings.Fields(names)
	if len(fields) != HandSize {
		t.Fatalf("%q is not five cards", names)
	}
	for i, f := range fields {
		h[i] = card(t, f)
	}
	return h
}

// deck lays out cards for a machine to deal next, from names.
func deck(t *testing.T, names string) []cards.Card {
	t.Helper()
	var cs []cards.Card
	for _, f := range strings.Fields(names) {
		cs = append(cs, card(t, f))
	}
	return cs
}

func TestDealerHands(t *testing.T) {
	if dealerHands != 163_185 {
		t.Errorf("the dealer can hold %d hands, want 163,185", dealerHands)
	}
}

func TestQualifies(t *testing.T) {
	tests := []struct {
		hand string
		want bool
	}{
		{"Ac Kd 4h 3s 2c", true},
		{"Ac Kd Qh Js 9c", true},
		{"Ac Qd Jh 9s 8c", false},
		{"Kc Qd Jh 9s 8c", false},
		{"2c 2d 5h 4s 3c", true},
		{"Ac 2d 3h 4s 5c", true},
	}
	for _, tt := range tests {
		if got := Qualifies(hand(t, tt.hand).Score()); got != tt.want {
			t.Errorf("%s qualifies: %v, want %v", tt.hand, got, tt.want)
		}
	}
}

func TestSettle(t *testing.T) {
	tests := []struct {
		player, dealer string
		outcome        Outcome
		paid           int // for an ante of 1, with the raise of 2
	}{
		// The dealer does not qualify: the ante pays 1 to 1 and the raise
		// comes back, whatever the player holds.
		{"7c 5d 4h 3s 2c", "Ac Qd Jh 9s 8c", NotQualified, 4},
		{"As Ks Qs Js Ts", "Ac Qd Jh 9s 8h", NotQualified, 4},
		// It does: the better hand wins, and the raise pays by the hand.
		{"Ac Kd 5h 3s 2c", "Ad Kh 4s 3c 2d", Won, 6},
		{"7c 7d 4h 3s 2c", "Ac Kd Jh 9s 8c", Won, 6},
		{"7c 7d 4h 4s 2c", "Ac Kd Jh 9s 8c", Won, 8},
		{"7c 7d 7h 4s 2c", "Ac Kd Jh 9s 8c", Won, 10},
		{"Ac 2d 3h 4s 5c", "Ac Kd Jh 9s 8c", Won, 12},
		{"2h 9h Jh 4h Kh", "Ac Kd Jh 9s 8c", Won, 14},
		{"3c 3d 3h Ks Kc", "Ac Kd Jh 9s 8c", Won, 18},
		{"7c 7d 7h 7s 2c", "Ac Kd Jh 9s 8c", Won, 44},
		{"9h Kh Qh Jh Th", "Ac Kd Jh 9s 8c", Won, 104},
		{"As Ks Qs Js Ts", "Ac Kd Jh 9s 8c", Won, 204},
		{"Ac Kd 4h 3s 2c", "Ad Kh 4s 3c 2d", Pushed, 3},
		{"Ac Kd 4h 3s 2c", "Ad Kh 5s 3c 2d", Lost, 0},
		{"Kc Kd 4h 3s 2c", "Ac Ad 5s 3c 2d", Lost, 0},
	}
	for _, tt := range tests {
		o, paid := Settle(hand(t, tt.player).Score(), hand(t, tt.dealer).Score())
		if o != tt.outcome || paid != tt.paid {
			t.Errorf("%s against %s: %v paying %d, want %v paying %d", tt.player, tt.dealer, o, paid, tt.outcome, tt.paid)
		}
	}
}

func TestAdvise(t *testing.T) {
	// A pair of aces is worth raising whatever the dealer shows, and a hand
	// of nothing below a king is not.
	for _, up := range deck(t, "2c 9d Kh As") {
		if a := Advise(hand(t, "Ac Ad 9h 6s 2c"), up); !a.Best || a.Raise <= -1 {
			t.Errorf("a pair of aces against %v: %+v", up, a)
		}
		if a := Advise(hand(t, "Qc Jd 8h 6s 2c"), up); a.Best || a.Raise > -1 {
			t.Errorf("queen high against %v: %+v", up, a)
		}
	}
	// A royal flush never loses: it wins 1 when the dealer does not
	// qualify and 201 when the dealer does, so it is worth between them.
	a := Advise(hand(t, "As Ks Qs Js Ts"), card(t, "2d"))
	if !a.Best || a.Raise <= 1 || a.Raise >= 201 {
		t.Errorf("a royal flush: %+v", a)
	}
}

// TestKept checks the figures kept in odds.go, which TestReturn counted,
// against the published ones for the game. They are close enough to show
// that nothing has gone astray in the copying.
func TestKept(t *testing.T) {
	if returnStaked == 0 {
		t.Fatal("no figures are kept yet: run TestReturn with -long -timeout 0")
	}
	if e := Edge(); math.Abs(e-0.05224) > 0.0001 {
		t.Errorf("the edge is %.5f of the ante, want about 0.05224", e)
	}
	if r := Return(); math.Abs(1-r-0.02556) > 0.0001 {
		t.Errorf("the return is %.5f, want about %.5f", r, 1-0.02556)
	}
	if r := Raised(); r < 0.4 || r > 0.6 {
		t.Errorf("%.3f of hands are raised", r)
	}
}

func TestDealAndRaise(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 500
	if !m.Place() || !m.Place() || m.Ante != 1000 {
		t.Fatalf("the ante is %d after two chips of 500", m.Ante)
	}
	m.next = deck(t, "7c 7d 4h 4s 2c  Ac  Kd Jh 9s 8c")
	if !m.Deal() {
		t.Fatal("no hand was dealt")
	}
	if !m.Dealt || m.Stake != 1000 || m.Ante != 0 || m.Last != 1000 || b.Credits != bank.StartCredits-1000 {
		t.Errorf("after the deal: dealt %v, stake %d, ante %d, last %d, credits %d", m.Dealt, m.Stake, m.Ante, m.Last, b.Credits)
	}
	if m.Hand != hand(t, "7c 7d 4h 4s 2c") || m.Up != card(t, "Ac") {
		t.Errorf("dealt %v with %v up", m.Hand, m.Up)
	}
	// Nothing can be bet with a hand waiting.
	if m.Place() || m.Deal() {
		t.Error("bet or dealt with a hand waiting")
	}

	r, ok := m.Raise()
	if !ok || m.Dealt {
		t.Fatal("the hand was not raised")
	}
	// Two pair beats ace, king: the ante pays 1 to 1 and the raise 2 to 1.
	if r.Dealer != hand(t, "Kd Jh 9s 8c Ac") || r.Outcome != Won || r.Raise != 2000 || r.Paid != 8000 {
		t.Errorf("raised: %+v", r)
	}
	if b.Credits != bank.StartCredits-3000+8000 {
		t.Errorf("the bank has %d credits", b.Credits)
	}
	if _, ok := m.Raise(); ok {
		t.Error("raised with no hand")
	}
}

func TestFold(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Place()
	m.Deal()
	r, ok := m.Fold()
	if !ok || m.Dealt || r.Outcome != Folded || r.Paid != 0 || r.Raise != 0 {
		t.Errorf("folded: %+v", r)
	}
	if r.Dealer[HandSize-1] != m.Up || b.Credits != bank.StartCredits-m.Stake {
		t.Errorf("after folding: dealer %v, credits %d", r.Dealer, b.Credits)
	}
	if _, ok := m.Fold(); ok {
		t.Error("folded with no hand")
	}
}

// TestCover checks that the bank must cover the raise before a hand is
// dealt, and that the hand can be folded if it no longer can.
func TestCover(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 100
	b.Credits = 299
	if m.Place() {
		t.Error("an ante of 100 was put down with 299 in the bank")
	}
	b.Credits = 300
	if !m.Place() || !m.CanDeal() || !m.Deal() {
		t.Fatal("an ante of 100 could not be dealt with 300 in the bank")
	}
	b.Credits = 100
	if _, ok := m.Raise(); ok || !m.Dealt {
		t.Error("raised 200 with 100 in the bank")
	}
	if _, ok := m.Fold(); !ok {
		t.Error("could not fold")
	}

	// No more than the most an ante may be.
	b.Credits = 10_000_000
	m.Chip = 100_000
	for range 5 {
		m.Place()
	}
	if m.Place() || m.Ante != MaxBet {
		t.Errorf("the ante went to %d", m.Ante)
	}
	m.Remove()
	m.Chip = 500
	m.Remove()
	if m.Ante != MaxBet-100_500 {
		t.Errorf("after taking two chips off the ante is %d", m.Ante)
	}
	m.Clear()
	if m.Ante != 0 {
		t.Errorf("after clearing the ante is %d", m.Ante)
	}
}

func TestRebet(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	if m.Rebet() {
		t.Error("made the last bet again with no last hand")
	}
	m.Chip = 2500
	m.Place()
	m.Deal()
	m.Fold()
	if !m.Rebet() || m.Ante != 2500 {
		t.Errorf("the rebet ante is %d", m.Ante)
	}
	b.Credits = 7499
	m.Clear()
	if m.Rebet() {
		t.Error("made a bet the bank cannot cover the raise for")
	}
}

func TestSaveAndRestore(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 2500
	m.Place()
	m.Deal()
	s := m.Save()

	again := New(b, 3, 4)
	again.Restore(s)
	if again.Save() != s || !again.Dealt || again.Hand != m.Hand || again.Up != m.Up {
		t.Errorf("restored %+v, want %+v", again.Save(), s)
	}

	// A hand that could not have been dealt is thrown away.
	bad := s
	bad.Up = bad.Hand[2]
	again.Restore(bad)
	if again.Dealt {
		t.Error("restored a hand with the dealer's card among the player's")
	}
	bad = s
	bad.Stake = 0
	again.Restore(bad)
	if again.Dealt {
		t.Error("restored a hand dealt for nothing")
	}
	bad = s
	bad.Chip, bad.Last, bad.Dealt, bad.Ante = 7, MaxBet+100, false, 150
	again.Restore(bad)
	if again.Chip != ChipChoices[0] || again.Last != 0 || again.Ante != 0 {
		t.Errorf("restored a chip of %d, last %d, ante %d", again.Chip, again.Last, again.Ante)
	}
}

// TestRaisePays checks the table against the standard one.
func TestRaisePays(t *testing.T) {
	want := []int{1, 1, 2, 3, 4, 5, 7, 20, 50, 100}
	for c := range pokerhand.Categories {
		if RaisePays[c] != want[c] {
			t.Errorf("%v pays %d, want %d", c, RaisePays[c], want[c])
		}
	}
}
