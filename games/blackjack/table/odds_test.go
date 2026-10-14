package table

import (
	"math"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
)

// The odds take a moment to work out, so the tests share them.
var testOdds = sync.OnceValue(NewOdds)

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-12
}

// TestPairOdds checks Perfect Pairs against the counts worked out by hand:
// of the 311 cards left after the first, 5 are the same card again, 6 are
// its other suit of the same colour and 12 are its rank in the other
// colour.
func TestPairOdds(t *testing.T) {
	if PairOf(cards.New(cards.King, cards.Hearts), cards.New(cards.Queen, cards.Hearts)) != NoPair {
		t.Error("a king and a queen count as a pair")
	}
	chance := PairChances()
	want := [...]float64{NoPair: 288.0 / 311, MixedPair: 12.0 / 311, ColouredPair: 6.0 / 311, PerfectPair: 5.0 / 311}
	total := 0.0
	for kind, c := range chance {
		total += c
		if !near(c, want[kind]) {
			t.Errorf("%v comes up %v of the time, want %v", PairKind(kind), c, want[kind])
		}
	}
	if !near(total, 1) {
		t.Errorf("the chances add up to %v", total)
	}

	// 5 at 25, 6 at 12 and 12 at 6, each with the bet back, out of 311.
	ret := PairReturn()
	if !near(ret, 292.0/311) {
		t.Errorf("Perfect Pairs pays back %v, want %v", ret, 292.0/311)
	}
	t.Logf("Perfect Pairs: return to player %.4f%%; a pair once in %.1f hands", ret*100, 1/(1-chance[NoPair]))
}

func TestDealerEnds(t *testing.T) {
	for up := 1; up <= 10; up++ {
		sh := freshShoe()
		sh.take(up)
		before := sh
		e := dealerEnds(up, &sh)
		if sh != before {
			t.Errorf("working out the dealer's hand changed the shoe")
		}
		total := 0.0
		for _, p := range e {
			total += p
		}
		if !near(total, 1) {
			t.Errorf("showing %d, the ways the dealer ends add up to %v", up, total)
		}
		// A blackjack is the one other card it takes, out of the 311 left.
		want := 0.0
		switch up {
		case 1:
			want = 96.0 / 311
		case 10:
			want = 24.0 / 311
		}
		if !near(e[endBlackjack], want) || !near(blackjackChance(up, &sh), want) {
			t.Errorf("showing %d, a blackjack %v of the time, want %v", up, e[endBlackjack], want)
		}
	}

	// A dealer showing a six goes bust more often than one showing a ten.
	six, ten := freshShoe(), freshShoe()
	six.take(6)
	ten.take(10)
	if b6, b10 := dealerEnds(6, &six)[endBust], dealerEnds(10, &ten)[endBust]; b6 <= b10 {
		t.Errorf("the dealer busts %v of the time showing a six and %v showing a ten", b6, b10)
	}
}

func TestStandValue(t *testing.T) {
	// Half the time the dealer makes 20 and the rest goes bust.
	e := ends{3: 0.5, endBust: 0.5}
	tests := []struct {
		total int
		units float64
		want  float64
	}{
		{21, 1, 1},
		{20, 1, 0.5},
		{19, 1, 0},
		{12, 1, 0},
		{21, 2, 2},
		{19, 2, 0},
	}
	for _, tt := range tests {
		if got := e.stand(tt.total, tt.units, 1); !near(got, tt.want) {
			t.Errorf("standing on %d for %v bets wins %v, want %v", tt.total, tt.units, got, tt.want)
		}
	}
	// A dealer's blackjack costs one bet, however many were staked.
	e = ends{endBlackjack: 1}
	if got := e.stand(20, 2, 1); !near(got, -1) {
		t.Errorf("a doubled hand loses %v bets to a blackjack, want 1", -got)
	}
}

func TestReturn(t *testing.T) {
	o := testOdds()
	ret := o.Return()
	t.Logf("return to player without splitting %.4f%%", ret*100)
	// The game should cost a careful player a little, and no more.
	if ret < 0.98 || ret >= 1 {
		t.Errorf("the game pays back %v", ret)
	}
}

func TestAdvise(t *testing.T) {
	o := testOdds()
	tests := []struct {
		hand string
		up   string
		best Action
	}{
		{"Ts Td", "6h", ActStand},
		{"Ts 6d", "6h", ActStand}, // the dealer is likely to bust
		{"6s 5d", "6h", ActDouble},
		{"2s 3d", "Th", ActHit},
		{"As 7d", "Th", ActHit}, // a soft 18 is behind a ten
		{"As 2d", "9h", ActHit},
		{"Ts 9d", "Ah", ActStand},
		{"8s 3d 2c", "6h", ActStand}, // three cards cannot be doubled
		{"As 7d", "6h", ActStand},    // a soft 18 cannot be doubled, and is good enough
		{"5s 4d", "6h", ActDouble},
	}
	for _, tt := range tests {
		hand := deck(tt.hand)
		a := o.Advise(deck(tt.up)[0], hand)
		if a.Best != tt.best {
			t.Errorf("%s against %s: best to %v (stand %.4f, hit %.4f, double %.4f), want %v",
				tt.hand, tt.up, a.Best, a.Stand, a.Hit, a.Double, tt.best)
		}
		if a.CanDouble != CanDoubleOn(hand) {
			t.Errorf("%s: can double %v", tt.hand, a.CanDouble)
		}
		// No play wins more than two bets or loses more than two.
		for _, v := range []float64{a.Stand, a.Hit, a.Double} {
			if v < -2 || v > 2 {
				t.Errorf("%s against %s: a play worth %v bets", tt.hand, tt.up, v)
			}
		}
	}

	// Standing on 20 against a six is a winning play.
	if a := o.Advise(deck("6h")[0], deck("Ts Td")); a.Stand <= 0.5 {
		t.Errorf("standing on 20 against a six is worth %v", a.Stand)
	}
}

// TestAdviseAgainstDraws checks what a hit is worth the long way, for a
// hand one card from the end: by taking each card in turn.
func TestAdviseAgainstDraws(t *testing.T) {
	o := testOdds()
	up, hand := deck("7h")[0], deck("Ts 9d") // any card but a two busts
	a := o.Advise(up, hand)

	sh := freshShoe()
	for _, c := range append(deck("7h"), hand...) {
		sh.take(Value(c))
	}
	want := 0.0
	for v := 1; v <= 10; v++ {
		p := float64(sh.left[v]) / float64(sh.n)
		switch v {
		case 1: // 20, which can be hit again but is best left
			sh.take(v)
			e := dealerEnds(7, &sh)
			sh.put(v)
			want += p * e.stand(20, 1, 1)
		case 2: // 21, which stands
			sh.take(v)
			e := dealerEnds(7, &sh)
			sh.put(v)
			want += p * e.stand(21, 1, 1)
		default:
			want -= p
		}
	}
	if math.Abs(a.Hit-want) > 1e-9 {
		t.Errorf("hitting 19 against a seven is worth %v, want %v", a.Hit, want)
	}
}

// TestPlayReturn plays a great many hands, each from a fresh shoe, with the
// best play and never splitting, and checks that what comes back is close to the return
// worked out.
func TestPlayReturn(t *testing.T) {
	hands := 3_000_000
	if testing.Short() {
		hands = 300_000
	}
	o := testOdds()
	m := New(bank.New(), 11, 12)
	bet, won := 0, 0
	for range hands {
		m.Bank.Credits = bank.StartCredits
		m.shuffle() // the odds worked out are those of a fresh shoe
		if !m.Deal() {
			t.Fatal("could not deal")
		}
		for m.Dealt {
			switch o.Advise(m.Dealer[0], m.Hands[0].Cards).Best {
			case ActStand:
				m.Stand()
			case ActHit:
				m.Hit()
			case ActDouble:
				m.Double()
			}
		}
		bet += m.Stake
		won += m.Last.Paid - m.Last.Staked
	}
	played := 1 + float64(won)/float64(bet)
	want := o.Return()
	t.Logf("%d hands paid back %.2f%%; worked out as %.2f%%", hands, played*100, want*100)

	// A hand wins or loses a little over one bet on average, so the return
	// over this many hands wanders by well under a quarter of a percent.
	margin := 0.004
	if testing.Short() {
		margin = 0.012
	}
	if math.Abs(played-want) > margin {
		t.Errorf("paid back %.4f over %d hands, want about %.4f", played, hands, want)
	}
}

// TestAdviseSplit checks the advice for hands that can be split, and for
// the hands a split makes.
func TestAdviseSplit(t *testing.T) {
	tests := []struct {
		cards string // the player's two cards with the dealer's between them
		best  Action
	}{
		{"8s 6h 8d", ActSplit},
		{"As 6h Ad", ActSplit},
		{"Ts 6h Kd", ActStand},  // twenty is too good to break up
		{"5s 6h 5d", ActDouble}, // ten is better doubled than split
		{"9s 7h 9d", ActStand},  // eighteen beats a likely seventeen
	}
	for _, tt := range tests {
		m := stacked(20, 0, tt.cards)
		m.Deal()
		a, ok := BestPlay(m)
		if !ok || !a.CanSplit || a.CanDouble != CanDoubleOn(m.Hands[0].Cards) {
			t.Fatalf("%s: no advice on splitting: %+v", tt.cards, a)
		}
		if a.Best != tt.best {
			t.Errorf("%s: best to %v (stand %.4f, hit %.4f, double %.4f, split %.4f), want %v",
				tt.cards, a.Best, a.Stand, a.Hit, a.Double, a.Split, tt.best)
		}
	}

	// A split the bank cannot cover is not advised.
	m := stacked(20, 0, "8s 6h 8d")
	m.Bank.Credits = 20
	m.Deal()
	if a, _ := BestPlay(m); a.CanSplit || a.CanDouble || a.Best == ActSplit || a.Best == ActDouble {
		t.Errorf("advised %v with nothing left in the bank: %+v", a.Best, a)
	}

	// Each hand a split makes is weighed up in its turn, and can be doubled.
	m = stacked(20, 0, "8s 6h 8d 3c")
	m.Deal()
	m.Split()
	a, ok := BestPlay(m)
	if !ok || a.CanSplit || !a.CanDouble || a.Best != ActDouble {
		t.Errorf("eleven after a split: %+v", a)
	}

	// With no hand being played there is no advice.
	m = stacked(20, 0, "")
	if _, ok := BestPlay(m); ok {
		t.Error("advice given with no hand dealt")
	}
}

// TestBestPlayFreshShoe checks that the advice worked out from the shoe as
// it stands agrees with the odds of a fresh shoe when the shoe is fresh.
func TestBestPlayFreshShoe(t *testing.T) {
	o := testOdds()
	for _, cards := range []string{"Ts 6h 6d", "5s 6h 4d", "As 9h 6d", "2s Th 3d", "9s 7h 8d"} {
		m := stacked(20, 0, cards)
		m.Deal()
		got, ok := BestPlay(m)
		want := o.Advise(m.Dealer[0], m.Hands[0].Cards)
		if !ok || !near(got.Stand, want.Stand) || !near(got.Hit, want.Hit) || !near(got.Double, want.Double) || got.Best != want.Best {
			t.Errorf("%s: from the shoe %+v, from a fresh shoe %+v", cards, got, want)
		}
	}
}

// TestBestPlayFollowsShoe checks that the advice moves with what is left
// in the shoe: 16 against a ten is hit from a full shoe, but stood on once
// the small cards that would help it have gone.
func TestBestPlayFollowsShoe(t *testing.T) {
	m := stacked(20, 0, "Ts Th 6d")
	m.Deal()
	if a, _ := BestPlay(m); a.Best != ActHit {
		t.Errorf("16 against a ten from a full shoe: best to %v (stand %.4f, hit %.4f)", a.Best, a.Stand, a.Hit)
	}

	// Take every ace, two, three, four and five out of the shoe.
	for c := range m.Left {
		if v := Value(cards.Card(c)); v <= 5 {
			m.ToCut -= m.Left[c]
			m.Left[c] = 0
		}
	}
	a, _ := BestPlay(m)
	if a.Best != ActStand || !near(a.Hit, -1) {
		t.Errorf("16 against a ten with no small cards left: best to %v (stand %.4f, hit %.4f)", a.Best, a.Stand, a.Hit)
	}
}
