package threecard

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/threecard/table"
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
	g := &Game{snd: testSounds(), bank: b, m: table.New(b, 1, 2), mark: table.Ante}
	g.credits = b.Credits
	return g
}

// settle lets time pass until the cards are all shown, and returns how
// many ticks that took.
func settle(t *testing.T, g *Game) int {
	t.Helper()
	for n := range 1000 {
		if !g.showing {
			return n
		}
		g.ticks++
		g.show(false)
	}
	t.Fatal("the cards were never all shown")
	return 0
}

// bet moves the marker to a bet and puts a chip on it.
func bet(g *Game, b table.Bet) {
	g.mark = b
	g.place()
}

// dealUntil deals hands, folding them, until one is dealt that want is
// true of, and leaves that one waiting.
func dealUntil(t *testing.T, g *Game, want func(table.Hand) bool) {
	t.Helper()
	for range 5000 {
		g.bank.Credits = 10_000
		if g.m.Total() == 0 && g.m.LastTotal() == 0 {
			g.m.Chip = 100
			bet(g, table.Ante)
			bet(g, table.PairPlus)
		}
		g.deal()
		settle(t, g)
		if !g.m.Dealt {
			t.Fatal("no hand was dealt")
		}
		if want(g.m.Hand) {
			return
		}
		g.fold()
	}
	t.Fatal("the hand wanted was never dealt")
}

// TestDealAndPlay follows a hand from the bets to the showdown.
func TestDealAndPlay(t *testing.T) {
	g := testGame()
	g.m.Chip = 100
	bet(g, table.Ante)
	bet(g, table.PairPlus)
	if g.m.Total() != 200 || g.bank.Credits != bank.StartCredits {
		t.Fatalf("%d on the table, %d in the bank before the deal", g.m.Total(), g.bank.Credits)
	}

	// A pair, so that Pair Plus pays at the deal.
	dealUntil(t, g, func(h table.Hand) bool { return h.Category() == table.Pair })
	had := g.bank.Credits
	if g.showing || g.playerShown != table.HandSize || g.dealerShown != 0 || g.over {
		t.Fatalf("after the deal: showing %v, %d of the player's and %d of the dealer's cards up", g.showing, g.playerShown, g.dealerShown)
	}
	if g.m.PairPaid != 200 || g.win != 200 || g.credits != had || !strings.Contains(g.message, "PAIR PLUS PAYS $2 ") {
		t.Errorf("Pair Plus paid %d, win %d on show, %d credits on show of %d: %q", g.m.PairPaid, g.win, g.credits, had, g.message)
	}
	// While the hand waits nothing can be bet.
	bet(g, table.Ante)
	if g.m.Total() != 0 || !g.m.Dealt {
		t.Error("a chip was put down with a hand waiting")
	}

	// The hint is the advice of the rules.
	g.showHint()
	if !g.hinting || g.advice != table.Advise(g.m.Hand) || !g.advice.Best {
		t.Errorf("the hint for a pair is %+v", g.advice)
	}

	g.play()
	if !g.showing || g.m.Dealt || g.hinting {
		t.Fatalf("after playing: showing %v, dealt %v, hinting %v", g.showing, g.m.Dealt, g.hinting)
	}
	r := g.result
	if r.Play != 100 || g.bank.Credits != had-100+r.Paid {
		t.Errorf("the play bet was %d, and the bank went from %d to %d on %d paid", r.Play, had, g.bank.Credits, r.Paid)
	}
	// The result is not shown until the dealer's cards are up.
	if g.credits != had-100 || g.over {
		t.Errorf("while the dealer's cards are turned: %d credits on show, over %v", g.credits, g.over)
	}
	for up := 1; up <= table.HandSize; up++ {
		for g.dealerShown < up {
			g.ticks++
			g.show(false)
		}
		if g.over || !g.showing {
			t.Fatalf("the hand ended with %d of the dealer's cards up", g.dealerShown)
		}
	}
	if ticks := settle(t, g); ticks > dealTicks+1 {
		t.Errorf("the result took %d more ticks to show", ticks)
	}
	if !g.over || g.credits != g.bank.Credits || g.win != r.Win() || g.playerShown != table.HandSize {
		t.Errorf("at the end: over %v, %d credits on show, win %d of %d", g.over, g.credits, g.win, r.Win())
	}

	// DEAL with nothing on the table makes the last bets again.
	g.bank.Credits = 10_000
	g.deal()
	if !g.showing || !g.m.Dealt || g.m.Stake != [table.Bets]int{table.PairPlus: 100, table.Ante: 100} || g.over || g.playerShown != 0 {
		t.Errorf("the next deal: showing %v, stake %v", g.showing, g.m.Stake)
	}
	// The deal can be cut short.
	g.ticks++
	g.show(true)
	if g.showing || g.playerShown != table.HandSize {
		t.Error("the deal was not cut short")
	}
}

// TestFold folds a hand: the ante is lost and the dealer's cards stay face
// down.
func TestFold(t *testing.T) {
	g := testGame()
	dealUntil(t, g, func(h table.Hand) bool { return h.Category() == table.HighCard })
	had := g.bank.Credits
	g.showHint()
	if g.advice != table.Advise(g.m.Hand) || g.advice.Best != (g.advice.Play > -1) {
		t.Errorf("the hint is %+v", g.advice)
	}
	g.fold()
	if g.m.Dealt || !g.over || g.showing || g.dealerShown != 0 || g.playerShown != table.HandSize {
		t.Errorf("after folding: dealt %v, over %v, %d of the dealer's cards up", g.m.Dealt, g.over, g.dealerShown)
	}
	if g.bank.Credits != had || g.result.Outcome != table.Folded || g.result.Win() != 0 || g.hinting {
		t.Errorf("folding changed the bank from %d to %d: %+v", had, g.bank.Credits, g.result)
	}
	// Nothing happens without a hand.
	g.play()
	g.fold()
	if g.showing || g.bank.Credits != had {
		t.Error("played or folded with no hand")
	}
}

// TestBank checks that the game plays from the bank it is given, and that
// a coin or a reset made in the game is made to the bank.
func TestBank(t *testing.T) {
	g := testGame()
	g.bank.Pay(2500) // won somewhere else
	g.m.Chip = 500
	bet(g, table.Ante)
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

	// With too little for the play bet the hand can still be folded.
	g.bank.Credits = 499
	g.play()
	if !g.m.Dealt || g.showing || g.bank.Credits != 499 {
		t.Error("played with too little in the bank for the bet")
	}
	g.fold()
	if g.m.Dealt {
		t.Error("could not fold")
	}

	// Chips come off again, and the table can be cleared.
	g.bank.Credits = 10_000
	bet(g, table.PairPlus)
	g.m.Chip = 100
	g.remove()
	if g.m.Bets[table.PairPlus] != 400 || g.over {
		t.Errorf("taking 100 off 500 left %d", g.m.Bets[table.PairPlus])
	}
	g.clear()
	if g.m.Total() != 0 {
		t.Errorf("a cleared table has %d on it", g.m.Total())
	}
}

func TestDealWithoutCredits(t *testing.T) {
	g := testGame()
	g.deal()
	if g.showing || g.m.Dealt || g.bank.Credits != bank.StartCredits {
		t.Error("dealt with nothing on the table")
	}

	// Pair Plus alone gets no hand.
	g.m.Chip = 100
	bet(g, table.PairPlus)
	g.deal()
	if g.m.Dealt || !strings.Contains(g.message, "ANTE") {
		t.Errorf("dealt on Pair Plus alone: %q", g.message)
	}

	// The bank must cover the ante twice over.
	bet(g, table.Ante)
	g.bank.Credits, g.credits = 299, 299
	g.deal()
	if g.m.Dealt || g.bank.Credits != 299 {
		t.Error("dealt without enough for the play bet")
	}

	// With none at all nothing is dealt, and the bank is left for the
	// player to fill: it is every game's, so the deal button does not
	// reset it.
	g.bank.Credits = 0
	g.deal()
	if g.m.Dealt || g.bank.Credits != 0 {
		t.Errorf("with no credits: dealt %v, credits %d", g.m.Dealt, g.bank.Credits)
	}
	if g.message != outOfCredits {
		t.Errorf("with no credits the game says %q", g.message)
	}

	g.insertCoin()
	g.deal()
	if !g.m.Dealt || g.bank.Credits != bank.CoinCredits-200+g.m.PairPaid {
		t.Errorf("after a coin: dealt %v, credits %d", g.m.Dealt, g.bank.Credits)
	}
}

// TestSaveAndResume closes the game with a hand waiting and opens it again
// from the save file.
func TestSaveAndResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	g := New(bank.Open(path), testSounds())
	g.setChip(100)
	bet(g, table.Ante)
	bet(g, table.PairPlus)
	g.deal()
	settle(t, g)

	again := New(bank.Open(path), testSounds())
	if want, got := g.m.Save(), again.m.Save(); got != want {
		t.Errorf("resumed with %+v, want %+v", got, want)
	}
	if again.bank.Credits != g.bank.Credits || again.credits != again.bank.Credits {
		t.Errorf("resumed with %d credits and %d on show, want %d", again.bank.Credits, again.credits, g.bank.Credits)
	}
	if !again.m.Dealt || again.m.Hand != g.m.Hand || again.playerShown != table.HandSize || again.dealerShown != 0 || again.showing {
		t.Errorf("the hand to finish is not on show: %v, %d cards up", again.m.Hand, again.playerShown)
	}
	// Nothing of the dealer's hand is in the save: it has not been dealt.
	var kept struct{ Dealer []cards.Card }
	if again.bank.Load(saveName, &kept); len(kept.Dealer) != 0 {
		t.Errorf("the save holds dealer's cards %v", kept.Dealer)
	}

	// It can be played out where it was left.
	had := again.bank.Credits
	again.play()
	settle(t, again)
	if !again.over || again.bank.Credits != had-100+again.result.Paid {
		t.Errorf("the hand played out: over %v, credits from %d to %d on %d paid", again.over, had, again.bank.Credits, again.result.Paid)
	}
}
