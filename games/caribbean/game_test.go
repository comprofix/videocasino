package caribbean

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/caribbean/table"
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

// dealUntil deals hands, folding them, until one is dealt that want is
// true of, and leaves that one waiting.
func dealUntil(t *testing.T, g *Game, want func(table.Hand, cards.Card) bool) {
	t.Helper()
	for range 5000 {
		g.bank.Credits = 100_000
		if g.m.Ante == 0 && g.m.Last == 0 {
			g.m.Chip = 500
			g.place()
		}
		g.deal()
		settle(t, g)
		if !g.m.Dealt {
			t.Fatal("no hand was dealt")
		}
		if want(g.m.Hand, g.m.Up) {
			return
		}
		g.fold()
	}
	t.Fatal("the hand wanted was never dealt")
}

// TestShuffleAndDeal follows a hand from the bet through the shuffle to the
// cards on show.
func TestShuffleAndDeal(t *testing.T) {
	g := testGame()
	g.m.Chip = 500
	g.place()
	if g.m.Ante != 500 || g.bank.Credits != bank.StartCredits {
		t.Fatalf("%d on the ante, %d in the bank before the deal", g.m.Ante, g.bank.Credits)
	}

	g.pressDeal()
	if !g.shuffling || g.m.Dealt {
		t.Fatalf("after pressing deal: shuffling %v, dealt %v", g.shuffling, g.m.Dealt)
	}
	for n := 0; g.shuffling; n++ {
		if n > shuffleLength.Ticks()+1 {
			t.Fatal("the shuffle never ended")
		}
		g.ticks++
		g.shuffleStep(false)
	}
	if got := g.ticks - g.shuffleStart; got != shuffleLength.Ticks() {
		t.Errorf("the shuffle took %d ticks, want %d", got, shuffleLength.Ticks())
	}
	if !g.m.Dealt || !g.showing || g.bank.Credits != bank.StartCredits-500 {
		t.Fatalf("after the shuffle: dealt %v, showing %v, %d credits", g.m.Dealt, g.showing, g.bank.Credits)
	}

	// The player's five cards come one by one, then the dealer's face up.
	for up := 1; up <= table.HandSize; up++ {
		for g.playerShown < up {
			g.ticks++
			g.show(false)
		}
		if g.upShown || !g.showing {
			t.Fatalf("the dealer's card was shown with %d of the player's", up)
		}
	}
	settle(t, g)
	if !g.upShown || g.dealerShown != 0 || g.showing || !strings.HasSuffix(g.message, "RAISE OR FOLD") {
		t.Errorf("after the deal: up card shown %v, %d of the others, %q", g.upShown, g.dealerShown, g.message)
	}

	// The shuffle can be cut short, and so can the deal.
	g.fold()
	g.pressDeal()
	g.ticks++
	g.shuffleStep(true)
	if g.shuffling || !g.showing || !g.m.Dealt {
		t.Error("the shuffle was not cut short")
	}
	g.ticks++
	g.show(true)
	if g.showing || g.playerShown != table.HandSize || !g.upShown {
		t.Error("the deal was not cut short")
	}
}

// TestRaise raises a hand and follows it to the showdown.
func TestRaise(t *testing.T) {
	g := testGame()
	dealUntil(t, g, func(h table.Hand, _ cards.Card) bool { return h.Category() > 0 })
	had := g.bank.Credits

	// While the hand waits nothing can be bet.
	g.place()
	g.pressDeal()
	if g.m.Ante != 0 || g.shuffling {
		t.Error("bet or dealt with a hand waiting")
	}

	g.showHint()
	if !g.hinting || g.advice != table.Advise(g.m.Hand, g.m.Up) || !g.advice.Best {
		t.Errorf("the hint for %v is %+v", g.m.Hand, g.advice)
	}

	g.raise()
	if !g.showing || g.m.Dealt || g.hinting {
		t.Fatalf("after raising: showing %v, dealt %v, hinting %v", g.showing, g.m.Dealt, g.hinting)
	}
	r := g.result
	if r.Raise != 1000 || g.bank.Credits != had-1000+r.Paid {
		t.Errorf("the raise was %d, and the bank went from %d to %d on %d paid", r.Raise, had, g.bank.Credits, r.Paid)
	}
	// The result is not shown until the dealer's cards are up.
	if g.credits != had-1000 || g.over {
		t.Errorf("while the dealer's cards are turned: %d credits on show, over %v", g.credits, g.over)
	}
	if ticks := settle(t, g); ticks < (table.HandSize-1)*dealTicks {
		t.Errorf("the dealer's cards were turned in %d ticks", ticks)
	}
	if !g.over || g.credits != g.bank.Credits || g.win != r.Paid || g.dealerShown != table.HandSize-1 {
		t.Errorf("at the end: over %v, %d credits on show, win %d of %d", g.over, g.credits, g.win, r.Paid)
	}

	// DEAL with nothing on the table makes the last bet again.
	g.pressDeal()
	settle(t, g)
	if !g.m.Dealt || g.m.Stake != 500 || g.over {
		t.Errorf("the next deal: dealt %v, stake %d", g.m.Dealt, g.m.Stake)
	}
}

// TestFold folds a hand: the ante is lost and the dealer's other cards
// stay face down.
func TestFold(t *testing.T) {
	g := testGame()
	dealUntil(t, g, func(h table.Hand, _ cards.Card) bool { return h.Score() < table.DealerNeeds })
	had := g.bank.Credits
	g.showHint()
	if g.advice.Best || !strings.HasPrefix(g.message, "BEST TO FOLD") {
		t.Errorf("the hint for %v is %+v: %q", g.m.Hand, g.advice, g.message)
	}
	g.fold()
	if g.m.Dealt || !g.over || g.showing || g.dealerShown != 0 || !g.upShown {
		t.Errorf("after folding: dealt %v, over %v, %d of the dealer's cards up", g.m.Dealt, g.over, g.dealerShown)
	}
	if g.bank.Credits != had || g.result.Outcome != table.Folded || g.result.Paid != 0 || g.hinting {
		t.Errorf("folding changed the bank from %d to %d: %+v", had, g.bank.Credits, g.result)
	}
	// Nothing happens without a hand.
	g.raise()
	g.fold()
	if g.showing || g.bank.Credits != had {
		t.Error("raised or folded with no hand")
	}
}

// TestBank checks that the game plays from the bank it is given, and that
// a coin or a reset made in the game is made to the bank.
func TestBank(t *testing.T) {
	g := testGame()
	g.bank.Pay(2500) // won somewhere else
	g.m.Chip = 500
	g.place()
	g.deal()
	settle(t, g)
	if want := bank.StartCredits + 2500 - 500; g.bank.Credits != want || g.credits != want {
		t.Errorf("the bank has %d credits with %d on show, want %d", g.bank.Credits, g.credits, want)
	}

	// A coin and a reset act on the bank and leave the hand waiting.
	g.insertCoin()
	if g.bank.Credits != bank.StartCredits+2000+bank.CoinCredits || g.credits != g.bank.Credits {
		t.Errorf("after a coin the bank has %d credits, with %d on show", g.bank.Credits, g.credits)
	}
	g.reset()
	if g.bank.Credits != bank.StartCredits || g.credits != bank.StartCredits || !g.m.Dealt || g.playerShown != table.HandSize {
		t.Errorf("after a reset: %d credits, hand waiting %v", g.bank.Credits, g.m.Dealt)
	}

	// With too little for the raise the hand can still be folded.
	g.bank.Credits = 999
	g.raise()
	if !g.m.Dealt || g.showing || g.bank.Credits != 999 {
		t.Error("raised with too little in the bank")
	}
	g.fold()
	if g.m.Dealt {
		t.Error("could not fold")
	}

	// Chips come off again, and the table can be cleared.
	g.bank.Credits = 100_000
	g.place()
	g.m.Chip = 100
	g.remove()
	if g.m.Ante != 400 || g.over {
		t.Errorf("taking 100 off 500 left %d", g.m.Ante)
	}
	g.clear()
	if g.m.Ante != 0 {
		t.Errorf("a cleared table has %d on it", g.m.Ante)
	}
}

func TestDealWithoutCredits(t *testing.T) {
	g := testGame()
	g.pressDeal()
	if g.shuffling || g.m.Dealt || !strings.Contains(g.message, "ANTE") {
		t.Errorf("dealt with nothing on the table: %q", g.message)
	}

	// The bank must cover the ante three times over.
	g.m.Chip = 100
	g.place()
	g.bank.Credits, g.credits = 299, 299
	g.pressDeal()
	if g.shuffling || g.m.Dealt || g.bank.Credits != 299 {
		t.Error("dealt without enough for the raise")
	}

	// With none at all nothing is dealt, and the bank is left for the
	// player to fill: it is every game's, so the deal button does not
	// reset it.
	g.bank.Credits = 0
	g.pressDeal()
	if g.shuffling || g.m.Dealt || g.bank.Credits != 0 {
		t.Errorf("with no credits: dealt %v, credits %d", g.m.Dealt, g.bank.Credits)
	}
	if g.message != outOfCredits {
		t.Errorf("with no credits the game says %q", g.message)
	}

	g.insertCoin()
	g.pressDeal()
	settle(t, g)
	if !g.m.Dealt || g.bank.Credits != bank.CoinCredits-100 {
		t.Errorf("after a coin: dealt %v, credits %d", g.m.Dealt, g.bank.Credits)
	}
}

// TestSaveAndResume closes the game with a hand waiting and opens it again
// from the save file.
func TestSaveAndResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	g := New(bank.Open(path), testSounds())
	g.setChip(2500)
	g.place()
	g.deal()
	settle(t, g)

	again := New(bank.Open(path), testSounds())
	if want, got := g.m.Save(), again.m.Save(); got != want {
		t.Errorf("resumed with %+v, want %+v", got, want)
	}
	if again.bank.Credits != g.bank.Credits || again.credits != again.bank.Credits {
		t.Errorf("resumed with %d credits and %d on show, want %d", again.bank.Credits, again.credits, g.bank.Credits)
	}
	if !again.m.Dealt || again.m.Hand != g.m.Hand || again.m.Up != g.m.Up || again.playerShown != table.HandSize || !again.upShown || again.showing {
		t.Errorf("the hand to finish is not on show: %v with %v up", again.m.Hand, again.m.Up)
	}

	// It can be played out where it was left.
	had := again.bank.Credits
	again.raise()
	settle(t, again)
	if !again.over || again.bank.Credits != had-5000+again.result.Paid {
		t.Errorf("the hand played out: over %v, credits from %d to %d on %d paid", again.over, had, again.bank.Credits, again.result.Paid)
	}
}
