package paigow

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/games/paigow/table"
	"videocasino/ui"
)

// Only one set of sounds can be made in a program, so the tests share it.
var testSounds = sync.OnceValue(func() *ui.Sounds {
	s := ui.NewSounds()
	s.On = false
	return s
})

// testGame returns a game with fixed cards and a bank that is not saved
// anywhere.
func testGame() *Game {
	b := bank.New()
	g := &Game{snd: testSounds(), bank: b, m: table.New(b, 1, 2)}
	g.credits = b.Credits
	return g
}

// settle lets time pass until the shuffle is over and the cards are all
// shown, and returns how many ticks that took.
func settle(t *testing.T, g *Game) int {
	t.Helper()
	for n := range 1000 {
		switch {
		case g.shuffling:
			g.ticks++
			g.shuffleStep(false)
		case g.showing:
			g.ticks++
			g.show(false)
		default:
			return n
		}
	}
	t.Fatal("the cards were never all shown")
	return 0
}

// TestDealSetAndPlay follows a hand from the bet through the shuffle, the
// setting and the dealer's cards to the end.
func TestDealSetAndPlay(t *testing.T) {
	g := testGame()
	g.m.Chip = 2500
	g.place()
	g.pressDeal()
	if !g.shuffling || g.m.Dealt {
		t.Fatalf("after pressing deal: shuffling %v, dealt %v", g.shuffling, g.m.Dealt)
	}
	if ticks := settle(t, g); ticks < shuffleLength.Ticks()+table.HandSize*dealTicks {
		t.Errorf("the shuffle and the deal took %d ticks", ticks)
	}
	if !g.m.Dealt || g.playerShown != table.HandSize || g.message != chooseLow {
		t.Fatalf("after the deal: dealt %v, %d shown, %q", g.m.Dealt, g.playerShown, g.message)
	}
	// The row is highest first.
	for i := 1; i < table.HandSize; i++ {
		a, b := g.m.Hand[g.order[i-1]], g.m.Hand[g.order[i]]
		if a != table.Joker && (b == table.Joker || a.Rank() < b.Rank()) {
			t.Errorf("the row is not highest first: %v before %v", table.Name(a), table.Name(b))
		}
	}

	// Nothing plays until two cards are low and the hand does not foul.
	g.play()
	if !g.m.Dealt {
		t.Fatal("played with nothing set low")
	}
	g.houseWay()
	s, ok := g.setting()
	if !ok || len(g.low) != table.LowSize || !strings.HasPrefix(g.message, "SET THE HOUSE WAY") {
		t.Fatalf("the house way: %v, %q", g.low, g.message)
	}
	want := g.m.Ranker.HouseWay(g.m.Hand)
	if s.Low != want {
		t.Errorf("set %v low, the house way is %v", s.Low, want)
	}

	had := g.bank.Credits
	g.play()
	r := g.result
	if g.m.Dealt || !g.showing || g.bank.Credits != had+r.Paid {
		t.Fatalf("after playing: dealt %v, showing %v, credits from %d to %d on %d paid", g.m.Dealt, g.showing, had, g.bank.Credits, r.Paid)
	}
	if g.credits != had || g.over {
		t.Errorf("while the dealer's cards are turned: %d credits on show, over %v", g.credits, g.over)
	}
	settle(t, g)
	if !g.over || g.dealerShown != table.HandSize || g.win != r.Paid || g.credits != g.bank.Credits {
		t.Errorf("at the end: over %v, %d of the dealer's shown, win %d of %d", g.over, g.dealerShown, g.win, r.Paid)
	}

	// DEAL with nothing on the table makes the last bet again.
	g.pressDeal()
	settle(t, g)
	if !g.m.Dealt || g.m.Stake != 2500 || len(g.low) != 0 {
		t.Errorf("the next deal: dealt %v, stake %d, low %v", g.m.Dealt, g.m.Stake, g.low)
	}
}

// TestToggle checks moving cards into the low hand and out of it.
func TestToggle(t *testing.T) {
	g := testGame()
	g.m.Chip = 100
	g.place()
	g.deal()
	settle(t, g)

	g.toggle(0)
	if len(g.low) != 1 || g.low[0] != g.order[0] || g.message != chooseLow {
		t.Fatalf("after one card: %v, %q", g.low, g.message)
	}
	g.toggle(0)
	if len(g.low) != 0 {
		t.Error("the card did not come back out of the low hand")
	}
	// A third card takes the place of the first chosen.
	g.toggle(4)
	g.toggle(5)
	g.toggle(6)
	if len(g.low) != 2 || slices.Contains(g.low, g.order[4]) || !slices.Contains(g.low, g.order[6]) {
		t.Errorf("after three cards: %v", g.low)
	}
	// The two highest low fouls whenever the rest make no pair.
	g.low = g.low[:0]
	g.toggle(0)
	g.toggle(1)
	if s, ok := g.setting(); !ok {
		if !strings.Contains(g.message, "MUST BEAT") {
			t.Errorf("a foul says %q", g.message)
		}
		g.play()
		if !g.m.Dealt {
			t.Error("played a foul")
		}
	} else if !strings.Contains(g.message, s.High.String()) {
		t.Errorf("the setting says %q", g.message)
	}
}

func TestDealWithoutCredits(t *testing.T) {
	g := testGame()
	g.pressDeal()
	if g.shuffling || g.m.Dealt || !strings.Contains(g.message, "BET") {
		t.Errorf("dealt with nothing on the table: %q", g.message)
	}
	g.m.Chip = 100
	g.place()
	g.bank.Credits = 0
	g.pressDeal()
	if g.shuffling || g.m.Dealt || g.message != outOfCredits {
		t.Errorf("with no credits: dealt %v, %q", g.m.Dealt, g.message)
	}
	g.insertCoin()
	g.pressDeal()
	settle(t, g)
	if !g.m.Dealt || g.bank.Credits != bank.CoinCredits-100 {
		t.Errorf("after a coin: dealt %v, credits %d", g.m.Dealt, g.bank.Credits)
	}
}

func TestBank(t *testing.T) {
	g := testGame()
	g.bank.Pay(2500)
	g.m.Chip = 500
	g.place()
	g.deal()
	settle(t, g)
	if want := bank.StartCredits + 2000; g.bank.Credits != want || g.credits != want {
		t.Errorf("the bank has %d credits with %d on show, want %d", g.bank.Credits, g.credits, want)
	}
	g.insertCoin()
	g.reset()
	if g.bank.Credits != bank.StartCredits || !g.m.Dealt {
		t.Errorf("after a coin and a reset: %d credits, dealt %v", g.bank.Credits, g.m.Dealt)
	}
	g.houseWay()
	g.play()
	settle(t, g)
	g.place()
	g.m.Chip = 100
	g.remove()
	if g.m.Bet != 400 {
		t.Errorf("taking 100 off 500 left %d", g.m.Bet)
	}
	g.clear()
	if g.m.Bet != 0 {
		t.Errorf("a cleared table has %d on it", g.m.Bet)
	}
}

// TestSaveAndResume closes the game with a hand to set and opens it again.
func TestSaveAndResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	g := New(bank.Open(path), testSounds())
	g.setChip(2500)
	g.place()
	g.deal()
	settle(t, g)

	again := New(bank.Open(path), testSounds())
	if !again.m.Dealt || again.m.Hand != g.m.Hand || again.playerShown != table.HandSize || again.showing {
		t.Fatalf("the hand to set is not on show: %v", again.m.Hand)
	}
	if again.order != g.order || len(again.low) != 0 {
		t.Errorf("resumed with the row %v and low %v, want %v and none", again.order, again.low, g.order)
	}
	had := again.bank.Credits
	again.houseWay()
	again.play()
	settle(t, again)
	if !again.over || again.bank.Credits != had+again.result.Paid {
		t.Errorf("the hand played out: over %v, credits from %d to %d on %d paid", again.over, had, again.bank.Credits, again.result.Paid)
	}
}
