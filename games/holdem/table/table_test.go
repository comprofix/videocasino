package table

import (
	"math"
	"strings"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/pokerhand"
)

// deck makes cards from names such as "As Kd 4c".
func deck(t *testing.T, names string) []cards.Card {
	t.Helper()
	var cs []cards.Card
	for _, f := range strings.Fields(names) {
		r := strings.IndexByte("23456789TJQKA", f[0])
		s := strings.IndexByte("cdhs", f[1])
		if len(f) != 2 || r < 0 || s < 0 {
			t.Fatalf("%q is not a card", f)
		}
		cs = append(cs, cards.New(cards.Two+cards.Rank(r), cards.Suit(s)))
	}
	return cs
}

// best scores the best five of the cards named.
func best(t *testing.T, names string) pokerhand.Score {
	t.Helper()
	_, s := pokerhand.Best(deck(t, names))
	return s
}

// The odds share one score table, which takes a moment to make.
var testOdds = sync.OnceValue(NewOdds)

func TestCounts(t *testing.T) {
	if flops != 19_600 || turnRivers != 1_081 || dealerHoles != 990 {
		t.Errorf("%d flops, %d turns and rivers, %d dealer's hands", flops, turnRivers, dealerHoles)
	}
}

func TestSettle(t *testing.T) {
	tests := []struct {
		player, dealer, board string
		play                  int
		outcome               Outcome
		paid                  int // for an ante and a blind of 100
	}{
		// A win: the play bet and the ante pay 1 to 1, the blind by the
		// hand, and below a straight the blind is handed back.
		{"Ac Ad", "9c 9d", "Kh Qs 2c 7d 8h", 400, Won, 800 + 200 + 100},
		{"2c 3d", "Ac Ad", "4h 5s 6c Kd Kh", 100, Won, 200 + 200 + 200},
		{"2h 9h", "Ac Ad", "Jh 4h Kh 3c 3d", 200, Won, 400 + 200 + 250},
		{"As Ks", "Ac Ad", "Qs Js Ts 2c 3d", 100, Won, 200 + 200 + 50_100},
		// The dealer does not qualify: the ante is handed back on a win.
		{"Ac Kd", "Qc Jd", "9h 7s 2c 3d 5h", 100, Won, 200 + 100 + 100},
		// A loss: the play bet and the blind are lost, the ante only if the
		// dealer qualifies.
		{"Kc Kd", "Ac Ad", "5h Qs 2c 7d 8h", 400, Lost, 0},
		{"Qc Jd", "Ac Kd", "9h 7s 2c 3d 5h", 100, Lost, 100},
		// A tie hands everything back.
		{"Ac Kd", "As Kc", "Qh Js Tc 2d 3h", 300, Pushed, 500},
	}
	for _, tt := range tests {
		player := best(t, tt.player+" "+tt.board)
		dealer := best(t, tt.dealer+" "+tt.board)
		o, paid := Settle(player, dealer, 100, 100, tt.play)
		if o != tt.outcome || paid != tt.paid {
			t.Errorf("%s against %s on %s, play %d: %v paying %d, want %v paying %d", tt.player, tt.dealer, tt.board, tt.play, o, paid, tt.outcome, tt.paid)
		}
	}
}

func TestTripsPaid(t *testing.T) {
	tests := []struct {
		cards string
		paid  int // for a bet of 100
	}{
		{"Ac Ad Kh Ks 2c 7d 8h", 0},
		{"Ac Ad Ah Ks 2c 7d 8h", 400},
		{"2c 3d 4h 5s 6c Kd Kh", 600},
		{"2h 9h Jh 4h Kh 3c 3d", 700},
		{"3c 3d 3h Ks Kc 2d 7h", 900},
		{"7c 7d 7h 7s 2c 3d 4h", 3100},
		{"9h Kh Qh Jh Th 2c 3d", 4100},
		{"As Ks Qs Js Ts 2c 3d", 5100},
	}
	for _, tt := range tests {
		if got := TripsPaid(best(t, tt.cards), 100); got != tt.paid {
			t.Errorf("Trips of 100 on %s pays %d, want %d", tt.cards, got, tt.paid)
		}
	}
}

func TestClasses(t *testing.T) {
	seen := make(map[int]int)
	for a := range cards.Card(cards.DeckSize) {
		for b := range a {
			seen[Class([HoleSize]cards.Card{a, b})]++
		}
	}
	if len(seen) != Classes {
		t.Fatalf("%d kinds of starting hand, want %d", len(seen), Classes)
	}
	for k := range Classes {
		hole, like := ClassHand(k)
		if Class(hole) != k || seen[k] != like {
			t.Errorf("kind %d: %v is of kind %d, and there are %d like it, want %d", k, hole, Class(hole), seen[k], like)
		}
	}
	// A pair, a suited hand and one not suited.
	for _, names := range []string{"Ac Ad", "Ah Kh", "Ah Kc"} {
		hole := [HoleSize]cards.Card(deck(t, names))
		swapped := [HoleSize]cards.Card{hole[1], hole[0]}
		if Class(hole) != Class(swapped) {
			t.Errorf("%s is of a different kind the other way round", names)
		}
	}
}

func TestRounds(t *testing.T) {
	if b := Preflop.Bets(); len(b) != 2 || b[0] != 4 || b[1] != 3 {
		t.Errorf("before the flop the bets are %v", b)
	}
	if Flop.Bets()[0] != 2 || River.Bets()[0] != 1 || NoHand.Bets() != nil {
		t.Error("the bets of the later rounds are wrong")
	}
	if Preflop.Shown() != 0 || Flop.Shown() != 3 || River.Shown() != 5 {
		t.Error("the board is not turned as the rounds go")
	}
}

// TestPlayHand checks a hand checked to the river and bet there.
func TestPlayHand(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 500
	m.Place(Ante)
	m.Chip = 100
	m.Place(Trips)
	if m.Total() != 1100 {
		t.Fatalf("%d on the table, want 1100 with the blind", m.Total())
	}
	m.next = deck(t, "Ah Qh  As Kd 7c  2h Qc  9s 9d")
	if !m.Deal() || m.Round != Preflop || b.Credits != bank.StartCredits-1100 {
		t.Fatalf("after the deal: round %v, %d credits", m.Round, b.Credits)
	}
	if m.Place(Ante) || m.Deal() {
		t.Error("bet or dealt with a hand waiting")
	}
	if !m.Check() || m.Round != Flop || m.Board[2] != deck(t, "7c")[0] {
		t.Fatalf("after checking: round %v, board %v", m.Round, m.Board)
	}
	if m.CanBet(4) || !m.CanBet(2) {
		t.Error("on the flop only 2 antes can be bet")
	}
	if !m.Check() || m.Round != River {
		t.Fatal("did not check to the river")
	}
	if m.Check() {
		t.Error("checked on the river")
	}

	r, ok := m.Bet(1)
	if !ok || m.Round != NoHand {
		t.Fatal("the hand was not bet")
	}
	// Two pair beats the dealer's nines: the play bet and the ante pay
	// 1 to 1 and the blind is handed back; Trips loses.
	if r.Outcome != Won || !r.Qualified || r.Play != 500 || r.Paid != 1000+1000+500 || r.TripsPaid != 0 {
		t.Errorf("the hand came out %+v", r)
	}
	if b.Credits != bank.StartCredits-1100-500+2500 {
		t.Errorf("the bank has %d credits", b.Credits)
	}
}

// TestBetEarly checks a bet before the flop, which turns the whole board.
func TestBetEarly(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 100
	m.Place(Ante)
	m.next = deck(t, "Ah Ad  Ac Kd 7c 2h 3c  9s 9d")
	m.Deal()
	if m.CanBet(2) || m.CanBet(1) || !m.CanBet(3) {
		t.Error("before the flop only 4 or 3 antes can be bet")
	}
	r, ok := m.Bet(4)
	if !ok || r.Board != [BoardSize]cards.Card(deck(t, "Ac Kd 7c 2h 3c")) {
		t.Fatalf("bet 4: %+v", r)
	}
	// Three aces against a pair of nines.
	if r.Outcome != Won || r.Play != 400 || r.Paid != 800+200+100 {
		t.Errorf("the hand came out %+v", r)
	}
}

func TestFold(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 100
	m.Place(Ante)
	m.Place(Trips)
	m.next = deck(t, "7h 7d  7c Kd 2c 2h 3c")
	m.Deal()
	if _, ok := m.Fold(); ok {
		t.Error("folded before the river")
	}
	m.Check()
	m.Check()
	r, ok := m.Fold()
	// The ante and the blind are lost, but Trips pays on the full house.
	if !ok || m.Round != NoHand || r.Outcome != Folded || r.Paid != 0 || r.TripsPaid != 900 {
		t.Errorf("folded: %+v", r)
	}
	if b.Credits != bank.StartCredits-300+900 {
		t.Errorf("the bank has %d credits", b.Credits)
	}
}

func TestCover(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 100
	b.Credits = 599
	if m.Place(Ante) {
		t.Error("an ante of 100 was put down with 599 in the bank")
	}
	b.Credits = 600
	if !m.Place(Ante) || !m.Deal() {
		t.Fatal("an ante of 100 could not be dealt with 600 in the bank")
	}
	b.Credits = 300
	if m.CanBet(4) || !m.CanBet(3) {
		t.Error("the bets the bank can cover are wrong")
	}
	b.Credits = 10_000_000
	m.Bet(3)

	m.Chip = 100_000
	for range 5 {
		m.Place(Trips)
	}
	if m.Place(Trips) || m.Bets[Trips] != MaxBet {
		t.Errorf("Trips went to %d", m.Bets[Trips])
	}
	m.Remove(Trips)
	m.Clear()
	if m.Total() != 0 || !m.Rebet() || m.Bets[Ante] != 100 {
		t.Errorf("after clearing and making the last bets again: %v", m.Bets)
	}
}

func TestSaveAndRestore(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 2500
	m.Place(Ante)
	m.Deal()
	m.Check()
	s := m.Save()
	if len(s.Board) != FlopSize {
		t.Fatalf("the save holds %d of the board", len(s.Board))
	}

	again := New(b, 3, 4)
	again.Restore(s)
	if again.Round != Flop || again.Hole != m.Hole || again.Board != m.Board || again.Stake != m.Stake {
		t.Errorf("restored %+v, want %+v", again.Save(), s)
	}

	bad := s
	bad.Board = append([]cards.Card{}, s.Board[0], s.Board[1], s.Hole[0])
	again.Restore(bad)
	if again.Round != NoHand {
		t.Error("restored a board holding one of the player's cards")
	}
	bad = s
	bad.Round = River
	again.Restore(bad)
	if again.Round != NoHand {
		t.Error("restored a river with only the flop turned")
	}
	bad = s
	bad.Chip, bad.Round, bad.Bets = 7, NoHand, [Bets]int{150, 0}
	again.Restore(bad)
	if again.Chip != ChipChoices[0] || again.Bets != ([Bets]int{}) {
		t.Errorf("restored a chip of %d and bets %v", again.Chip, again.Bets)
	}
}

// TestAdviseRiver checks the advice on the river against hands whose worth
// can be seen.
func TestAdviseRiver(t *testing.T) {
	o := testOdds()
	advise := func(hole, board string) Advice {
		return o.Advise(River, [HoleSize]cards.Card(deck(t, hole)), [BoardSize]cards.Card(deck(t, board)))
	}
	// The nuts: a royal flush cannot lose. Betting 1 ante wins the play bet
	// and the blind's 500 to 1 every time, and the ante whenever the dealer
	// qualifies, which with a board paired is always.
	a := advise("As Ks", "Qs Js Ts 2c 2d")
	if want := 1 + 1 + 500.0; a.Best != 1 || math.Abs(a.Choices[1].Worth-want) > 1e-9 {
		t.Errorf("a royal flush is worth %+v, want %v", a, want)
	}
	if a.Choices[0].Antes != 0 || a.Choices[0].Worth != -2 {
		t.Errorf("folding is worth %+v", a.Choices[0])
	}
	// Seven high on a board of nothing loses to almost everything.
	if a := advise("7c 2d", "Ah Kh 9s 5c 3d"); a.Best != 0 {
		t.Errorf("seven high is advised %+v", a)
	}
}

// TestAdviseFlop checks the flop's advice against counting the river's.
func TestAdviseFlop(t *testing.T) {
	o := testOdds()
	hole := [HoleSize]cards.Card(deck(t, "Ah Ad"))
	flop := [FlopSize]cards.Card(deck(t, "As Kd 7c"))
	a := o.Advise(Flop, hole, [BoardSize]cards.Card{flop[0], flop[1], flop[2]})
	// Three aces: betting is better than checking, and is worth more than
	// the ante and the blind together.
	if a.Best != 1 || a.Choices[1].Antes != 2 || a.Choices[1].Worth < 2 || a.Choices[0].Worth > a.Choices[1].Worth {
		t.Errorf("three aces on the flop: %+v", a)
	}
}

// TestKept checks the figures TestPreflop counted against the published
// ones for the game. They are close enough to show that nothing has gone
// astray in the copying.
func TestKept(t *testing.T) {
	if returnStaked == 0 {
		t.Fatal("no figures are kept yet: run TestPreflop with -long -update -timeout 0")
	}
	if e := Edge(); math.Abs(e-0.02185) > 0.0002 {
		t.Errorf("the edge is %.5f of the ante, want about 0.02185", e)
	}
	if r := TripsReturn(); math.Abs(1-r-0.0190) > 0.0002 {
		t.Errorf("Trips pays back %.4f, want about %.4f", r, 1-0.0190)
	}
	// Aces are always bet 4 antes before the flop, and seven, two not
	// suited never is.
	o := testOdds()
	if a := o.Advise(Preflop, [HoleSize]cards.Card(deck(t, "Ac Ad")), [BoardSize]cards.Card{}); a.Choices[a.Best].Antes != 4 {
		t.Errorf("aces are advised %+v", a)
	}
	if a := o.Advise(Preflop, [HoleSize]cards.Card(deck(t, "7c 2d")), [BoardSize]cards.Card{}); a.Choices[a.Best].Antes != 0 {
		t.Errorf("seven, two is advised %+v", a)
	}
}
