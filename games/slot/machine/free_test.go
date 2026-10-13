package machine

import (
	"math"
	"testing"

	"videocasino/bank"
)

func TestFreeStrips(t *testing.T) {
	for reel, strip := range FreeStrips {
		if reel >= BigFirst && reel <= BigLast {
			if strip != nil {
				t.Errorf("reel %d has a strip for free games, but shows the giant symbol", reel+1)
			}
			continue
		}
		n := len(strip)
		wilds := 0
		for i, sym := range strip {
			switch {
			case sym == Wild:
				wilds++
			case !sym.Paying():
				t.Errorf("reel %d has a %v on its free-game strip", reel+1, sym)
			}
			if sym == strip[(i+1)%n] || sym == strip[(i+2)%n] {
				t.Errorf("reel %d: %v comes twice within three stops at %d", reel+1, sym, i)
			}
		}
		if want := reel == Reels-1; (wilds > 0) != want {
			t.Errorf("reel %d has %d wilds on its free-game strip", reel+1, wilds)
		}
	}
	for sym, w := range BigWeights {
		if w <= 0 {
			t.Errorf("%v never comes up as the giant symbol", Symbol(sym))
		}
	}
}

// In a free game the three middle reels match, the joker does not double,
// and nothing pays for fewer than four.
func TestFreePays(t *testing.T) {
	const (
		k = King
		b = Bell
		d = Dollar
		w = Wild
	)
	tests := []struct {
		line [Reels]Symbol
		pay  int
		sym  Symbol
		n    int
	}{
		{[Reels]Symbol{k, k, k, k, b}, 5, k, 4},
		{[Reels]Symbol{k, k, k, k, k}, 15, k, 5},
		{[Reels]Symbol{k, k, k, k, w}, 15, k, 5},
		{[Reels]Symbol{k, w, w, w, b}, 5, k, 4},
		{[Reels]Symbol{d, w, w, w, w}, 100, d, 5},
		{[Reels]Symbol{d, w, w, w, d}, 100, d, 5},
		{[Reels]Symbol{k, b, b, b, b}, 0, 0, 0},
		{[Reels]Symbol{k, b, b, b, k}, 0, 0, 0},
		{[Reels]Symbol{k, Scatter, Scatter, Scatter, k}, 0, 0, 0},
		{[Reels]Symbol{k, Link, Link, Link, w}, 0, 0, 0},
	}
	for _, tt := range tests {
		pay, sym, n := evaluate(tt.line, &FreePays, 1)
		if pay != tt.pay || sym != tt.sym || n != tt.n {
			t.Errorf("free game %v pays %d for %d %v; want %d for %d %v", tt.line, pay, n, sym, tt.pay, tt.n, tt.sym)
		}
	}
	for sym, pays := range FreePays {
		if pays[2] != 0 || pays[3] != 0 {
			t.Errorf("%v pays for fewer than four in a free game", Symbol(sym))
		}
	}
}

// freeLineReturn works from one payline. Check it against every payline
// over every way a free game can stop.
func TestFreeLineReturn(t *testing.T) {
	first, last := FreeStrips[0], FreeStrips[Reels-1]
	total := 0
	for big, w := range BigWeights {
		for a := range first {
			for b := range last {
				var grid [Reels][Rows]Symbol
				grid[0], grid[Reels-1] = FreeWindow(0, a), FreeWindow(Reels-1, b)
				for i := BigFirst; i <= BigLast; i++ {
					grid[i] = [Rows]Symbol{Symbol(big), Symbol(big), Symbol(big)}
				}
				for _, rows := range Paylines {
					var line [Reels]Symbol
					for i, row := range rows {
						line[i] = grid[i][row]
					}
					pay, _, _ := evaluate(line, &FreePays, 1)
					total += w * pay
				}
			}
		}
	}
	want := float64(total) / float64(bigTotal()*len(first)*len(last)*len(Paylines))
	if got := freeLineReturn(); math.Abs(got-want) > 1e-9 {
		t.Errorf("freeLineReturn() = %v, want %v", got, want)
	}
}

// Free games played for real should deal each giant symbol as often as its
// weight says, and a giant chip should hold nine cells with one prize.
func TestFreeGames(t *testing.T) {
	const games = 2_000_000
	m := New(bank.New(), 11, 12)
	m.Lines, m.BetPerLine = len(Paylines), 1
	var dealt [NumSymbols]int
	for range games {
		m.FreeSpins = 1
		r, _ := m.Spin()
		dealt[r.Big]++
		if r.Big == Link {
			l := m.Link
			if l == nil || !l.Big || l.Count() != 9 || l.Respins != LinkRespins {
				t.Fatalf("a giant chip left the feature as %+v", l)
			}
			prize := l.Chips[BigReel][Middle]
			if prize.Jackpot == NoJackpot && prize.Bets < 5 {
				t.Fatalf("a giant chip carries %+v", prize)
			}
			if got, want := m.LinkTotal(), m.ChipValue(prize, l.Bet); got != want {
				t.Fatalf("a giant chip is worth %d, want its one prize of %d", got, want)
			}
			playLink(t, m)
		}
		m.FreeSpins = 0
	}
	for sym, n := range dealt {
		want := bigOdds(Symbol(sym))
		if got := float64(n) / games; math.Abs(got-want) > want/20 {
			t.Errorf("giant %v on %.4f of free games, want about %.4f", Symbol(sym), got, want)
		}
	}
}
