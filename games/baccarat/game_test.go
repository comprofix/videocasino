package baccarat

import (
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/baccarat/table"
	"videocasino/shuffle"
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
	g := &Game{snd: testSounds(), bank: b, m: table.New(b, 1, 2), mark: table.Player}
	g.credits = b.Credits
	return g
}

// deck reads cards written as two letters each, such as "Td 9s".
func deck(s string) []cards.Card {
	var out []cards.Card
	for _, f := range strings.Fields(s) {
		r := strings.IndexByte("23456789TJQKA", f[0])
		u := strings.IndexByte("cdhs", f[1])
		out = append(out, cards.New(cards.Two+cards.Rank(r), cards.Suit(u)))
	}
	return out
}

// settle lets time pass until the cards are all down, and returns how many
// ticks that took.
func settle(t *testing.T, g *Game) int {
	t.Helper()
	for n := range 2000 {
		if !g.revealing && !g.shuffling {
			return n
		}
		g.ticks++
		if g.shuffling {
			g.shuffleStep(false)
		} else {
			g.reveal(false)
		}
	}
	t.Fatal("the hand was never dealt out")
	return 0
}

// bet moves the marker to a bet and puts a chip on it.
func bet(g *Game, b table.Bet) {
	g.mark = b
	g.place()
}

// TestDealAndShow deals the first hand from a new shoe and follows it onto
// the table: the burnt cards first, then the hand card by card.
func TestDealAndShow(t *testing.T) {
	g := testGame()
	// A two is turned up and two cards burnt, then the player draws a third
	// card and wins with a pair, 9 to 7.
	g.m.Stack(deck("2s Ac Ad 2h Kd 2c 7h 5s")...)
	g.m.Chip = 100
	bet(g, table.Player)
	bet(g, table.PlayerPair)
	bet(g, table.Banker)
	if g.m.Total() != 300 || g.bank.Credits != bank.StartCredits {
		t.Fatalf("%d on the table, %d in the bank before the deal", g.m.Total(), g.bank.Credits)
	}

	g.pressDeal()
	if !g.revealing || g.shuffling {
		t.Fatalf("after the deal: revealing %v, shuffling %v", g.revealing, g.shuffling)
	}
	r := g.result
	if r.Burnt != 2 || !reflect.DeepEqual(r.Player, deck("2h 2c 5s")) || !reflect.DeepEqual(r.Banker, deck("Kd 7h")) {
		t.Fatalf("burnt %d, dealt %v and %v", r.Burnt, r.Player, r.Banker)
	}
	// Paid: 200 on the player, 1200 on the pair, nothing on the banker.
	if want := bank.StartCredits - 300 + 1400; g.bank.Credits != want {
		t.Errorf("the bank has %d credits, want %d", g.bank.Credits, want)
	}
	// The win is not shown until the cards are down.
	if g.credits != bank.StartCredits-300 || g.win != 0 || g.shown {
		t.Errorf("while the cards are dealt: %d credits and a win of %d on show", g.credits, g.win)
	}

	// The burnt cards come first, one at a time, and no card of the hand
	// until they have lain there a while.
	wantSteps := 1 + 2 + burnPause + 5
	if g.dealSteps() != wantSteps {
		t.Fatalf("the deal takes %d steps, want %d", g.dealSteps(), wantSteps)
	}
	for step := 1; step <= wantSteps; step++ {
		for g.shownStep < step {
			g.ticks++
			g.reveal(false)
			if !g.revealing {
				t.Fatalf("the deal ended at step %d of %d", g.shownStep, wantSteps)
			}
		}
		down, burning := g.burning()
		switch {
		case step <= 3:
			if !burning || down != step || g.handShown() != 0 {
				t.Errorf("step %d: burning %v with %d down and %d of the hand", step, burning, down, g.handShown())
			}
		case step <= 3+burnPause-1:
			if !burning || down != 3 || g.handShown() != 0 {
				t.Errorf("step %d: burning %v with %d down and %d of the hand", step, burning, down, g.handShown())
			}
		case step > 3+burnPause:
			if burning || g.handShown() != step-3-burnPause {
				t.Errorf("step %d: burning %v, %d of the hand on show", step, burning, g.handShown())
			}
		}
	}
	ticks := settle(t, g)
	if ticks > dealTicks+1 {
		t.Errorf("the result took %d more ticks to show", ticks)
	}
	if g.credits != g.bank.Credits || g.win != 1400 || !g.shown {
		t.Errorf("once the cards are down: %d credits and a win of %d on show", g.credits, g.win)
	}
	if !strings.HasPrefix(g.message, "PLAYER WINS 9 TO 7") {
		t.Errorf("the game says %q", g.message)
	}

	// The next hand is not the first of the shoe: nothing is burnt, and
	// with nothing on the table the last bets are made again.
	g.pressDeal()
	if !g.revealing || g.result.Burnt != 0 || g.result.Bet != 300 || g.burnSteps() != 0 {
		t.Errorf("the second hand: burnt %d on a bet of %d", g.result.Burnt, g.result.Bet)
	}
	// A deal can be cut short.
	g.ticks++
	g.reveal(true)
	if g.revealing || !g.shown || g.handShown() != len(g.result.Order()) {
		t.Error("the deal was not cut short")
	}
}

// TestBank checks that the game plays from the bank it is given, and that
// a coin or a reset made in the game is made to the bank.
func TestBank(t *testing.T) {
	g := testGame()
	g.bank.Pay(2500) // won somewhere else
	g.m.Chip = 500
	bet(g, table.Tie)
	g.pressDeal()
	settle(t, g)
	if want := bank.StartCredits + 2500 - 500 + g.result.Win; g.bank.Credits != want || g.credits != want {
		t.Errorf("the bank has %d credits with %d on show, want %d", g.bank.Credits, g.credits, want)
	}

	had := g.bank.Credits
	g.insertCoin()
	if g.bank.Credits != had+bank.CoinCredits || g.credits != g.bank.Credits {
		t.Errorf("after a coin the bank has %d credits, with %d on show", g.bank.Credits, g.credits)
	}

	// A reset leaves the bets and the shoe as they are.
	bet(g, table.Banker)
	left := g.m.CardsLeft()
	g.reset()
	if g.bank.Credits != bank.StartCredits || g.credits != bank.StartCredits {
		t.Errorf("after a reset: %d credits, with %d on show", g.bank.Credits, g.credits)
	}
	if g.m.Total() != 500 || g.m.LastTotal() != 500 || g.m.CardsLeft() != left {
		t.Errorf("a reset changed the game: %d on the table, %d remembered, %d cards", g.m.Total(), g.m.LastTotal(), g.m.CardsLeft())
	}

	// Chips come off again, and the table can be cleared.
	g.m.Chip = 100
	g.remove()
	if g.m.Bets[table.Banker] != 400 {
		t.Errorf("taking 100 off 500 left %d", g.m.Bets[table.Banker])
	}
	g.clear()
	if g.m.Total() != 0 {
		t.Errorf("a cleared table has %d on it", g.m.Total())
	}
}

func TestDealWithoutCredits(t *testing.T) {
	g := testGame()
	g.pressDeal()
	if g.revealing || g.bank.Credits != bank.StartCredits {
		t.Error("dealt with nothing on the table")
	}

	g.m.Chip = 100
	bet(g, table.Player)
	g.bank.Credits, g.credits = 99, 99
	left := g.m.CardsLeft()
	g.pressDeal()
	if g.revealing || g.bank.Credits != 99 || g.m.CardsLeft() != left {
		t.Errorf("dealt for 100 with 99 credits")
	}
	bet(g, table.Banker)
	if g.m.Total() != 100 {
		t.Errorf("%d on the table with 99 credits", g.m.Total())
	}

	// With none at all nothing is dealt, and the bank is left for the
	// player to fill: it is every game's, so the deal button does not
	// reset it.
	g.bank.Credits = 0
	g.pressDeal()
	if g.revealing || g.bank.Credits != 0 {
		t.Errorf("with no credits: revealing %v, credits %d", g.revealing, g.bank.Credits)
	}
	if g.message != outOfCredits {
		t.Errorf("with no credits the game says %q", g.message)
	}

	g.insertCoin()
	g.pressDeal()
	if !g.revealing || g.bank.Credits != bank.CoinCredits-100+g.result.Win {
		t.Errorf("after a coin: revealing %v, credits %d", g.revealing, g.bank.Credits)
	}
}

// TestCutCardAndShuffle deals a shoe down to its cut card, and checks that
// the card is dealt onto the table, that one hand more is dealt, and that
// the shuffle is then shown before the next.
func TestCutCardAndShuffle(t *testing.T) {
	g := testGame()
	g.m.Chip = 10
	play := func() {
		g.bank.Credits = 1000
		if g.m.Total() == 0 && g.m.LastTotal() == 0 {
			bet(g, table.Player)
		}
		g.pressDeal()
	}

	// Deal until the cut card comes out.
	for !g.m.CutOut() {
		play()
		if g.shuffling {
			t.Fatal("the shoe was shuffled before the cut card came out")
		}
		if !g.m.CutOut() {
			settle(t, g)
			if g.cutShown {
				t.Fatal("the cut card is on the table before it has come out")
			}
		}
	}
	// It is dealt onto the table in its turn: after the cards before it,
	// and not sooner.
	after := g.result.CutAfter
	if after < 1 || g.cutShown {
		t.Fatalf("the cut card came out after %d cards, shown already: %v", after, g.cutShown)
	}
	for g.revealing {
		g.ticks++
		g.reveal(false)
		if g.cutShown != (g.handShown() >= after) {
			t.Fatalf("with %d cards down the cut card is shown: %v; it came after %d", g.handShown(), g.cutShown, after)
		}
	}
	if g.m.Finished {
		t.Fatal("the shoe is finished with a hand still to deal")
	}

	// One hand more, with no shuffle before it.
	play()
	if g.shuffling || !g.revealing || !g.m.Finished {
		t.Fatalf("the hand after the cut card: shuffling %v, revealing %v, finished %v", g.shuffling, g.revealing, g.m.Finished)
	}
	settle(t, g)
	if !g.cutShown {
		t.Error("the cut card left the table before the shuffle")
	}

	// Then the shuffle is shown, and the hand dealt after it from a new
	// shoe, with cards burnt.
	left := g.m.CardsLeft()
	play()
	if !g.shuffling || g.revealing || g.m.CardsLeft() != left {
		t.Fatalf("after the last hand: shuffling %v, revealing %v, %d cards of %d", g.shuffling, g.revealing, g.m.CardsLeft(), left)
	}
	n := 0
	for g.shuffling {
		g.ticks++
		g.shuffleStep(false)
		if n++; n > shuffle.Full.Ticks()+1 {
			t.Fatal("the shuffle never ended")
		}
	}
	if n != shuffle.Full.Ticks() {
		t.Errorf("the shuffle took %d ticks, want %d", n, shuffle.Full.Ticks())
	}
	if !g.revealing || g.result.Burnt == 0 || g.cutShown || g.m.Finished || g.m.CardsLeft() < table.ShoeSize-17 {
		t.Errorf("after the shuffle: revealing %v, burnt %d, cut card shown %v, %d cards", g.revealing, g.result.Burnt, g.cutShown, g.m.CardsLeft())
	}
	settle(t, g)

	// With no bet the bank can cover there is no shuffle to show.
	g.m.Finished = true
	g.bank.Credits = 0
	g.pressDeal()
	if g.shuffling || g.revealing {
		t.Error("the shoe was shuffled for a hand that could not be dealt")
	}
	// The shuffle can be cut short.
	g.bank.Credits = 1000
	g.pressDeal()
	g.ticks++
	g.shuffleStep(true)
	if g.shuffling || !g.revealing {
		t.Error("the shuffle was not cut short")
	}
}

// TestSaveAndResume closes the game part of the way through a shoe and
// opens it again from the save file.
func TestSaveAndResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	g := New(bank.Open(path), testSounds())
	g.setChip(100)
	for range 6 {
		g.m.Clear()
		bet(g, table.Banker)
		g.pressDeal()
		settle(t, g)
	}
	g.setChip(500)
	bet(g, table.Tie)

	again := New(bank.Open(path), testSounds())
	if want, got := g.m.Save(), again.m.Save(); !reflect.DeepEqual(got, want) {
		t.Errorf("resumed with %+v, want %+v", got, want)
	}
	if again.bank.Credits != g.bank.Credits || again.credits != again.bank.Credits {
		t.Errorf("resumed with %d credits and %d on show, want %d", again.bank.Credits, again.credits, g.bank.Credits)
	}
	if again.m.Total() != 500 || again.m.LastTotal() != 100 || again.m.Chip != 500 || len(again.m.History) != 6 {
		t.Errorf("resumed with %d on the table, %d remembered, a chip of %d and %d hands", again.m.Total(), again.m.LastTotal(), again.m.Chip, len(again.m.History))
	}
	if again.m.CardsLeft() != g.m.CardsLeft() || again.m.ToCut != g.m.ToCut || again.m.Fresh || again.cutShown {
		t.Errorf("resumed with %d cards and the cut card in %d, want %d and %d", again.m.CardsLeft(), again.m.ToCut, g.m.CardsLeft(), g.m.ToCut)
	}

	// The hand left on the table can be dealt, with nothing burnt.
	had := again.bank.Credits
	again.pressDeal()
	if !again.revealing || again.result.Burnt != 0 || again.bank.Credits != had-500+again.result.Win {
		t.Errorf("the deal: burnt %d, credits from %d to %d on a win of %d", again.result.Burnt, had, again.bank.Credits, again.result.Win)
	}
}
