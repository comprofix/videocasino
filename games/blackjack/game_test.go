package blackjack

import (
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/games/blackjack/table"
	"videocasino/ui"
)

// Only one set of sounds can be made in a program, and the odds take a
// moment to work out, so the tests share them.
var (
	testSounds = sync.OnceValue(func() *ui.Sounds {
		s := ui.NewSounds()
		s.On = false
		return s
	})
	testOdds = sync.OnceValue(table.NewOdds)
)

// testGame returns a game with fixed cards and a bank that is not saved
// anywhere. The odds are ready unless slow is set.
func testGame(slow bool) *Game {
	b := bank.New()
	g := &Game{snd: testSounds(), bank: b, m: table.New(b, 1, 2)}
	g.credits = b.Credits
	if !slow {
		g.odds.Store(testOdds())
	}
	return g
}

// shown lets time pass until the dealer's cards have all been shown.
func shown(t *testing.T, g *Game) {
	t.Helper()
	for range 40 * revealTicks {
		if !g.revealing {
			return
		}
		g.ticks++
		g.reveal()
	}
	t.Fatal("the dealer's cards never finished being shown")
}

// inPlay deals until there is a hand to play: a blackjack is settled as
// soon as it is dealt.
func inPlay(t *testing.T, g *Game) {
	t.Helper()
	for range 50 {
		g.bank.Reset()
		g.deal()
		if g.m.Dealt {
			return
		}
		shown(t, g)
	}
	t.Fatal("never dealt a hand that could be played")
}

func TestDealAndPlay(t *testing.T) {
	g := testGame(false)
	g.m.Bet, g.m.SideBet = 20, 10
	inPlay(t, g)

	// Perfect Pairs is settled at the deal, and shows straight away.
	want := bank.StartCredits - 30
	if g.m.Pair != table.NoPair {
		want += 10 + g.m.SideWin
	}
	if g.bank.Credits != want || g.credits != want || g.win != g.m.SideWin {
		t.Errorf("after the deal: bank %d, on show %d, win %d; want %d and a win of %d", g.bank.Credits, g.credits, g.win, want, g.m.SideWin)
	}
	if g.dealerShown != 1 || g.message == "" || g.revealing {
		t.Errorf("after the deal: %d of the dealer's cards on show, message %q, revealing %v", g.dealerShown, g.message, g.revealing)
	}

	// Standing settles the hand at once, but the result waits for the
	// dealer's cards to be shown.
	g.act(table.ActStand)
	if g.m.Dealt || !g.revealing {
		t.Fatalf("after standing: dealt %v, revealing %v", g.m.Dealt, g.revealing)
	}
	r := g.m.Last
	if g.credits != g.bank.Credits-r.Paid {
		t.Errorf("while the dealer plays: %d credits on show, the bank has %d and %d was paid", g.credits, g.bank.Credits, r.Paid)
	}
	shown(t, g)
	if g.dealerShown != len(g.m.Dealer) || g.credits != g.bank.Credits {
		t.Errorf("once the dealer has played: %d of %d cards on show, %d credits on show and %d in the bank",
			g.dealerShown, len(g.m.Dealer), g.credits, g.bank.Credits)
	}
	if want := g.m.SideWin + max(r.Paid-r.Staked, 0); g.win != want {
		t.Errorf("a win of %d on show, want %d", g.win, want)
	}
	if g.message == "" || g.message != g.result() {
		t.Errorf("the hand ended with %q, want %q", g.message, g.result())
	}
	if g.bank.Credits != want+r.Paid {
		t.Errorf("the bank has %d credits after %d was paid, want %d", g.bank.Credits, r.Paid, want+r.Paid)
	}
}

func TestDealWithoutCredits(t *testing.T) {
	g := testGame(false)
	g.m.Bet, g.m.SideBet = 20, 10
	g.bank.Credits, g.credits = 29, 29
	g.deal()
	if g.m.Dealt || g.revealing || g.bank.Credits != 29 {
		t.Errorf("dealt a hand of 30 for 29 credits")
	}

	// With none at all nothing is dealt, and the bank is left for the
	// player to fill: it is every game's, so the deal button does not
	// reset it.
	g.bank.Credits = 0
	g.deal()
	if g.m.Dealt || g.revealing || g.bank.Credits != 0 {
		t.Errorf("with no credits: dealt %v, credits %d", g.m.Dealt, g.bank.Credits)
	}
	if g.message != outOfCredits {
		t.Errorf("with no credits the game says %q", g.message)
	}

	// A coin puts that right.
	g.insertCoin()
	g.deal()
	if !g.m.Dealt && !g.revealing {
		t.Error("could not deal after a coin")
	}
}

// TestBank checks that a coin or a reset made in the game is made to the
// bank, and leaves a hand that is being played.
func TestBank(t *testing.T) {
	g := testGame(false)
	inPlay(t, g)
	hand := g.m.Save()

	had := g.bank.Credits
	g.insertCoin()
	if g.bank.Credits != had+bank.CoinCredits || g.credits != g.bank.Credits {
		t.Errorf("after a coin the bank has %d credits, with %d on show", g.bank.Credits, g.credits)
	}
	g.reset()
	if g.bank.Credits != bank.StartCredits || g.credits != bank.StartCredits {
		t.Errorf("after a reset: %d credits, with %d on show", g.bank.Credits, g.credits)
	}
	if !reflect.DeepEqual(g.m.Save(), hand) {
		t.Errorf("a coin and a reset changed the hand: %+v", g.m.Save())
	}

	// A double the bank cannot cover is refused, and says so.
	g.bank.Credits = 0
	g.act(table.ActDouble)
	if !g.m.Dealt || g.bank.Credits != 0 || !reflect.DeepEqual(g.m.Save(), hand) {
		t.Errorf("doubled with nothing in the bank: %+v", g.m.Save())
	}
	if g.message == "" {
		t.Error("nothing is said when a double is refused")
	}
}

func TestHint(t *testing.T) {
	// The hint is worked out from the shoe as it stands, so it does not
	// wait for the odds of a fresh shoe.
	g := testGame(true)
	inPlay(t, g)
	g.showHint()
	want, _ := table.BestPlay(g.m)
	if !g.hinting || g.hint != want.Best || g.message == "" {
		t.Errorf("hint %v (on show %v, message %q), want %v", g.hint, g.hinting, g.message, want.Best)
	}
	// The hint only marks the play: the hand is as it was.
	if !g.m.Dealt || len(g.m.Hands) != 1 || len(g.m.Hands[0].Cards) != 2 {
		t.Errorf("the hint changed the hand: %+v", g.m.Hands)
	}

	// It goes once something is done with the hand.
	g.act(table.ActStand)
	if g.hinting {
		t.Error("the hint is still on show after the hand was played")
	}
}

// TestSaveAndResume closes the game in the middle of a hand and opens it
// again from the save file.
func TestSaveAndResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	g := New(bank.Open(path), testSounds())
	g.setBet(50)
	g.setSideBet(5)
	inPlay(t, g)

	again := New(bank.Open(path), testSounds())
	if !reflect.DeepEqual(again.m.Save(), g.m.Save()) {
		t.Fatalf("resumed with %+v, want %+v", again.m.Save(), g.m.Save())
	}
	if again.bank.Credits != g.bank.Credits || again.credits != again.bank.Credits {
		t.Errorf("resumed with %d credits and %d on show, want %d", again.bank.Credits, again.credits, g.bank.Credits)
	}
	if again.dealerShown != 1 || again.message == "" {
		t.Errorf("the hand to be finished is not on show: %d of the dealer's cards, message %q", again.dealerShown, again.message)
	}

	// The hand can be played out where it was left, and pays into the bank.
	had := again.bank.Credits
	again.act(table.ActStand)
	shown(t, again)
	if again.m.Dealt || again.bank.Credits != had+again.m.Last.Paid {
		t.Errorf("played out the resumed hand: credits from %d to %d with %d paid", had, again.bank.Credits, again.m.Last.Paid)
	}

	// With the hand over, only the bets are kept.
	last := New(bank.Open(path), testSounds())
	if last.m.Dealt || last.m.Bet != 50 || last.m.SideBet != 5 {
		t.Errorf("after the hand: %+v", last.m.Save())
	}
}
