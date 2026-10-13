package machine

import (
	"math"
	"testing"

	"videocasino/bank"
)

func TestEvaluate(t *testing.T) {
	const (
		t0 = Ten
		a  = Ace
		c  = Cherry
		p  = Plum
		b  = Bell
		d  = Dollar
		w  = Wild
		x  = Scatter
		k  = Link
	)
	tests := []struct {
		line    [Reels]Symbol
		pay     int
		sym     Symbol
		matched int
	}{
		{[Reels]Symbol{d, d, d, d, d}, 150, d, 5},
		{[Reels]Symbol{d, d, d, d, b}, 50, d, 4},
		{[Reels]Symbol{d, d, d, b, d}, 10, d, 3},
		{[Reels]Symbol{b, b, b, b, b}, 100, b, 5},
		{[Reels]Symbol{t0, t0, t0, a, a}, 5, t0, 3},
		{[Reels]Symbol{a, a, a, a, c}, 20, a, 4},
		{[Reels]Symbol{c, c, c, c, c}, 100, c, 5},
		{[Reels]Symbol{c, c, b, c, c}, 0, 0, 0},
		{[Reels]Symbol{c, p, c, c, c}, 0, 0, 0},
		{[Reels]Symbol{p, p, b, p, p}, 0, 0, 0},
		{[Reels]Symbol{p, b, b, b, b}, 0, 0, 0},

		// Wilds stand in for the symbol the line starts with, and double the
		// win however many of them are in it.
		{[Reels]Symbol{b, w, b, p, p}, 10, b, 3},
		{[Reels]Symbol{b, w, w, w, b}, 200, b, 5},
		{[Reels]Symbol{b, w, w, w, w}, 200, b, 5},
		{[Reels]Symbol{c, w, p, p, p}, 0, 0, 0},
		{[Reels]Symbol{p, w, c, c, c}, 0, 0, 0},
		{[Reels]Symbol{w, w, a, a, p}, 40, a, 4},
		{[Reels]Symbol{w, w, w, w, w}, 0, 0, 0},
		{[Reels]Symbol{b, b, b, p, w}, 5, b, 3}, // a wild outside the win

		// Only the dollar pays for a pair.
		{[Reels]Symbol{d, d, p, p, p}, 2, d, 2},
		{[Reels]Symbol{d, w, p, p, p}, 4, d, 2},
		{[Reels]Symbol{t0, t0, a, t0, t0}, 0, 0, 0},
		{[Reels]Symbol{a, a, p, p, p}, 0, 0, 0},

		// Scatters and chips never win along a line, with or without wilds.
		{[Reels]Symbol{x, x, x, x, x}, 0, 0, 0},
		{[Reels]Symbol{k, k, k, k, k}, 0, 0, 0},
		{[Reels]Symbol{x, w, w, w, w}, 0, 0, 0},
		{[Reels]Symbol{w, w, k, k, k}, 0, 0, 0},
		{[Reels]Symbol{b, b, x, b, b}, 0, 0, 0},
	}
	for _, tt := range tests {
		pay, sym, matched := Evaluate(tt.line)
		if pay != tt.pay || sym != tt.sym || matched != tt.matched {
			t.Errorf("Evaluate(%v) = %d, %v, %d; want %d, %v, %d",
				tt.line, pay, sym, matched, tt.pay, tt.sym, tt.matched)
		}
	}
}

func TestStrips(t *testing.T) {
	for reel, strip := range Strips {
		n := len(strip)
		stacks := 0
		for i, sym := range strip {
			if sym == Link {
				// Chips come in one stack of three.
				if strip[(i+n-1)%n] != Link {
					stacks++
					if strip[(i+1)%n] != Link || strip[(i+2)%n] != Link || strip[(i+3)%n] == Link {
						t.Errorf("reel %d: the chips at %d are not a stack of three", reel+1, i)
					}
				}
				continue
			}
			if sym == strip[(i+1)%n] || sym == strip[(i+2)%n] {
				t.Errorf("reel %d: %v comes twice within three stops at %d", reel+1, sym, i)
			}
		}
		if stacks != 1 {
			t.Errorf("reel %d has %d stacks of chips, want 1", reel+1, stacks)
		}
		if count(reel, Scatter) == 0 {
			t.Errorf("reel %d has no scatter", reel+1)
		}
	}
	if n := count(0, Wild); n != 0 {
		t.Errorf("reel 1 has %d wilds, want none", n)
	}
	for reel := 1; reel < Reels; reel++ {
		if count(reel, Wild) == 0 {
			t.Errorf("reel %d has no wild", reel+1)
		}
	}
}

func TestRTP(t *testing.T) {
	// The return is quoted at every line and one credit a line.
	bet := len(Paylines)
	r := ReturnAt(bet)
	t.Logf("return to player %.2f%% at a bet of %d: lines %.2f%%, dice %.2f%%, link %.2f%%, major %.2f%%, grand %.2f%%, pot growth %.2f%%",
		r.Total()*100, bet, r.Lines*100, r.Dice*100, r.Link*100, r.Major*100, r.Grand*100, r.Growth*100)
	t.Logf("free games won 1 spin in %.0f, %.1f played each time; link feature 1 spin in %.0f", 1/FreeSpinOdds(), freeGames()/FreeSpinOdds(), 1/LinkOdds())
	t.Logf("return at a bet of 1: %.2f%%; at %d: %.2f%%", RTP(1)*100, MaxBet, RTP(MaxBet)*100)
	if got := r.Total(); math.Abs(got-0.92) > 0.005 {
		t.Errorf("RTP(%d) = %v, want about 0.92", bet, got)
	}
	// Only the grand pot should make the return depend on the bet.
	small, large := ReturnAt(1), ReturnAt(MaxBet)
	small.Grand, large.Grand = 0, 0
	if math.Abs(small.Total()-large.Total()) > 2e-4 {
		t.Errorf("return without the grand pot is %v at the smallest bet and %v at the largest", small.Total(), large.Total())
	}
}

// lineReturn works the odds out from symbol counts. Check it against every
// combination of stops on one payline.
func TestLineReturn(t *testing.T) {
	total, combos := 0, 0
	var line [Reels]Symbol
	for _, line[0] = range Strips[0] {
		for _, line[1] = range Strips[1] {
			for _, line[2] = range Strips[2] {
				for _, line[3] = range Strips[3] {
					for _, line[4] = range Strips[4] {
						pay, _, _ := Evaluate(line)
						total += pay
						combos++
					}
				}
			}
		}
	}
	want := float64(total) / float64(combos)
	if got := lineReturn(); math.Abs(got-want) > 1e-9 {
		t.Errorf("lineReturn() = %v, want %v", got, want)
	}
}

func TestWindow(t *testing.T) {
	for reel, strip := range Strips {
		n := len(strip)
		for stop := range strip {
			w := Window(reel, stop)
			want := [Rows]Symbol{strip[(stop+1)%n], strip[stop], strip[(stop+n-1)%n]}
			if w != want {
				t.Fatalf("Window(%d, %d) = %v, want %v", reel, stop, w, want)
			}
		}
	}
}

func TestPaylines(t *testing.T) {
	if len(Paylines) != 50 {
		t.Fatalf("%d paylines, want 50", len(Paylines))
	}
	seen := map[[Reels]int]int{}
	for i, line := range Paylines {
		for _, row := range line {
			if row < Top || row > Bottom {
				t.Errorf("payline %d has row %d", i+1, row)
			}
		}
		if j, ok := seen[line]; ok {
			t.Errorf("paylines %d and %d are the same", j+1, i+1)
		}
		seen[line] = i
	}
	if got := LineChoices[len(LineChoices)-1]; got != len(Paylines) {
		t.Errorf("largest line choice is %d, want %d", got, len(Paylines))
	}
}

// The result of every spin should agree with the grid it reports, line by
// line, and free games should cost nothing and follow their own rules.
func TestSpinAccounting(t *testing.T) {
	m := New(bank.New(), 1, 2)
	freeGames, links, bigLinks := 0, 0, 0
	for n := range 300000 {
		if m.FreeSpins == 0 {
			m.Bank.Credits = bank.StartCredits
			m.Lines = LineChoices[n%len(LineChoices)]
			m.BetPerLine = BetChoices[n/len(LineChoices)%len(BetChoices)]
		}
		before, waiting := m.Bank.Credits, m.FreeSpins
		major, grand := m.majorPot, m.grandPot

		r, ok := m.Spin()
		if !ok {
			t.Fatalf("Spin refused with %d credits and bet %d", before, m.TotalBet())
		}
		free := waiting > 0
		cost, pays, wild := m.TotalBet(), &Pays, WildMultiplier
		if free {
			cost, pays, wild = 0, &FreePays, 1
			freeGames++
		}
		if r.Free != free {
			t.Fatalf("Free = %v with %d free games waiting", r.Free, waiting)
		}
		if m.majorPot != major+cost*majorGrowth || m.grandPot != grand+cost*grandGrowth {
			t.Fatalf("pots grew by %d and %d on a spin costing %d", m.majorPot-major, m.grandPot-grand, cost)
		}

		// The grid: every reel from its strip on a paid spin; on a free
		// game reels 1 and 5 from theirs and the giant symbol between.
		scatters, chips := 0, 0
		for i, stop := range r.Stops {
			want := Window(i, stop)
			switch {
			case free && i >= BigFirst && i <= BigLast:
				want = [Rows]Symbol{r.Big, r.Big, r.Big}
			case free:
				want = FreeWindow(i, stop)
			}
			if r.Grid[i] != want {
				t.Fatalf("reel %d shows %v, want %v", i+1, r.Grid[i], want)
			}
			for row, s := range r.Grid[i] {
				switch s {
				case Scatter:
					scatters++
				case Link:
					chips++
					c := r.Chips[i][row]
					empty := c.Jackpot == NoJackpot && c.Bets == 0
					// Only the middle cell of a giant chip carries its prize.
					if want := free && !(i == BigReel && row == Middle); empty != want {
						t.Fatalf("the chip at reel %d row %d carries %+v", i+1, row, c)
					}
				}
			}
		}

		var wins []LineWin
		total := 0
		for l := range m.Lines {
			var line [Reels]Symbol
			for i, row := range Paylines[l] {
				line[i] = r.Grid[i][row]
			}
			if pay, sym, matched := evaluate(line, pays, wild); pay > 0 {
				win := pay * m.BetPerLine
				wins = append(wins, LineWin{l, sym, matched, pay > pays[sym][matched], win})
				total += win
			}
		}
		if len(wins) != len(r.Wins) {
			t.Fatalf("%d winning lines, want %d", len(r.Wins), len(wins))
		}
		for i := range wins {
			if r.Wins[i] != wins[i] {
				t.Fatalf("win %d = %+v, want %+v", i, r.Wins[i], wins[i])
			}
		}

		// Dice: by their number on a paid spin; a giant one on a free game.
		scatterWin, awarded := ScatterPays[min(scatters, Reels)]*m.TotalBet(), FreeSpinAwards[min(scatters, Reels)]
		if free {
			scatterWin, awarded = 0, 0
			if r.Big == Scatter {
				scatterWin, awarded = BigScatterPay*m.TotalBet(), BigScatterGames
			}
		}
		total += scatterWin
		if r.Scatters != scatters || r.ScatterWin != scatterWin || r.FreeSpinsWon != awarded {
			t.Fatalf("scatters %d paying %d with %d free games; want %d, %d, %d",
				r.Scatters, r.ScatterWin, r.FreeSpinsWon, scatters, scatterWin, awarded)
		}
		if r.Win != total || m.Bank.Credits != before-cost+total {
			t.Fatalf("win %d and credits %d, want %d and %d", r.Win, m.Bank.Credits, total, before-cost+total)
		}
		if want := max(waiting-1, 0) + awarded; m.FreeSpins != want {
			t.Fatalf("%d free games waiting, want %d", m.FreeSpins, want)
		}

		// Six or more chips start the link feature, as does a giant chip. It
		// must be played out before the next spin.
		started := chips >= LinkTrigger
		if r.LinkStarted != started || (m.Link != nil) != started {
			t.Fatalf("%d chips showing: LinkStarted = %v, feature in progress = %v", chips, r.LinkStarted, m.Link != nil)
		}
		if started {
			links++
			if m.Link.Big {
				bigLinks++
			}
			if m.Link.Count() != chips || m.Link.Bet != m.TotalBet() || m.Link.Big != free {
				t.Fatalf("feature holds %d chips at bet %d (giant chip: %v), want %d at %d", m.Link.Count(), m.Link.Bet, m.Link.Big, chips, m.TotalBet())
			}
			if _, ok := m.Spin(); ok {
				t.Fatal("Spin succeeded during a link feature")
			}
			credits := m.Bank.Credits
			end := playLink(t, m)
			if m.Link != nil || m.Bank.Credits != credits+end.Win {
				t.Fatalf("feature paid %d: credits went from %d to %d", end.Win, credits, m.Bank.Credits)
			}
		}
	}
	if freeGames == 0 || links == 0 || bigLinks == 0 {
		t.Errorf("played %d free games and %d link features, %d from a giant chip; want some of each", freeGames, links, bigLinks)
	}
}

// A long run of real spins on all lines, features included, should pay back
// close to the calculated return and trigger the features about as often as
// calculated. The two growing pots are left out: they pay too rarely for
// any run to settle near their average.
func TestSpinReturn(t *testing.T) {
	const spins = 4_000_000
	m := New(bank.New(), 3, 4)
	m.Lines = len(Paylines)
	bet, won, paid, frees, links := 0, 0, 0, 0, 0
	played := 0 // paid spins and free games together
	for paid < spins {
		m.Bank.Credits = bank.StartCredits
		r, _ := m.Spin()
		played++
		if !r.Free {
			paid++
			bet += m.TotalBet()
			if r.FreeSpinsWon > 0 {
				frees++
			}
		}
		won += r.Win
		if r.LinkStarted {
			links++
			end := playLink(t, m)
			won += end.Win - end.MajorWin - end.GrandWin
		}
	}
	want := ReturnAt(m.TotalBet())
	got := float64(won) / float64(bet)
	freeOdds := float64(frees) / spins
	linkOdds := float64(links) / spins
	freePlayed := float64(played-paid) / spins
	t.Logf("paid back %.2f%% over %d spins of %d lines; free games won 1 in %.1f, %.4f played per spin; link feature 1 in %.1f",
		got*100, spins, m.Lines, 1/freeOdds, freePlayed, 1/linkOdds)
	if w := want.Lines + want.Dice + want.Link; math.Abs(got-w) > 0.015 {
		t.Errorf("paid back %v, want about %v", got, w)
	}
	if w := FreeSpinOdds(); math.Abs(freeOdds-w) > w/20 {
		t.Errorf("free games won on %v of spins, want about %v", freeOdds, w)
	}
	if w := freeGames(); math.Abs(freePlayed-w) > w/20 {
		t.Errorf("%v free games played per spin, want about %v", freePlayed, w)
	}
	if w := LinkOdds(); math.Abs(linkOdds-w) > w/20 {
		t.Errorf("%v link features per spin, want about %v", linkOdds, w)
	}
}

func TestSpinNeedsCredits(t *testing.T) {
	m := New(bank.New(), 1, 2)
	m.Lines, m.BetPerLine = 25, 20
	m.Bank.Credits = m.TotalBet() - 1
	if _, ok := m.Spin(); ok {
		t.Error("Spin succeeded with too few credits")
	}
	if m.Bank.Credits != m.TotalBet()-1 {
		t.Errorf("credits = %d after refused spin, want %d", m.Bank.Credits, m.TotalBet()-1)
	}

	// A free spin needs no credits.
	m.Bank.Credits, m.FreeSpins = 0, 1
	if r, ok := m.Spin(); !ok || !r.Free {
		t.Error("free spin refused with no credits")
	}
}

func TestInsertCoin(t *testing.T) {
	m := New(bank.New(), 1, 2)
	m.Bank.Credits = 0
	m.Bank.InsertCoin()
	m.Bank.InsertCoin()
	if m.Bank.Credits != 2*bank.CoinCredits {
		t.Errorf("credits = %d after two coins, want %d", m.Bank.Credits, 2*bank.CoinCredits)
	}
	if _, ok := m.Spin(); !ok {
		t.Error("Spin refused after inserting coins")
	}
}

// Fixed seeds repeat the same spins and are left alone; a machine made for
// play differs from every other and is seeded afresh as it goes.
func TestGenerators(t *testing.T) {
	a, b := New(bank.New(), 1, 2), New(bank.New(), 1, 2)
	for range 500 {
		ra, _ := a.Spin()
		rb, _ := b.Spin()
		if ra.Stops != rb.Stops {
			t.Fatal("two machines with the same seeds spun differently")
		}
		for a.Link != nil {
			a.Respin()
			b.Respin()
		}
		a.Bank.Credits, b.Bank.Credits = bank.StartCredits, bank.StartCredits
	}
	if a.reseedIn != 0 {
		t.Error("a machine with fixed seeds is set to take a fresh one")
	}

	c, d := NewRandom(bank.New()), NewRandom(bank.New())
	same := true
	for n := range 3 * reseedEvery {
		rc, _ := c.Spin()
		rd, _ := d.Spin()
		if rc.Stops != rd.Stops {
			same = false
		}
		for c.Link != nil {
			c.Respin()
		}
		for d.Link != nil {
			d.Respin()
		}
		c.Bank.Credits, d.Bank.Credits = bank.StartCredits, bank.StartCredits
		if want := reseedEvery - (n+1)%reseedEvery; c.reseedIn != want {
			t.Fatalf("after %d spins a fresh seed is due in %d, want %d", n+1, c.reseedIn, want)
		}
	}
	if same {
		t.Error("two machines made for play spun the same")
	}
}
