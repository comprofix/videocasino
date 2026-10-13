package slot

import (
	"path/filepath"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/games/slot/machine"
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
	g := &Game{snd: testSounds(), bank: b, m: machine.New(b, 1, 2)}
	g.credits = b.Credits
	return g
}

// stop lets time pass until the reels have all stopped.
func stop(t *testing.T, g *Game) {
	t.Helper()
	for range 600 {
		if !g.spinning {
			return
		}
		g.ticks++
		g.advanceReels()
	}
	t.Fatal("the reels never stopped")
}

// startLink starts a link feature holding n chips, each worth the bet.
func startLink(g *Game, n int) {
	l := &machine.Feature{Bet: g.m.TotalBet(), Respins: machine.LinkRespins}
	for i := range n {
		l.Held[i/machine.Rows][i%machine.Rows] = true
		l.Chips[i/machine.Rows][i%machine.Rows] = machine.Chip{Bets: 1}
	}
	g.m.Link = l
}

// TestBank checks that the game plays from the bank it is given, and that
// a coin or a reset made in the game is made to the bank.
func TestBank(t *testing.T) {
	g := testGame()
	g.bank.Pay(2500) // won somewhere else
	g.m.Lines, g.m.BetPerLine = 25, 2
	g.spin()
	if !g.spinning {
		t.Fatal("the reels are not turning after a spin")
	}
	win := g.result.Win
	if want := bank.StartCredits + 2500 - 50 + win; g.bank.Credits != want {
		t.Errorf("the bank has %d credits after a bet of 50 and a win of %d, want %d", g.bank.Credits, win, want)
	}
	// The win is not shown until the reels stop.
	if g.credits != g.bank.Credits-win || g.win != 0 {
		t.Errorf("while the reels turn: %d credits and a win of %d on show", g.credits, g.win)
	}
	stop(t, g)
	if g.credits != g.bank.Credits || g.win != win {
		t.Errorf("once the reels stop: %d credits and a win of %d on show, want %d and %d", g.credits, g.win, g.bank.Credits, win)
	}

	had := g.bank.Credits
	g.insertCoin()
	if g.bank.Credits != had+bank.CoinCredits || g.credits != g.bank.Credits {
		t.Errorf("after a coin the bank has %d credits, with %d on show", g.bank.Credits, g.credits)
	}

	// A reset leaves the bet, the pots and any free games as they are.
	g.m.FreeSpins = 2
	major, grand := g.m.Major(), g.m.Grand()
	g.reset()
	if g.bank.Credits != bank.StartCredits || g.credits != bank.StartCredits {
		t.Errorf("after a reset: %d credits, with %d on show", g.bank.Credits, g.credits)
	}
	if g.m.Lines != 25 || g.m.BetPerLine != 2 || g.m.FreeSpins != 2 || g.m.Major() != major || g.m.Grand() != grand {
		t.Errorf("a reset changed the game: %+v", g.m.Save())
	}
}

func TestSpinWithoutCredits(t *testing.T) {
	g := testGame()
	g.m.Lines, g.m.BetPerLine = 25, 2
	g.bank.Credits, g.credits = 49, 49
	g.spin()
	if g.spinning || g.bank.Credits != 49 {
		t.Errorf("spun for 50 with 49 credits")
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

	// A free game costs nothing, so it is played all the same.
	g.m.FreeSpins = 1
	g.spin()
	if !g.spinning || !g.result.Free || g.bank.Credits != g.result.Win {
		t.Errorf("a free game with no credits: spinning %v, free %v, credits %d after a win of %d",
			g.spinning, g.result.Free, g.bank.Credits, g.result.Win)
	}
	stop(t, g)

	// A coin lets the next spin be paid for.
	for g.m.FreeSpins > 0 || g.m.Link != nil {
		if g.m.Link != nil {
			g.m.Respin()
			continue
		}
		g.spin()
		stop(t, g)
	}
	g.bank.Credits = 0
	g.insertCoin()
	g.spin()
	if !g.spinning || g.bank.Credits != bank.CoinCredits-50+g.result.Win {
		t.Errorf("after a coin: spinning %v, credits %d", g.spinning, g.bank.Credits)
	}
}

// TestSaveAndResume closes the game with free games waiting and opens it
// again from the save file.
func TestSaveAndResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	g := New(bank.Open(path), testSounds())
	g.setLines(25)
	g.setBet(5)
	for range 20 {
		g.spin()
		stop(t, g)
		for g.m.Link != nil {
			g.m.Respin()
		}
	}
	g.m.FreeSpins = 3
	g.save()
	if g.m.Major() == machine.MajorStart || g.m.Grand() == machine.GrandStart {
		t.Fatal("the pots did not grow during play")
	}

	again := New(bank.Open(path), testSounds())
	want, got := g.m.Save(), again.m.Save()
	if got != want {
		t.Errorf("resumed with %+v, want %+v", got, want)
	}
	if again.bank.Credits != g.bank.Credits || again.credits != again.bank.Credits {
		t.Errorf("resumed with %d credits and %d on show, want %d", again.bank.Credits, again.credits, g.bank.Credits)
	}
	if !again.free.active || again.free.total != 3 {
		t.Errorf("the free games to play are not on show: %+v", again.free)
	}

	// They can be played out where they were left, at no cost.
	had := again.bank.Credits
	again.spin()
	if !again.spinning || !again.result.Free || again.bank.Credits != had+again.result.Win {
		t.Errorf("the free game: spinning %v, free %v, credits from %d to %d on a win of %d",
			again.spinning, again.result.Free, had, again.bank.Credits, again.result.Win)
	}
}

// TestResumeLink closes the game in the middle of a link feature and opens
// it again from the save file.
func TestResumeLink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	g := New(bank.Open(path), testSounds())
	g.setLines(25)
	g.setBet(5)
	startLink(g, 7)
	g.save()

	again := New(bank.Open(path), testSounds())
	if again.m.Link == nil || *again.m.Link != *g.m.Link {
		t.Fatalf("resumed with link feature %+v, want %+v", again.m.Link, g.m.Link)
	}
	if !again.link.show || !again.linkBusy() || again.link.total != again.m.LinkTotal() {
		t.Errorf("the link feature to finish is not on show: %+v", again.link)
	}

	// Nothing else can be spun until it is played out, and it pays into
	// the bank.
	had := again.bank.Credits
	if _, ok := again.m.Spin(); ok {
		t.Error("spun the reels in the middle of a link feature")
	}
	var end machine.Respin
	for again.m.Link != nil {
		end, _ = again.m.Respin()
	}
	if !end.Done || end.Win <= 0 || again.bank.Credits != had+end.Win {
		t.Errorf("the link feature paid %d: credits went from %d to %d", end.Win, had, again.bank.Credits)
	}
}
