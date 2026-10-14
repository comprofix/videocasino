package holdem

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/holdem/table"
	"videocasino/ui"
)

// Only one set of sounds can be made in a program, and the odds take a
// moment to make, so the tests share them.
var (
	testSounds = sync.OnceValue(func() *ui.Sounds {
		s := ui.NewSounds()
		s.On = false
		return s
	})
	testOdds = sync.OnceValue(table.NewOdds)
)

// testGame returns a game with fixed cards and a bank that is not saved
// anywhere.
func testGame() *Game {
	b := bank.New()
	g := &Game{snd: testSounds(), bank: b, m: table.New(b, 1, 2), mark: table.Ante}
	g.credits = b.Credits
	g.odds.Store(testOdds())
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

// bet moves the marker to a bet and puts a chip on it.
func bet(g *Game, b table.Bet) {
	g.mark = b
	g.place()
}

// TestShuffleAndDeal follows a hand from the bets through the shuffle to
// the player's cards.
func TestShuffleAndDeal(t *testing.T) {
	g := testGame()
	g.m.Chip = 500
	bet(g, table.Ante)
	g.m.Chip = 100
	bet(g, table.Trips)
	if g.m.Total() != 1100 || g.bank.Credits != bank.StartCredits {
		t.Fatalf("%d on the table, %d in the bank before the deal", g.m.Total(), g.bank.Credits)
	}

	g.pressDeal()
	if !g.shuffling || g.m.Round != table.NoHand {
		t.Fatalf("after pressing deal: shuffling %v, round %v", g.shuffling, g.m.Round)
	}
	for n := 0; g.shuffling; n++ {
		if n > shuffleLength.Ticks()+1 {
			t.Fatal("the shuffle never ended")
		}
		g.ticks++
		g.shuffleStep(false)
	}
	if g.m.Round != table.Preflop || !g.showing || g.bank.Credits != bank.StartCredits-1100 {
		t.Fatalf("after the shuffle: round %v, showing %v, %d credits", g.m.Round, g.showing, g.bank.Credits)
	}
	if ticks := settle(t, g); ticks < table.HoleSize*dealTicks {
		t.Errorf("the cards were dealt in %d ticks", ticks)
	}
	if g.playerShown != table.HoleSize || g.boardShown != 0 || !strings.HasSuffix(g.message, "CHECK, OR BET 4X OR 3X") {
		t.Errorf("after the deal: %d cards shown, %d of the board, %q", g.playerShown, g.boardShown, g.message)
	}

	// While the hand waits nothing can be bet.
	bet(g, table.Ante)
	g.pressDeal()
	if g.m.Bets[table.Ante] != 0 || g.shuffling {
		t.Error("bet or dealt with a hand waiting")
	}
}

// TestCheckAndBet checks a hand to the river and bets it there.
func TestCheckAndBet(t *testing.T) {
	g := testGame()
	g.m.Chip = 500
	bet(g, table.Ante)
	g.deal()
	settle(t, g)

	g.showHint()
	if !g.hinting || len(g.advice.Choices) != 3 || !strings.HasPrefix(g.message, "BEST TO ") {
		t.Errorf("the hint before the flop: %+v, %q", g.advice, g.message)
	}
	g.choose(0) // check
	if g.m.Round != table.Flop || !g.showing || g.hinting {
		t.Fatalf("after checking: round %v, showing %v", g.m.Round, g.showing)
	}
	settle(t, g)
	if g.boardShown != table.FlopSize || !strings.HasSuffix(g.message, "CHECK, OR BET 2X") {
		t.Errorf("on the flop: %d of the board shown, %q", g.boardShown, g.message)
	}
	g.showHint()
	if len(g.advice.Choices) != 2 {
		t.Errorf("the hint on the flop: %+v", g.advice)
	}
	g.choose(0)
	settle(t, g)
	if g.m.Round != table.River || g.boardShown != table.BoardSize || !strings.HasSuffix(g.message, "BET 1X, OR FOLD") {
		t.Fatalf("on the river: round %v, %d shown, %q", g.m.Round, g.boardShown, g.message)
	}

	had := g.bank.Credits
	g.choose(1)
	r := g.result
	if !g.showing || g.m.Round != table.NoHand || r.Play != 500 || g.bank.Credits != had-500+r.Win() {
		t.Fatalf("after betting: showing %v, play %d, credits from %d to %d on %d won", g.showing, r.Play, had, g.bank.Credits, r.Win())
	}
	// The result is not shown until the dealer's cards are up.
	if g.credits != had-500 || g.over {
		t.Errorf("while the dealer's cards are turned: %d credits on show, over %v", g.credits, g.over)
	}
	if ticks := settle(t, g); ticks < table.HoleSize*dealTicks {
		t.Errorf("the dealer's cards were turned in %d ticks", ticks)
	}
	if !g.over || g.dealerShown != table.HoleSize || g.credits != g.bank.Credits || g.win != r.Win() {
		t.Errorf("at the end: over %v, %d of the dealer's shown, win %d of %d", g.over, g.dealerShown, g.win, r.Win())
	}

	// DEAL with nothing on the table makes the last bets again.
	g.pressDeal()
	settle(t, g)
	if g.m.Round != table.Preflop || g.m.Stake[table.Ante] != 500 || g.over {
		t.Errorf("the next deal: round %v, stake %v", g.m.Round, g.m.Stake)
	}
}

// TestBetEarly bets 4 antes before the flop: the board is all turned, then
// the dealer's cards.
func TestBetEarly(t *testing.T) {
	g := testGame()
	g.m.Chip = 100
	bet(g, table.Ante)
	g.deal()
	settle(t, g)
	g.choose(4)
	if g.m.Round != table.NoHand || g.result.Play != 400 {
		t.Fatalf("after betting 4: round %v, play %d", g.m.Round, g.result.Play)
	}
	settle(t, g)
	if g.boardShown != table.BoardSize || g.dealerShown != table.HoleSize || !g.over {
		t.Errorf("at the end: %d of the board and %d of the dealer's shown", g.boardShown, g.dealerShown)
	}
	// Bets that cannot be made in a round are not.
	g.pressDeal()
	settle(t, g)
	g.choose(2)
	g.choose(1)
	if g.m.Round != table.Preflop {
		t.Error("bet 2 or 1 before the flop")
	}
}

// TestFold folds on the river: the ante and the blind are lost, the
// dealer's cards stay face down, and Trips is still settled.
func TestFold(t *testing.T) {
	g := testGame()
	g.m.Chip = 100
	bet(g, table.Ante)
	bet(g, table.Trips)
	g.deal()
	settle(t, g)
	g.choose(0)
	settle(t, g)
	g.choose(0)
	settle(t, g)
	had := g.bank.Credits
	g.choose(0) // on the river, a fold
	r := g.result
	if g.m.Round != table.NoHand || !g.over || g.showing || g.dealerShown != 0 || r.Outcome != table.Folded {
		t.Errorf("after folding: round %v, over %v, %d of the dealer's shown, %v", g.m.Round, g.over, g.dealerShown, r.Outcome)
	}
	if g.bank.Credits != had+r.TripsPaid || g.win != r.TripsPaid || !strings.HasPrefix(g.message, "YOU FOLD") {
		t.Errorf("folding changed the bank from %d to %d: %+v, %q", had, g.bank.Credits, r, g.message)
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
	if want := bank.StartCredits + 2500 - 1000; g.bank.Credits != want || g.credits != want {
		t.Errorf("the bank has %d credits with %d on show, want %d", g.bank.Credits, g.credits, want)
	}

	// A coin and a reset act on the bank and leave the hand waiting.
	g.insertCoin()
	if g.bank.Credits != bank.StartCredits+1500+bank.CoinCredits || g.credits != g.bank.Credits {
		t.Errorf("after a coin the bank has %d credits, with %d on show", g.bank.Credits, g.credits)
	}
	g.reset()
	if g.bank.Credits != bank.StartCredits || g.credits != bank.StartCredits || g.m.Round != table.Preflop {
		t.Errorf("after a reset: %d credits, round %v", g.bank.Credits, g.m.Round)
	}

	// With too little for a bet of 4 the hand can still be bet 3, or
	// checked.
	g.bank.Credits = 1999
	g.choose(4)
	if g.m.Round != table.Preflop || g.bank.Credits != 1999 {
		t.Error("bet 4 antes with too little in the bank")
	}
	g.choose(3)
	if g.m.Round != table.NoHand {
		t.Error("could not bet 3 antes")
	}
	settle(t, g)

	// Chips come off again, and the table can be cleared.
	g.bank.Credits = 100_000
	bet(g, table.Trips)
	g.m.Chip = 100
	g.remove()
	if g.m.Bets[table.Trips] != 400 || g.over {
		t.Errorf("taking 100 off 500 left %d", g.m.Bets[table.Trips])
	}
	g.clear()
	if g.m.Total() != 0 {
		t.Errorf("a cleared table has %d on it", g.m.Total())
	}
}

func TestDealWithoutCredits(t *testing.T) {
	g := testGame()
	g.pressDeal()
	if g.shuffling || g.m.Round != table.NoHand {
		t.Error("dealt with nothing on the table")
	}

	// Trips alone gets no hand.
	g.m.Chip = 100
	bet(g, table.Trips)
	g.pressDeal()
	if g.m.Round != table.NoHand || !strings.Contains(g.message, "ANTE") {
		t.Errorf("dealt on Trips alone: %q", g.message)
	}

	// The bank must cover a bet of 4 antes as well.
	bet(g, table.Ante)
	g.bank.Credits, g.credits = 699, 699
	g.pressDeal()
	if g.shuffling || g.m.Round != table.NoHand || g.bank.Credits != 699 {
		t.Error("dealt without enough for a bet of 4 antes")
	}

	// With none at all nothing is dealt, and the bank is left for the
	// player to fill: it is every game's, so the deal button does not
	// reset it.
	g.bank.Credits = 0
	g.pressDeal()
	if g.m.Round != table.NoHand || g.bank.Credits != 0 || g.message != outOfCredits {
		t.Errorf("with no credits: round %v, credits %d, %q", g.m.Round, g.bank.Credits, g.message)
	}

	g.insertCoin()
	g.pressDeal()
	settle(t, g)
	if g.m.Round != table.Preflop || g.bank.Credits != bank.CoinCredits-300 {
		t.Errorf("after a coin: round %v, credits %d", g.m.Round, g.bank.Credits)
	}
}

// TestSaveAndResume closes the game on the flop and opens it again from the
// save file.
func TestSaveAndResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	g := New(bank.Open(path), testSounds())
	g.setChip(2500)
	bet(g, table.Ante)
	g.deal()
	settle(t, g)
	g.choose(0)
	settle(t, g)

	again := New(bank.Open(path), testSounds())
	if again.m.Round != table.Flop || again.m.Hole != g.m.Hole || again.m.Board != g.m.Board {
		t.Errorf("resumed at %v with %v and %v, want the flop with %v and %v", again.m.Round, again.m.Hole, again.m.Board, g.m.Hole, g.m.Board)
	}
	if again.bank.Credits != g.bank.Credits || again.playerShown != table.HoleSize || again.boardShown != table.FlopSize || again.showing {
		t.Errorf("the hand to finish is not on show: %d credits, %d and %d shown", again.bank.Credits, again.playerShown, again.boardShown)
	}

	// It can be played out where it was left.
	again.odds.Store(testOdds())
	had := again.bank.Credits
	again.choose(2)
	settle(t, again)
	if !again.over || again.bank.Credits != had-5000+again.result.Win() {
		t.Errorf("the hand played out: over %v, credits from %d to %d on %d won", again.over, had, again.bank.Credits, again.result.Win())
	}
}

func TestHandName(t *testing.T) {
	c := func(r cards.Rank, s cards.Suit) cards.Card { return cards.New(r, s) }
	tests := []struct {
		hole  [table.HoleSize]cards.Card
		board []cards.Card
		want  string
	}{
		{[2]cards.Card{c(cards.Ace, cards.Hearts), c(cards.Queen, cards.Hearts)}, nil, "ACE QUEEN SUITED"},
		{[2]cards.Card{c(cards.Two, cards.Hearts), c(cards.King, cards.Clubs)}, nil, "KING TWO"},
		{[2]cards.Card{c(cards.Nine, cards.Hearts), c(cards.Nine, cards.Clubs)}, nil, "PAIR OF NINES"},
		{[2]cards.Card{c(cards.Ace, cards.Hearts), c(cards.Queen, cards.Hearts)}, []cards.Card{c(cards.Ace, cards.Spades)}, "ACE QUEEN SUITED"},
		{[2]cards.Card{c(cards.Ace, cards.Hearts), c(cards.Queen, cards.Hearts)},
			[]cards.Card{c(cards.Ace, cards.Spades), c(cards.King, cards.Diamonds), c(cards.Seven, cards.Clubs)}, "PAIR OF ACES"},
	}
	for _, tt := range tests {
		if got := handName(tt.hole, tt.board); got != tt.want {
			t.Errorf("%v with %v is named %q, want %q", tt.hole, tt.board, got, tt.want)
		}
	}
}

// TestBetShapes checks that the pointer finds the bets by their shapes.
func TestBetShapes(t *testing.T) {
	tests := []struct {
		x, y int
		bet  table.Bet
		on   bool
	}{
		{int(tripsX), int(tripsY), table.Trips, true},
		{int(tripsX + tripsW - 2), int(tripsY), table.Trips, true},
		{int(tripsX + tripsW - 2), int(tripsY - tripsH + 4), 0, false}, // outside the diamond's corner
		{int(anteX), int(anteY), table.Ante, true},
		{int(blindX), int(blindY), table.Ante, true}, // the blind goes with the ante
		{int(playX), int(playY), 0, false},           // the play bet is made by playing
		{int(anteX + betR + 2), int(anteY), 0, false},
	}
	for _, tt := range tests {
		b, on := betAt(tt.x, tt.y)
		if on != tt.on || (on && b != tt.bet) {
			t.Errorf("(%d, %d) is on %v: %v, want %v: %v", tt.x, tt.y, b, on, tt.bet, tt.on)
		}
	}
}
