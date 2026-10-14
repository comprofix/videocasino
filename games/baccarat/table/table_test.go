package table

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
)

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

// dealer returns a draw that deals the given cards in order, and a count of
// how many it has dealt.
func dealer(t *testing.T, s string) (draw func() cards.Card, dealt *int) {
	next, n := deck(s), 0
	return func() cards.Card {
		if n >= len(next) {
			t.Fatalf("a card was drawn after %q had all been dealt", s)
		}
		n++
		return next[n-1]
	}, &n
}

func TestValues(t *testing.T) {
	for _, c := range []struct {
		hand        string
		total, burn int
	}{
		{"As", 1, 1}, {"2c", 2, 2}, {"9h", 9, 9}, {"Td", 0, 10}, {"Jc", 0, 10}, {"Qs", 0, 10}, {"Kh", 0, 10},
		{"7s 8d", 5, 7}, {"Kc Qd", 0, 10}, {"9s 9d 9h", 7, 9}, {"As 2s 3s", 6, 1}, {"5c 5d", 0, 5},
	} {
		hand := deck(c.hand)
		if got := Total(hand); got != c.total {
			t.Errorf("%s counts %d, want %d", c.hand, got, c.total)
		}
		if got := BurnCount(hand[0]); got != c.burn {
			t.Errorf("%d cards are burnt for %v, want %d", got, hand[0], c.burn)
		}
	}
	if !IsPair(deck("Ks Kd 3c")) || IsPair(deck("Ks Qs")) || IsPair(deck("Ts Jd")) || IsPair(deck("4c")) {
		t.Error("a pair is not two cards of the same rank")
	}
}

// TestThirdCard checks the rules for a third card against the table of them
// as it is printed at a baccarat table.
func TestThirdCard(t *testing.T) {
	for total := range 10 {
		if got, want := PlayerDraws(total), total <= 5; got != want {
			t.Errorf("the player draws on %d: %v", total, got)
		}
		// With no third card for the player the banker plays the same way.
		if got, want := BankerDraws(total, -1), total <= 5; got != want {
			t.Errorf("the banker draws on %d when the player stands: %v", total, got)
		}
	}
	// For each total of the banker's, the player's third cards it draws on.
	draws := [10]string{
		0: "0123456789",
		1: "0123456789",
		2: "0123456789",
		3: "012345679",
		4: "234567",
		5: "4567",
		6: "67",
		7: "",
		8: "",
		9: "",
	}
	for total, on := range draws {
		for third := range 10 {
			want := strings.ContainsRune(on, rune('0'+third))
			if got := BankerDraws(total, third); got != want {
				t.Errorf("the banker draws on %d against a third card of %d: %v", total, third, got)
			}
		}
	}
}

func TestPlay(t *testing.T) {
	for _, c := range []struct {
		name           string
		cards          string // in the order dealt
		player, banker string
		outcome        Outcome
	}{
		{"a natural nine beats a natural eight", "4s 3d 5c 5h", "4s 5c", "3d 5h", PlayerWins},
		{"a natural stops the other hand drawing", "2s 4d 2c 4h", "2s 2c", "4d 4h", BankerWins},
		{"both stand on six and seven", "Ks 9d 6c 8h", "Ks 6c", "9d 8h", BankerWins},
		{"the player stands and the banker draws", "Ks 2d 7c 3h 4s", "Ks 7c", "2d 3h 4s", BankerWins},
		{"the player draws and the banker stands on seven", "2s Kd 3c 7h 9s", "2s 3c 9s", "Kd 7h", BankerWins},
		{"the banker's three stands against an eight", "Ks Kd 2c 3h 8s", "Ks 2c 8s", "Kd 3h", BankerWins},
		{"the banker's three draws against a nine", "Ks Kd 2c 3h 9s 5d", "Ks 2c 9s", "Kd 3h 5d", BankerWins},
		{"the banker's six draws against a seven", "Ks Kd 2c 6h 7s 3d", "Ks 2c 7s", "Kd 6h 3d", Tied},
		{"the banker's six stands against a five", "Ks Kd 2c 6h 5s", "Ks 2c 5s", "Kd 6h", PlayerWins},
		{"both draw", "As Kd 2c Qh 3s 4d", "As 2c 3s", "Kd Qh 4d", PlayerWins},
	} {
		draw, dealt := dealer(t, c.cards)
		player, banker := Play(draw)
		if !reflect.DeepEqual(player, deck(c.player)) || !reflect.DeepEqual(banker, deck(c.banker)) {
			t.Errorf("%s: dealt %v and %v", c.name, player, banker)
		}
		if *dealt != len(deck(c.cards)) {
			t.Errorf("%s: %d cards were dealt", c.name, *dealt)
		}
		if got := OutcomeOf(player, banker); got != c.outcome {
			t.Errorf("%s: outcome %d, want %d", c.name, got, c.outcome)
		}
		// The order they were dealt in is the order they came.
		r := Result{Player: player, Banker: banker}
		if !reflect.DeepEqual(r.Order(), deck(c.cards)) {
			t.Errorf("%s: put in order as %v", c.name, r.Order())
		}
	}
}

func TestPaid(t *testing.T) {
	bets := [Bets]int{PlayerPair: 10, Player: 100, Tie: 20, Banker: 50, BankerPair: 30}
	for _, c := range []struct {
		name           string
		player, banker string
		want           [Bets]int
	}{
		{"the player wins", "4s 5c", "3d 5h", [Bets]int{Player: 200}},
		{"the banker wins", "Ks 6c", "9d 8h", [Bets]int{Banker: 100}},
		{"the banker wins with a six", "Ks 5c", "Kd 6h", [Bets]int{Banker: 75}},
		{"the banker wins with a six in three cards", "Ks 2c 3s", "Kd 2h 4d", [Bets]int{Banker: 75}},
		{"the player wins with a six", "Ks 6c", "Kd 5h Ts", [Bets]int{Player: 200}},
		{"a tie", "Ks 7c", "9d 8h", [Bets]int{Player: 100, Tie: 180, Banker: 50}},
		{"a tie on six pays the banker's bet back whole", "Ks 6c", "Td 6h", [Bets]int{Player: 100, Tie: 180, Banker: 50}},
		{"a pair for the player, who loses", "3s 3c", "9d Kh", [Bets]int{PlayerPair: 120, Banker: 100}},
		{"a pair for each in a tie", "4s 4c", "9d 9h", [Bets]int{PlayerPair: 120, Player: 100, Tie: 180, Banker: 50, BankerPair: 360}},
		{"a third card makes no pair", "2s 3c 2d", "Kd 8h", [Bets]int{Banker: 100}},
	} {
		if got := Paid(bets, deck(c.player), deck(c.banker)); got != c.want {
			t.Errorf("%s: paid %v, want %v", c.name, got, c.want)
		}
	}
}

// TestOdds checks the odds worked out against the figures published for
// eight decks, and against one another.
func TestOdds(t *testing.T) {
	o := NewOdds()
	near := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 0.00002 {
			t.Errorf("%s is %.6f, want %.6f", name, got, want)
		}
	}
	near("the chances together", o.PlayerWins+o.BankerWins+o.Tied, 1)
	near("the chance the player wins", o.PlayerWins, 0.446247)
	near("the chance the banker wins", o.BankerWins, 0.458597)
	near("the chance of a tie", o.Tied, 0.095156)
	near("the chance the banker wins with a six", o.BankerSix, 0.053864)
	near("the chance of a pair", o.Pair, 31.0/415)

	near("the return on the player", o.Return(Player), 0.987649)
	near("the return on the banker", o.Return(Banker), 0.985419)
	near("the return on a tie", o.Return(Tie), 0.856404)
	near("the return on the player's pair", o.Return(PlayerPair), 372.0/415)
	near("the return on the banker's pair", o.Return(BankerPair), 372.0/415)
	for b := Bet(0); b < Bets; b++ {
		t.Logf("%-12s returns %.4f%%", b, o.Return(b)*100)
	}
}

// TestPlayReturn deals a great many hands, each from a full shoe, and
// checks that what the bets pay back is what the odds say.
func TestPlayReturn(t *testing.T) {
	const hands = 400_000
	o := NewOdds()
	m := New(bank.New(), 11, 12)
	var staked, paid [Bets]float64
	for range hands {
		m.shuffle()
		m.Bank.Credits = 1000
		for b := Bet(0); b < Bets; b++ {
			m.Bets[b] = 100
		}
		r, ok := m.Deal()
		if !ok {
			t.Fatal("could not deal")
		}
		for b := range r.Paid {
			staked[b] += 100
			paid[b] += float64(r.Paid[b])
		}
	}
	// The pairs and the tie pay seldom and a lot, so they wander further.
	slack := [Bets]float64{PlayerPair: 0.02, Player: 0.006, Tie: 0.02, Banker: 0.006, BankerPair: 0.02}
	for b := Bet(0); b < Bets; b++ {
		got, want := paid[b]/staked[b], o.Return(b)
		if math.Abs(got-want) > slack[b] {
			t.Errorf("%v paid back %.4f over %d hands, want %.4f", b, got, hands, want)
		}
	}
}

func TestBets(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 100
	if !m.Place(Player) || !m.Place(Player) || !m.Place(Tie) {
		t.Fatal("could not put three chips down")
	}
	if m.Bets[Player] != 200 || m.Total() != 300 || b.Credits != bank.StartCredits {
		t.Errorf("%d on the player, %d in all, %d in the bank", m.Bets[Player], m.Total(), b.Credits)
	}
	m.Chip = 500
	if !m.Remove(Player) || m.Bets[Player] != 0 || m.Remove(Player) {
		t.Errorf("taking 500 off a bet of 200 left %d", m.Bets[Player])
	}
	m.Chip = 100_000
	b.Credits = 10 * MaxBet
	for range MaxBet / 100_000 {
		if !m.Place(Banker) {
			t.Fatal("could not bet up to the limit")
		}
	}
	if m.Place(Banker) || m.Bets[Banker] != MaxBet {
		t.Errorf("a bet holds %d", m.Bets[Banker])
	}
	b.Credits = m.Total() + 999
	if m.Place(Player) {
		t.Error("bet more than the bank has")
	}
	m.Clear()
	if m.Total() != 0 {
		t.Errorf("a cleared table has %d on it", m.Total())
	}
	if _, ok := m.Deal(); ok {
		t.Error("dealt with nothing bet")
	}
	if m.Rebet() {
		t.Error("bet again before anything had been bet")
	}
}

// TestDeal deals a hand whose cards are known, the first from a new shoe.
func TestDeal(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	// A three is turned up and three cards burnt, then a tie on six.
	m.Stack(deck("3s Ac Ad Ah Ks Td 6c 6h")...)
	m.Chip = 100
	m.Place(Player)
	m.Place(Banker)
	m.Place(Tie)
	m.Place(BankerPair)
	r, ok := m.Deal()
	if !ok || m.Stacked() != 0 {
		t.Fatalf("dealt %v with %d cards still stacked", ok, m.Stacked())
	}
	if r.Burn != deck("3s")[0] || r.Burnt != 3 {
		t.Errorf("turned up %v and burnt %d", r.Burn, r.Burnt)
	}
	if !reflect.DeepEqual(r.Player, deck("Ks 6c")) || !reflect.DeepEqual(r.Banker, deck("Td 6h")) || r.Outcome != Tied {
		t.Errorf("dealt %v and %v, outcome %d", r.Player, r.Banker, r.Outcome)
	}
	want := [Bets]int{Player: 100, Tie: 900, Banker: 100}
	if r.Paid != want || r.Bet != 400 || r.Win != 1100 || b.Credits != bank.StartCredits-400+1100 {
		t.Errorf("paid %v on a bet of %d: won %d, bank %d", r.Paid, r.Bet, r.Win, b.Credits)
	}
	if m.CardsLeft() != ShoeSize-8 || m.ToCut != ShoeSize-CutBehind-8 || m.Fresh || m.Finished || r.CutAfter != -1 {
		t.Errorf("after the hand: %d cards left, cut card in %d, fresh %v, finished %v", m.CardsLeft(), m.ToCut, m.Fresh, m.Finished)
	}
	if m.Total() != 0 || m.LastTotal() != 400 || !reflect.DeepEqual(m.History, []Outcome{Tied}) {
		t.Errorf("after the hand: %d on the table, %d remembered, history %v", m.Total(), m.LastTotal(), m.History)
	}

	// The next hand is not the first of the shoe: nothing is burnt.
	if !m.Rebet() {
		t.Fatal("could not bet the same again")
	}
	left := m.CardsLeft()
	r, _ = m.Deal()
	if r.Burnt != 0 || m.CardsLeft() != left-len(r.Order()) {
		t.Errorf("the second hand burnt %d and took %d cards for a hand of %d", r.Burnt, left-m.CardsLeft(), len(r.Order()))
	}

	// With too little in the bank nothing is dealt or taken.
	m.Rebet()
	b.Credits = 399
	left = m.CardsLeft()
	if _, ok := m.Deal(); ok || b.Credits != 399 || m.CardsLeft() != left {
		t.Errorf("dealt for 400 with 399: bank %d", b.Credits)
	}
}

// TestShoe deals shoe after shoe and checks that each is burnt, dealt down
// to the cut card and one hand more, and then shuffled.
func TestShoe(t *testing.T) {
	b := bank.New()
	m := New(b, 5, 6)
	m.Chip = 10
	shoes, lastHands := 0, 0
	cutSeen := false // the cut card has come out of this shoe
	for hand := range 4000 {
		b.Credits = 1000
		m.Place(Player)
		wasFinished, wasOut, left, toCut := m.Finished, m.CutOut(), m.CardsLeft(), m.ToCut
		wasFresh := m.Fresh
		r, ok := m.Deal()
		if !ok {
			t.Fatal("could not deal")
		}
		n := len(r.Order())
		if n < 4 || n > 6 {
			t.Fatalf("a hand of %d cards", n)
		}

		first := wasFresh || wasFinished
		if first {
			// A new shoe: a card turned up, cards burnt for it, then the hand.
			shoes++
			cutSeen = false
			if r.Burnt != BurnCount(r.Burn) || m.CardsLeft() != ShoeSize-1-r.Burnt-n {
				t.Fatalf("hand %d: burnt %d for %v and left %d cards after a hand of %d", hand, r.Burnt, r.Burn, m.CardsLeft(), n)
			}
			if len(m.History) != 1 {
				t.Fatalf("hand %d: a new shoe remembers %d hands", hand, len(m.History))
			}
		} else if r.Burnt != 0 || m.CardsLeft() != left-n {
			t.Fatalf("hand %d: burnt %d and went from %d cards to %d on a hand of %d", hand, r.Burnt, left, m.CardsLeft(), n)
		}
		if m.CardsLeft()-m.ToCut != CutBehind {
			t.Fatalf("hand %d: %d cards left with the cut card in %d", hand, m.CardsLeft(), m.ToCut)
		}

		switch {
		case first:
			if m.Finished || r.CutAfter != -1 {
				t.Fatalf("hand %d: the cut card came out of a new shoe", hand)
			}
		case wasOut:
			// The cut card was out: this was the one hand more.
			lastHands++
			if !cutSeen || !m.Finished || r.CutAfter != -1 {
				t.Fatalf("hand %d: after the cut card: seen %v, finished %v, cut after %d", hand, cutSeen, m.Finished, r.CutAfter)
			}
		case m.CutOut():
			// It came out during this hand, after as many cards as were
			// still to come before it.
			cutSeen = true
			if m.Finished || r.CutAfter != toCut || r.CutAfter < 1 || r.CutAfter > n {
				t.Fatalf("hand %d: the cut card came out after %d of %d cards, with %d to come before it", hand, r.CutAfter, n, toCut)
			}
		default:
			if m.Finished || r.CutAfter != -1 {
				t.Fatalf("hand %d: finished %v, cut after %d", hand, m.Finished, r.CutAfter)
			}
		}
		if m.CardsLeft() < 3 {
			t.Fatalf("hand %d: the shoe was dealt down to %d cards", hand, m.CardsLeft())
		}
		for c, k := range m.Left {
			if k < 0 || k > Decks {
				t.Fatalf("hand %d: %d of %v left in the shoe", hand, k, cards.Card(c))
			}
		}
		if len(m.History) > HistoryLen {
			t.Fatalf("hand %d: %d hands are remembered", hand, len(m.History))
		}
	}
	if shoes < 40 || lastHands < shoes-1 {
		t.Errorf("%d shoes were dealt, %d of them to their last hand", shoes, lastHands)
	}
}

func TestSaveAndRestore(t *testing.T) {
	b := bank.New()
	m := New(b, 3, 4)
	m.Chip = 100
	for range 30 {
		m.Place(Player)
		m.Place(BankerPair)
		m.Deal()
	}
	m.Chip = 500
	m.Place(Tie)

	again := New(b, 9, 9)
	again.Restore(m.Save())
	if !reflect.DeepEqual(again.Save(), m.Save()) {
		t.Errorf("restored %+v, want %+v", again.Save(), m.Save())
	}
	if again.ToCut != m.ToCut || again.Fresh || again.Finished != m.Finished || again.Total() != 500 || again.LastTotal() != 200 {
		t.Errorf("restored with the cut card in %d (want %d), fresh %v, %d on the table, %d remembered",
			again.ToCut, m.ToCut, again.Fresh, again.Total(), again.LastTotal())
	}

	// A new game saves a full shoe, which is still to be burnt.
	fresh := New(b, 1, 1)
	again.Restore(fresh.Save())
	if !again.Fresh || again.CardsLeft() != ShoeSize || again.ToCut != ShoeSize-CutBehind {
		t.Errorf("a full shoe was restored fresh %v with %d cards", again.Fresh, again.CardsLeft())
	}

	// A save that could not have come from play is put right.
	good := m.Save()
	for _, tt := range []struct {
		name string
		edit func(s *Saved)
	}{
		{"fewer than none of a card", func(s *Saved) { s.Shoe[0] = -1 }},
		{"more of a card than the decks hold", func(s *Saved) { s.Shoe[0] = Decks + 1 }},
		{"an empty shoe", func(s *Saved) { s.Shoe = [cards.DeckSize]int{} }},
		{"a full shoe that is finished", func(s *Saved) { *s = fresh.Save(); s.Finished = true }},
		{"a shoe one card short", func(s *Saved) { *s = fresh.Save(); s.Shoe[5]-- }},
		{"a shoe finished before the cut card", func(s *Saved) { s.Finished = true }},
	} {
		s := good
		tt.edit(&s)
		again.Restore(s)
		if !again.Fresh || again.Finished || again.CardsLeft() != ShoeSize || len(again.History) != 0 {
			t.Errorf("%s: restored with %d cards, fresh %v, finished %v", tt.name, again.CardsLeft(), again.Fresh, again.Finished)
		}
	}
	again.Restore(Saved{
		Chip:    7,
		Bets:    [Bets]int{Player: 15},
		Last:    [Bets]int{Tie: MaxBet + 10},
		History: []Outcome{Tied, 7, -1, BankerWins},
		Shoe:    good.Shoe,
	})
	if again.Chip != ChipChoices[0] || again.Total() != 0 || again.LastTotal() != 0 {
		t.Errorf("bad bets were restored: chip %d, %d on the table, %d remembered", again.Chip, again.Total(), again.LastTotal())
	}
	if !reflect.DeepEqual(again.History, []Outcome{Tied, BankerWins}) {
		t.Errorf("a bad history was restored as %v", again.History)
	}
}
