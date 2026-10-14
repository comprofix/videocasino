package videopoker

import (
	"path/filepath"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/videopoker/poker"
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
	testAdvice = sync.OnceValue(func() *advice {
		return &advice{
			one:  poker.NewOdds(&poker.NineSix, 1),
			most: poker.NewOdds(&poker.NineSix, poker.MaxCoins),
		}
	})
)

// testGame returns a game with fixed cards and a bank that is not saved
// anywhere. The odds are ready unless slow is set.
func testGame(slow bool) *Game {
	b := bank.New()
	g := &Game{snd: testSounds(), bank: b, autoHold: true, m: poker.New(b, 1, 2)}
	g.credits = b.Credits
	if !slow {
		g.advice.Store(testAdvice())
	}
	return g
}

// turn lets time pass until the cards have all been turned over.
func turn(t *testing.T, g *Game) {
	t.Helper()
	for range 10 * turnTicks {
		if !g.turning {
			return
		}
		g.ticks++
		g.turnCards()
	}
	t.Fatal("the cards never finished turning")
}

func TestDealAndDraw(t *testing.T) {
	g := testGame(false)
	g.m.Coins, g.m.CoinValue = 5, 10
	g.deal()
	if !g.m.Dealt || !g.turning || !g.shown {
		t.Fatalf("after the deal: dealt %v, turning %v, shown %v", g.m.Dealt, g.turning, g.shown)
	}
	if g.faceUp(0) {
		t.Error("the first card is face up as soon as it is dealt")
	}
	if g.credits != bank.StartCredits-50 {
		t.Errorf("credits on show after a bet of 50 are %d", g.credits)
	}
	turn(t, g)
	for i := range g.m.Hand {
		if !g.faceUp(i) {
			t.Errorf("card %d is still face down", i)
		}
	}

	// Auto hold is on, so the best cards are held.
	want, _, _ := g.best()
	if g.m.Held != want || g.holdDue {
		t.Errorf("held %v, want %v", g.m.Held, want)
	}
	if g.message == "" {
		t.Error("nothing is said after the deal")
	}

	held, dealt := g.m.Held, g.m.Hand
	g.draw()
	if g.m.Dealt {
		t.Fatal("the hand is still waiting after the draw")
	}
	for i := range dealt {
		if held[i] != g.faceUp(i) && g.turning {
			t.Errorf("card %d: held %v, but face up %v straight after the draw", i, held[i], g.faceUp(i))
		}
	}
	turn(t, g)
	made := poker.Evaluate(g.m.Hand)
	win := poker.NineSix.Pay(made, 5) * 10
	if g.made != made || g.win != win {
		t.Errorf("shows %v and a win of %d, want %v and %d", g.made, g.win, made, win)
	}
	if g.credits != g.bank.Credits || g.credits != bank.StartCredits-50+win {
		t.Errorf("credits on show are %d, the bank has %d", g.credits, g.bank.Credits)
	}
}

func TestAutoHoldOff(t *testing.T) {
	g := testGame(false)
	g.autoHold = false
	g.deal()
	turn(t, g)
	if g.m.Held != [poker.HandSize]bool{} {
		t.Errorf("held %v with auto hold off", g.m.Held)
	}

	// The hint marks the best cards but leaves the holding to the player.
	g.showHint()
	want, _, _ := g.best()
	if !g.hinting || g.hint != want || g.m.Held != [poker.HandSize]bool{} {
		t.Errorf("hint %v (on show %v), held %v; want hint %v and nothing held", g.hint, g.hinting, g.m.Held, want)
	}

	// Turning auto hold on in the middle of a hand holds them.
	g.toggleAuto()
	if !g.holdDue {
		t.Fatal("auto hold turned on, but no hold is due")
	}
	g.holdBest()
	if g.m.Held != want || g.hinting {
		t.Errorf("held %v (hint on show %v), want %v", g.m.Held, g.hinting, want)
	}

	// A change by the player is left alone.
	g.toggleHold(0)
	changed := g.m.Held
	if g.holdDue || changed == want {
		t.Errorf("after letting a card go: held %v, hold due %v", changed, g.holdDue)
	}
	g.draw()
	turn(t, g)
	if g.hinting {
		t.Error("the hint is still on show after the draw")
	}
}

// TestSlowOdds checks that a hand dealt before the odds are ready has its
// best cards held once they are.
func TestSlowOdds(t *testing.T) {
	g := testGame(true)
	g.deal()
	turn(t, g)
	if !g.holdDue || g.m.Held != [poker.HandSize]bool{} {
		t.Fatalf("with no odds: held %v, hold due %v", g.m.Held, g.holdDue)
	}
	g.showHint()
	if g.hinting {
		t.Error("a hint was given with no odds to go on")
	}

	g.advice.Store(testAdvice())
	g.holdBest()
	want, _, _ := g.best()
	if g.holdDue || g.m.Held != want {
		t.Errorf("held %v, want %v", g.m.Held, want)
	}
}

func TestDealWithoutCredits(t *testing.T) {
	g := testGame(false)
	g.m.Coins, g.m.CoinValue = 5, 100
	g.bank.Credits, g.credits = 499, 499
	g.deal()
	if g.m.Dealt || g.turning || g.bank.Credits != 499 {
		t.Errorf("dealt a hand of 500 for 499 credits")
	}

	// With none at all nothing is dealt, and the bank is left for the
	// player to fill: it is every game's, so the deal button does not
	// reset it.
	g.bank.Credits = 0
	g.deal()
	if g.m.Dealt || g.turning || g.bank.Credits != 0 {
		t.Errorf("with no credits: dealt %v, turning %v, credits %d", g.m.Dealt, g.turning, g.bank.Credits)
	}
	if g.message != outOfCredits {
		t.Errorf("with no credits the game says %q", g.message)
	}

	// A coin puts that right.
	g.insertCoin()
	g.deal()
	if !g.m.Dealt || g.bank.Credits != bank.CoinCredits-500 || g.credits != g.bank.Credits {
		t.Errorf("after a coin: dealt %v, credits %d, on show %d", g.m.Dealt, g.bank.Credits, g.credits)
	}
}

// TestBank checks that the game plays from the bank it is given, and that
// a coin or a reset made in the game is made to the bank.
func TestBank(t *testing.T) {
	g := testGame(false)
	g.bank.Pay(2500) // won somewhere else
	g.m.Coins, g.m.CoinValue = 2, 25
	g.deal()
	if g.bank.Credits != bank.StartCredits+2500-50 {
		t.Errorf("the bank has %d credits after a bet of 50", g.bank.Credits)
	}

	g.insertCoin()
	if g.bank.Credits != bank.StartCredits+2500-50+bank.CoinCredits || g.credits != g.bank.Credits {
		t.Errorf("after a coin the bank has %d credits, with %d on show", g.bank.Credits, g.credits)
	}

	// A reset leaves the hand that is being played.
	g.reset()
	if g.bank.Credits != bank.StartCredits || g.credits != bank.StartCredits || !g.m.Dealt {
		t.Errorf("after a reset: credits %d, on show %d, dealt %v", g.bank.Credits, g.credits, g.m.Dealt)
	}
}

// TestSaveAndResume closes the game in the middle of a hand and opens it
// again from the save file.
func TestSaveAndResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	g := New(bank.Open(path), testSounds())
	g.toggleAuto() // off, which is not how a new game starts
	g.setCoinValue(25)
	g.setCoins(5)
	g.deal()
	g.toggleHold(2)

	again := New(bank.Open(path), testSounds())
	if again.m.Save() != g.m.Save() || again.autoHold {
		t.Errorf("resumed with %+v and auto hold %v, want %+v and auto hold off", again.m.Save(), again.autoHold, g.m.Save())
	}
	if again.bank.Credits != bank.StartCredits-125 || again.credits != again.bank.Credits {
		t.Errorf("resumed with %d credits and %d on show, want %d", again.bank.Credits, again.credits, bank.StartCredits-125)
	}
	if !again.shown || !again.faceUp(0) {
		t.Error("the hand to be finished is not on show")
	}

	// The hand can be played out where it was left.
	again.draw()
	if again.m.Dealt || again.m.Hand[2] != g.m.Hand[2] {
		t.Errorf("held card %v became %v", g.m.Hand[2], again.m.Hand[2])
	}
}

// TestBestBet checks that a hand's worth follows the bet.
func TestBestBet(t *testing.T) {
	g := testGame(false)
	g.m.Hand = [poker.HandSize]cards.Card{
		cards.New(cards.King, cards.Hearts), cards.New(cards.King, cards.Spades),
		cards.New(cards.Four, cards.Clubs), cards.New(cards.Eight, cards.Diamonds),
		cards.New(cards.Two, cards.Hearts),
	}
	g.m.Coins, g.m.CoinValue = 1, 25
	_, one, _ := g.best()
	g.m.Coins, g.m.CoinValue = 3, 100
	held, more, _ := g.best()
	if held != [poker.HandSize]bool{true, true} {
		t.Errorf("held %v, want the two kings", held)
	}
	if diff := more - 12*one; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("worth %v at a bet of 300, and %v at a bet of 25", more, one)
	}
}
