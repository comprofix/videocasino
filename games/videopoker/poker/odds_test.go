package poker

import (
	"math"
	"testing"
	"videocasino/bank"
	"videocasino/cards"
)

// maxOdds is the odds at the most coins, where the game is at its best.
// They are worked out once for all the tests.
var maxOdds = NewOdds(&NineSix, MaxCoins)

// drawValue works out what a hold is worth the long way: by making every
// draw that can follow it and averaging what they pay, in coins.
func drawValue(p *Paytable, coins int, h [HandSize]cards.Card, held [HandSize]bool) float64 {
	var dealt [cards.DeckSize]bool
	for _, c := range h {
		dealt[c] = true
	}
	var deck []cards.Card
	for c := range cards.Card(cards.DeckSize) {
		if !dealt[c] {
			deck = append(deck, c)
		}
	}
	var open []int // the places to be filled
	for i, keep := range held {
		if !keep {
			open = append(open, i)
		}
	}

	total, n := 0, 0
	var fill func(place, from int)
	fill = func(place, from int) {
		if place == len(open) {
			total += p.Pay(Evaluate(h), coins)
			n++
			return
		}
		for i := from; i < len(deck); i++ {
			h[open[place]] = deck[i]
			fill(place+1, i+1)
		}
	}
	fill(0, 0)
	return float64(total) / float64(n)
}

// TestReturn checks the return to player against the published figure for
// 9/6 Jacks or Better, and reports what it comes to with fewer coins bet.
func TestReturn(t *testing.T) {
	rtp := maxOdds.Return()
	t.Logf("%d coins: return to player %.4f%%", MaxCoins, rtp*100)
	if want := 0.995439; math.Abs(rtp-want) > 1e-6 {
		t.Errorf("return is %.6f, want %.6f", rtp, want)
	}

	// With fewer coins the royal flush pays less for each of them, so the
	// return is lower; it is the same for any number short of the most.
	one := NewOdds(&NineSix, 1).Return()
	t.Logf("1 to %d coins: return to player %.4f%%", MaxCoins-1, one*100)
	if one >= rtp || one < 0.98 {
		t.Errorf("return at one coin is %.6f, against %.6f at %d", one, rtp, MaxCoins)
	}
	if four := NewOdds(&NineSix, MaxCoins-1).Return(); math.Abs(four-one) > 1e-12 {
		t.Errorf("return at %d coins is %.6f, but %.6f at one", MaxCoins-1, four, one)
	}
}

// TestReturnOtherTables checks the working against the published returns
// of some less generous paytables, which differ from 9/6 only in what the
// full house and the flush pay.
func TestReturnOtherTables(t *testing.T) {
	tests := []struct {
		fullHouse, flush int
		want             float64
	}{
		{8, 5, 0.972984},
		{7, 5, 0.961472},
		{6, 5, 0.949961},
	}
	for _, tt := range tests {
		p := NineSix
		p.Pays[FullHouse], p.Pays[Flush] = tt.fullHouse, tt.flush
		if rtp := NewOdds(&p, MaxCoins).Return(); math.Abs(rtp-tt.want) > 1e-6 {
			t.Errorf("%d/%d: return is %.6f, want %.6f", tt.fullHouse, tt.flush, rtp, tt.want)
		}
	}
}

// TestFrequencies reports how often each hand is made with the best play,
// and what share of the return it brings in.
func TestFrequencies(t *testing.T) {
	freq := maxOdds.Frequencies()
	sum, rtp := 0.0, 0.0
	for c := NumCategories - 1; c >= Nothing; c-- {
		share := freq[c] * float64(NineSix.Pay(c, MaxCoins)) / MaxCoins
		sum += freq[c]
		rtp += share
		t.Logf("%-16s %9.5f%% of hands (once in %.1f), %6.2f%% of what is bet", c, freq[c]*100, 1/freq[c], share*100)
		if freq[c] <= 0 {
			t.Errorf("%v is never made", c)
		}
	}
	if math.Abs(sum-1) > 1e-12 {
		t.Errorf("the frequencies add up to %v", sum)
	}
	if want := maxOdds.Return(); math.Abs(rtp-want) > 1e-9 {
		t.Errorf("the hands pay back %.9f between them, but the return is %.9f", rtp, want)
	}

	// The published figures for the hands that best play leaves no choice
	// about. Others shift a little with how ties between holds are broken.
	published := map[Category]float64{
		RoyalFlush:    0.00002476,
		StraightFlush: 0.00010931,
		FourOfAKind:   0.00236255,
		FullHouse:     0.01151221,
	}
	for c, want := range published {
		if math.Abs(freq[c]-want) > 5e-8 {
			t.Errorf("%v is made %.8f of the time, want %.8f", c, freq[c], want)
		}
	}
}

// TestBest checks some hands where the best play is well known.
func TestBest(t *testing.T) {
	tests := []struct {
		hand, hold string // hold has an x under each card to keep
		why        string
	}{
		{"Ts Js Qs Ks As", "x x x x x", "a royal flush is kept"},
		{"9s Ts Js Qs Ks", "x x x x x", "a straight flush is not broken for a royal"},
		{"As Ks Qs Js 3s", "x x x x .", "a flush is broken for four to a royal"},
		{"As Ks Qs Js Td", "x x x x .", "a straight is broken for four to a royal"},
		{"7c 7d 7h 2s 2c", "x x x x x", "a full house is kept"},
		{"7c 7d 7h Ks 2c", "x x x . .", "three of a kind draws two"},
		{"Jh Jd 4c 4s 9h", "x x x x .", "two pair draws one"},
		{"Kh Kd 5h 9h 2h", "x x . . .", "a high pair beats four to a flush"},
		{"4h 4d 5h 9h 2h", "x . x x x", "four to a flush beats a low pair"},
		{"2c 2d 7h 9s Kd", "x x . . .", "a low pair beats a high card"},
		{"2c 5d 7h 9s Jd", ". . . . x", "a lone high card is kept"},
		{"3c 5d 7h 9s Tc", ". . . . .", "nothing worth keeping"},
		{"Kc Qd 7h 4s 2c", "x x . . .", "two high cards are kept"},
	}
	for _, tt := range tests {
		var want [HandSize]bool
		for i := range want {
			want[i] = tt.hold[i*2] == 'x'
		}
		h := hand(tt.hand)
		got, value := maxOdds.Best(h)
		if got != want {
			t.Errorf("%s: held %v, want %v (%s)", tt.hand, got, want, tt.why)
		}
		if long := drawValue(&NineSix, MaxCoins, h, got); math.Abs(value-long) > 1e-9 {
			t.Errorf("%s: worth %v, but the draws average %v", tt.hand, value, long)
		}
	}
}

// TestBestAgainstDraws checks, for a few hands, that no hold is worth more
// than the one chosen, working each one out the long way.
func TestBestAgainstDraws(t *testing.T) {
	hands := []string{"4c 5d 6h 7s 7c", "Ah Kd 3s 3c Qh", "Ts 9s 8s 2d 2h", "Ac Jc 5c 4d 3h"}
	if testing.Short() {
		hands = hands[:1]
	}
	for _, s := range hands {
		h := hand(s)
		for _, coins := range []int{1, MaxCoins} {
			got, value := NewOdds(&NineSix, coins).Best(h)
			top := -1.0
			for hold := range numHolds {
				var held [HandSize]bool
				for i := range held {
					held[i] = hold>>i&1 == 1
				}
				v := drawValue(&NineSix, coins, h, held)
				top = max(top, v)
				if held == got && math.Abs(v-value) > 1e-9 {
					t.Errorf("%s at %d coins: holding %v is worth %v, not %v", s, coins, held, v, value)
				}
			}
			if math.Abs(top-value) > 1e-9 {
				t.Errorf("%s at %d coins: held %v for %v, but a hold is worth %v", s, coins, got, value, top)
			}
		}
	}
}

// TestBestOrder checks that the cards to hold follow the cards when the
// hand is put in a different order.
func TestBestOrder(t *testing.T) {
	h := hand("Kh 5h Kd 2h 9h")
	held, value := maxOdds.Best(h)
	if held != [HandSize]bool{0: true, 2: true} {
		t.Errorf("%v: held %v", h, held)
	}
	sorted, sortedValue := maxOdds.Best(hand("2h 5h 9h Kd Kh"))
	if sorted != [HandSize]bool{3: true, 4: true} || sortedValue != value {
		t.Errorf("in order: held %v for %v, want the kings for %v", sorted, sortedValue, value)
	}
}

// TestPlay plays a great many hands on a machine with the best holds, and
// checks that what comes back is close to the return worked out.
func TestPlay(t *testing.T) {
	hands := 2_000_000
	if testing.Short() {
		hands = 200_000
	}
	m := New(bank.New(), 11, 12)
	m.Coins = MaxCoins
	bet, won := 0, 0
	var made [NumCategories]int
	for range hands {
		m.Bank.Credits = bank.StartCredits
		if _, ok := m.Deal(); !ok {
			t.Fatal("could not deal")
		}
		m.Held, _ = maxOdds.Best(m.Hand)
		r, _ := m.Draw()
		bet += m.Bet()
		won += r.Win
		made[r.Made]++
	}
	played := float64(won) / float64(bet)
	want := maxOdds.Return()
	t.Logf("%d hands paid back %.2f%%; worked out as %.2f%%", hands, played*100, want*100)

	// The royal flush is rare and worth a lot, so the return over even
	// this many hands wanders by about a third of a percent.
	margin := 0.01
	if testing.Short() {
		margin = 0.03
	}
	if math.Abs(played-want) > margin {
		t.Errorf("play paid back %.4f, want %.4f give or take %.2f", played, want, margin)
	}

	// The commoner hands should turn up as often as worked out.
	freq := maxOdds.Frequencies()
	for c := Nothing; c <= FullHouse; c++ {
		got := float64(made[c]) / float64(hands)
		sd := math.Sqrt(freq[c] * (1 - freq[c]) / float64(hands))
		if math.Abs(got-freq[c]) > 5*sd {
			t.Errorf("%v came up %.5f of the time, want %.5f", c, got, freq[c])
		}
	}
}
