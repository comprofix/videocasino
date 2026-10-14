package roulette

import (
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/games/roulette/wheel"
	"videocasino/ui"
)

// Only one set of sounds can be made in a program, so the tests share it.
var testSounds = sync.OnceValue(func() *ui.Sounds {
	s := ui.NewSounds()
	s.On = false
	return s
})

// testGame returns a game with fixed spins and a bank that is not saved
// anywhere.
func testGame() *Game {
	b := bank.New()
	g := &Game{snd: testSounds(), bank: b, m: wheel.New(b, 1, 2)}
	g.mark = middle(0)
	g.credits = b.Credits
	return g
}

// land lets time pass until the ball is at rest.
func land(t *testing.T, g *Game) {
	t.Helper()
	for range 2 * landTicks {
		if !g.spinning {
			return
		}
		g.ticks++
		g.roll(false)
	}
	t.Fatal("the ball never came to rest")
}

// bet moves the marker to a number and puts a chip on it.
func bet(g *Game, n int) {
	g.mark = middle(n)
	g.place()
}

// TestTable checks that every bet has a place of its own on the table, and
// that the marker can be walked to every one of them and nowhere else.
func TestTable(t *testing.T) {
	places := map[[2]float64]int{}
	for spot, p := range spotPoint {
		got, ok := spotAt(p)
		if !ok || got != spot {
			t.Errorf("%v is at %v, where the marker finds spot %d", wheel.Spots[spot], p, got)
		}
		x, y := chipAt(p)
		if other, dup := places[[2]float64{x, y}]; dup {
			t.Errorf("%v and %v have their chips in the same place", wheel.Spots[spot], wheel.Spots[other])
		}
		places[[2]float64{x, y}] = spot

		// A click where a chip lies bets on that spot.
		if at, ok := pointAt(int(x), int(y)); !ok {
			t.Errorf("a click on %v misses the table", wheel.Spots[spot])
		} else if got, _ := spotAt(at); got != spot {
			t.Errorf("a click on %v bets on %v", wheel.Spots[spot], wheel.Spots[got])
		}
	}

	// Walk the marker everywhere it will go.
	reached := map[int]bool{}
	seen := map[point]bool{middle(0): true}
	for todo := []point{middle(0)}; len(todo) > 0; {
		p := todo[0]
		todo = todo[1:]
		spot, ok := spotAt(p)
		if !ok {
			t.Fatalf("the marker can be moved to %v, where there is no bet", p)
		}
		reached[spot] = true
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			if q := move(p, d[0], d[1]); !seen[q] {
				seen[q] = true
				todo = append(todo, q)
			}
		}
	}
	if len(reached) != len(wheel.Spots) {
		t.Errorf("the marker reaches %d of the %d bets", len(reached), len(wheel.Spots))
	}

	// A few of the steps: off the zero onto its split with 2, along a row
	// to its column, and down off the numbers to a dozen.
	if p := move(middle(0), 1, 0); wheel.Spots[pointSpot[p]].String() != "SPLIT 0/2" {
		t.Errorf("one step right of the zero is %v", wheel.Spots[pointSpot[p]])
	}
	if s := wheel.Spots[pointSpot[move(middle(36), 1, 0)]]; s.Kind != wheel.Column || !s.Covers(36) {
		t.Errorf("one step right of 36 is %v", s)
	}
	p := middle(14)
	for range 4 {
		p = move(p, 0, 1)
	}
	if spot, _ := spotAt(p); wheel.Spots[spot].String() != "2ND 12" {
		t.Errorf("down from 14 is %v", wheel.Spots[spot])
	}

	// A click in the middle of a number is on the number, near its side on
	// the split with the next, and near its corner on the corner.
	b := numberButton(17)
	for _, c := range []struct {
		dx, dy float64
		want   string
	}{
		{cellW / 2, cellH / 2, "STRAIGHT UP 17"},
		{cellW - 3, cellH / 2, "SPLIT 17/20"},
		{3, cellH / 2, "SPLIT 14/17"},
		{cellW / 2, 3, "SPLIT 17/18"},
		{cellW - 3, cellH - 3, "CORNER 16/17/19/20"},
	} {
		at, _ := pointAt(int(b.X+c.dx), int(b.Y+c.dy))
		if spot, _ := spotAt(at); wheel.Spots[spot].String() != c.want {
			t.Errorf("a click at %v, %v in 17 is on %v, want %s", c.dx, c.dy, wheel.Spots[spot], c.want)
		}
	}
	if _, ok := pointAt(wheelX, wheelY); ok {
		t.Error("a click on the wheel is on the table")
	}
}

// TestBank checks that the game plays from the bank it is given, and that
// a coin or a reset made in the game is made to the bank.
func TestBank(t *testing.T) {
	g := testGame()
	g.bank.Pay(2500) // won somewhere else
	g.m.Chip = 100
	for n := range wheel.Pockets {
		bet(g, n)
	}
	if g.m.Total() != 3700 || g.bank.Credits != bank.StartCredits+2500 {
		t.Fatalf("%d on the table, %d in the bank before the spin", g.m.Total(), g.bank.Credits)
	}

	// With a chip on every number one of them wins, whatever comes up.
	g.spin()
	if !g.spinning {
		t.Fatal("the ball is not running after a spin")
	}
	if g.result.Bet != 3700 || g.result.Win != 3600 {
		t.Fatalf("bet %d and won %d", g.result.Bet, g.result.Win)
	}
	if want := bank.StartCredits + 2500 - 100; g.bank.Credits != want {
		t.Errorf("the bank has %d credits, want %d", g.bank.Credits, want)
	}
	// The win is not shown until the ball is at rest.
	if g.credits != g.bank.Credits-3600 || g.win != 0 || g.shown {
		t.Errorf("while the ball runs: %d credits and a win of %d on show", g.credits, g.win)
	}
	land(t, g)
	if g.credits != g.bank.Credits || g.win != 3600 || !g.shown {
		t.Errorf("once the ball is at rest: %d credits and a win of %d on show", g.credits, g.win)
	}
	if g.number != g.result.Number || g.m.History[0] != g.number {
		t.Errorf("the ball is in %d, the result is %d and the history %v", g.number, g.result.Number, g.m.History)
	}

	had := g.bank.Credits
	g.insertCoin()
	if g.bank.Credits != had+bank.CoinCredits || g.credits != g.bank.Credits {
		t.Errorf("after a coin the bank has %d credits, with %d on show", g.bank.Credits, g.credits)
	}

	// A reset leaves the bets as they are.
	bet(g, 5)
	g.reset()
	if g.bank.Credits != bank.StartCredits || g.credits != bank.StartCredits {
		t.Errorf("after a reset: %d credits, with %d on show", g.bank.Credits, g.credits)
	}
	if g.m.Total() != 100 || g.m.LastTotal() != 3700 {
		t.Errorf("a reset changed the bets: %d on the table, %d remembered", g.m.Total(), g.m.LastTotal())
	}
}

func TestSpinWithoutCredits(t *testing.T) {
	g := testGame()

	// Nothing bet: nothing to spin.
	g.spin()
	if g.spinning || g.bank.Credits != bank.StartCredits {
		t.Error("spun with nothing on the table")
	}

	g.m.Chip = 100
	bet(g, 7)
	g.bank.Credits, g.credits = 99, 99
	g.spin()
	if g.spinning || g.bank.Credits != 99 || g.m.Total() != 100 {
		t.Errorf("spun for 100 with 99 credits")
	}
	// No more can be put down than the bank has.
	bet(g, 8)
	if g.m.Total() != 100 {
		t.Errorf("%d on the table with 99 credits", g.m.Total())
	}

	// With none at all nothing is spun, and the bank is left for the
	// player to fill: it is every game's, so the spin button does not
	// reset it.
	g.bank.Credits = 0
	g.spin()
	if g.spinning || g.bank.Credits != 0 {
		t.Errorf("with no credits: spinning %v, credits %d", g.spinning, g.bank.Credits)
	}
	if g.message != outOfCredits {
		t.Errorf("with no credits the game says %q", g.message)
	}

	// A coin lets the spin be paid for.
	g.insertCoin()
	g.spin()
	if !g.spinning || g.bank.Credits != bank.CoinCredits-100+g.result.Win {
		t.Errorf("after a coin: spinning %v, credits %d", g.spinning, g.bank.Credits)
	}
}

// TestRebet checks that the last bets can be made again, and that a spin
// with nothing on the table makes them.
func TestRebet(t *testing.T) {
	g := testGame()
	g.rebet()
	if g.m.Total() != 0 {
		t.Error("made bets again before any had been made")
	}

	g.m.Chip = 50
	bet(g, 17)
	bet(g, 17)
	g.mark = spotPoint[evenSpots[2]] // red
	g.place()
	g.spin()
	land(t, g)
	want := slices.Clone(g.m.Last)
	if g.m.Total() != 0 || g.m.LastTotal() != 150 {
		t.Fatalf("after a spin: %d on the table, %d remembered", g.m.Total(), g.m.LastTotal())
	}

	had := g.bank.Credits
	g.spin()
	if !g.spinning || g.result.Bet != 150 || g.bank.Credits != had-150+g.result.Win {
		t.Errorf("a spin with nothing on the table bet %d", g.result.Bet)
	}
	land(t, g)
	if !slices.Equal(g.m.Last, want) {
		t.Error("the bets made again are not the same")
	}

	// Taking chips off, and clearing the table.
	g.rebet()
	g.mark = middle(17)
	g.remove()
	if g.m.Bets[17] != 50 || g.m.Total() != 100 {
		t.Errorf("taking a chip off 17 left %d there and %d in all", g.m.Bets[17], g.m.Total())
	}
	g.clear()
	if g.m.Total() != 0 {
		t.Errorf("a cleared table has %d on it", g.m.Total())
	}
}

// TestBall checks the run of the ball: it stays on the wheel, ends in the
// pocket of the number that came up, and rides round with the wheel from
// then on.
func TestBall(t *testing.T) {
	g := testGame()
	for range 20 {
		g.m.Chip = 10
		bet(g, 0)
		g.spin()
		if _, r := g.ball(); r != trackR {
			t.Fatalf("the ball sets off %v from the middle, not on the rim", r)
		}
		last, _ := g.ball()
		for g.spinning {
			g.ticks++
			g.roll(false)
			angle, r := g.ball()
			if r < pocketR-0.001 || r > trackR+0.001 {
				t.Fatalf("the ball is %v from the middle", r)
			}
			// It never jumps: not even as it comes to rest.
			if math.Abs(angle-last) > 0.5 {
				t.Fatalf("the ball jumped from %v to %v", last, angle)
			}
			last = angle
		}
		if g.ticks-g.spinStart != landTicks {
			t.Fatalf("the ball took %d ticks to come to rest", g.ticks-g.spinStart)
		}

		// At rest it is over its number, and stays there as the wheel turns.
		for range coastTicks {
			g.ticks++
			angle, r := g.ball()
			pocket := math.Mod(angle-g.wheelAngle(), 2*math.Pi) / pocketTurn
			if r != pocketR || int(math.Round(pocket))%wheel.Pockets != wheel.Pocket(g.result.Number) {
				t.Fatalf("the ball is in pocket %v, %v from the middle; %d came up", pocket, r, g.result.Number)
			}
		}
		was := g.wheelAngle()
		g.ticks += 100
		if g.wheelAngle() != was {
			t.Fatal("the wheel never stops")
		}
	}

	// A spin can be cut short.
	bet(g, 0)
	g.spin()
	g.ticks++
	g.roll(true)
	if g.spinning || !g.shown {
		t.Error("the spin was not cut short")
	}
}

// TestSaveAndResume closes the game with bets on the table and opens it
// again from the save file.
func TestSaveAndResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	g := New(bank.Open(path), testSounds())
	g.setChip(500)
	bet(g, 17)
	g.spin()
	land(t, g)
	g.setChip(100)
	bet(g, 3)
	g.mark = spotPoint[dozenSpots[1]]
	g.place()

	again := New(bank.Open(path), testSounds())
	if want, got := g.m.Save(), again.m.Save(); !reflect.DeepEqual(got, want) {
		t.Errorf("resumed with %+v, want %+v", got, want)
	}
	if again.bank.Credits != g.bank.Credits || again.credits != again.bank.Credits {
		t.Errorf("resumed with %d credits and %d on show, want %d", again.bank.Credits, again.credits, g.bank.Credits)
	}
	if again.m.Total() != 200 || again.m.LastTotal() != 500 || again.m.Chip != 100 {
		t.Errorf("resumed with %d on the table, %d remembered and a chip of %d", again.m.Total(), again.m.LastTotal(), again.m.Chip)
	}
	// The ball lies in the last number to come up.
	if !again.hasBall || again.number != g.result.Number || again.spinning {
		t.Errorf("the ball is in %d, want %d", again.number, g.result.Number)
	}

	// The bets left on the table can be spun.
	had := again.bank.Credits
	again.spin()
	if !again.spinning || again.bank.Credits != had-200+again.result.Win {
		t.Errorf("the spin: credits from %d to %d on a win of %d", had, again.bank.Credits, again.result.Win)
	}
}
