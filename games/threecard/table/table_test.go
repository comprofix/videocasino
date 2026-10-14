package table

import (
	"math"
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

// hand reads a hand of three cards.
func hand(s string) Hand {
	return Hand(deck(s))
}

func TestScore(t *testing.T) {
	// Each hand beats every one after it, but for those marked as tying
	// with the one before.
	ladder := []struct {
		cards string
		name  string
		ties  bool
	}{
		{"Qh Kh Ah", "STRAIGHT FLUSH TO THE ACE", false},
		{"3s 4s 5s", "STRAIGHT FLUSH TO THE FIVE", false},
		{"2d 3d Ad", "STRAIGHT FLUSH TO THE THREE", false},
		{"Ac Ad Ah", "THREE ACES", false},
		{"2c 2d 2h", "THREE TWOS", false},
		{"Qc Kd Ah", "STRAIGHT TO THE ACE", false},
		{"As Kd Qc", "STRAIGHT TO THE ACE", true},
		{"4c 3d 2h", "STRAIGHT TO THE FOUR", false},
		{"2c Ad 3h", "STRAIGHT TO THE THREE", false},
		{"Ac Kc Jc", "FLUSH, ACE HIGH", false},
		{"Ad Kd Td", "FLUSH, ACE HIGH", false},
		{"Ah 9h 8h", "FLUSH, ACE HIGH", false},
		{"7s 5s 2s", "FLUSH, SEVEN HIGH", false},
		{"Ac Ad Kh", "PAIR OF ACES", false},
		{"Ac Ad 2h", "PAIR OF ACES", false},
		{"Kc Kd Ah", "PAIR OF KINGS", false},
		{"2c 2d Ah", "PAIR OF TWOS", false},
		{"3c 2d 2h", "PAIR OF TWOS", false},
		{"Ac Kd Jh", "ACE HIGH", false},
		{"Kc 2d Ah", "ACE HIGH", false},
		{"Ac Qd Jh", "ACE HIGH", false},
		{"Qc 6d 4h", "QUEEN HIGH", false},
		{"Qs 6h 4d", "QUEEN HIGH", true},
		{"Qc 6d 3h", "QUEEN HIGH", false},
		{"Qc 3d 2h", "QUEEN HIGH", false},
		{"Jc Td 8h", "JACK HIGH", false},
		{"5c 3d 2h", "FIVE HIGH", false},
	}
	for i, rung := range ladder {
		s := hand(rung.cards).Score()
		if s.String() != rung.name {
			t.Errorf("%s is called %q, want %q", rung.cards, s, rung.name)
		}
		if i == 0 {
			continue
		}
		above := hand(ladder[i-1].cards).Score()
		if rung.ties != (s == above) || !rung.ties && s >= above {
			t.Errorf("%s (%d) against %s (%d): ties %v", rung.cards, s, ladder[i-1].cards, above, rung.ties)
		}
	}

	// The order the cards are held in makes no difference.
	if hand("Kc 2d Ah").Score() != hand("Ah Kc 2d").Score() || hand("2h 2c 3d").Score() != hand("3d 2h 2c").Score() {
		t.Error("a hand scores differently in another order")
	}

	// The dealer needs a queen high.
	for cards, want := range map[string]bool{
		"Qc 3d 2h": true, "Jc Td 8h": false, "Kc 5d 2h": true, "2c 2d 3h": true, "4c 3d 2s": true, "7s 5s 2s": true,
	} {
		if got := hand(cards).Score().Qualifies(); got != want {
			t.Errorf("the dealer qualifies with %s: %v", cards, got)
		}
	}
}

func TestSettle(t *testing.T) {
	for _, c := range []struct {
		name           string
		player, dealer string
		outcome        Outcome
		paid           int // for each credit of the ante
	}{
		{"the dealer does not qualify", "Kc 5d 2h", "Jc Td 8h", NotQualified, 3},
		{"the dealer does not qualify, and has the better hand", "9c 5d 2h", "Jc Td 8h", NotQualified, 3},
		{"the player's hand is better", "Ac 5d 2h", "Qc Td 8h", Won, 4},
		{"the dealer's hand is better", "Kc 5d 2h", "Ac Td 8h", Lost, 0},
		{"the hands tie", "Kc 5d 2h", "Kd 5h 2s", Pushed, 2},
		{"a straight wins, with its bonus", "4c 5d 6h", "Qc Qd 8h", Won, 5},
		{"a straight loses, and keeps its bonus", "4c 5d 6h", "7s 8s 9s", Lost, 1},
		{"three of a kind against no hand", "4c 4d 4h", "Jc Td 8h", NotQualified, 7},
		{"a straight flush wins", "4c 5c 6c", "Ac Ad Ah", Won, 9},
		{"a flush has no bonus", "Ac 9c 6c", "Kd Kh 2s", Won, 4},
	} {
		o, paid := Settle(hand(c.player).Score(), hand(c.dealer).Score())
		if o != c.outcome || paid != c.paid {
			t.Errorf("%s: outcome %d paying %d, want %d paying %d", c.name, o, paid, c.outcome, c.paid)
		}
	}
}

// TestOdds checks the odds worked out against the figures published for
// the game, and against counts that can be made by hand.
func TestOdds(t *testing.T) {
	o := NewOdds()
	near := func(name string, got, want, within float64) {
		t.Helper()
		if math.Abs(got-want) > within {
			t.Errorf("%s is %.6f, want %.6f", name, got, want)
		}
	}
	// How many of the 22,100 hands make each thing.
	counts := [Categories]float64{
		HighCard: 16440, Pair: 3744, Flush: 1096, Straight: 720, ThreeOfAKind: 52, StraightFlush: 48,
	}
	for c, n := range counts {
		near("the chance of a "+Category(c).String(), o.Chance[c], n/22100, 1e-12)
	}
	// Pair Plus, from those counts and what it pays.
	near("the return on Pair Plus", o.PairPlusReturn, (3744*2+1096*5+720*7+52*31+48*41)/22100.0, 1e-12)
	near("the return on Pair Plus", o.PairPlusReturn, 0.9768, 0.0001)

	near("what the ante loses", o.AnteEdge, 0.0337, 0.0001)
	near("the return on the ante and the play bet", o.AnteReturn, 1-0.0201, 0.0001)
	near("the part of hands played", o.Played, 0.6742, 0.0001)

	// The weakest hand worth playing is a queen, a six and a four.
	if o.Weakest.String() != "QUEEN HIGH" {
		t.Errorf("the weakest hand worth playing is %v", o.Weakest)
	}
	for cards, want := range map[string]bool{
		"Qc 6d 4h": true, "Qc 6d 3h": false, "Qc 7d 2h": true, "Qc 5d 4h": false, "Jc Td 8s": false, "Kc 3d 2h": true, "2c 2d 3h": true,
	} {
		a := Advise(hand(cards))
		if a.Best != want || a.Best != (a.Play > -1) {
			t.Errorf("%s is worth %.4f played: best to play %v, want %v", cards, a.Play, a.Best, want)
		}
	}
	t.Logf("ante and play return %.4f%% (the ante loses %.4f%%), %.2f%% of hands played; Pair Plus returns %.4f%%",
		o.AnteReturn*100, o.AnteEdge*100, o.Played*100, o.PairPlusReturn*100)
}

// TestPlayReturn plays a great many hands by the best play and checks that
// what the bets pay back is what the odds say.
func TestPlayReturn(t *testing.T) {
	const hands = 300_000
	o := NewOdds()
	weakest := hand("Qc 6d 4h").Score()
	m := New(bank.New(), 21, 22)
	var staked, paid, pairStaked, pairPaid float64
	for range hands {
		m.Bank.Credits = 10_000
		m.Bets = [Bets]int{PairPlus: 100, Ante: 100}
		if !m.Deal() {
			t.Fatal("could not deal")
		}
		var r Result
		if m.Hand.Score() >= weakest {
			r, _ = m.Play()
		} else {
			r, _ = m.Fold()
		}
		staked += float64(r.Stake[Ante] + r.Play)
		paid += float64(r.Paid)
		pairStaked += 100
		pairPaid += float64(r.PairPaid)
	}
	if got := paid / staked; math.Abs(got-o.AnteReturn) > 0.006 {
		t.Errorf("the ante and the play bet paid back %.4f over %d hands, want %.4f", got, hands, o.AnteReturn)
	}
	if got := pairPaid / pairStaked; math.Abs(got-o.PairPlusReturn) > 0.03 {
		t.Errorf("Pair Plus paid back %.4f over %d hands, want %.4f", got, hands, o.PairPlusReturn)
	}
}

func TestBets(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 100
	if !m.Place(Ante) || !m.Place(Ante) || !m.Place(PairPlus) {
		t.Fatal("could not put three chips down")
	}
	if m.Bets[Ante] != 200 || m.Total() != 300 || Needed(m.Bets) != 500 || b.Credits != bank.StartCredits {
		t.Errorf("%d on the ante, %d in all needing %d, %d in the bank", m.Bets[Ante], m.Total(), Needed(m.Bets), b.Credits)
	}
	m.Chip = 500
	if !m.Remove(Ante) || m.Bets[Ante] != 0 || m.Remove(Ante) {
		t.Errorf("taking 500 off a bet of 200 left %d", m.Bets[Ante])
	}
	// There is no hand without an ante.
	if m.Deal() {
		t.Error("dealt on Pair Plus alone")
	}
	m.Chip = 1000
	b.Credits = 100_000
	m.Clear()
	for range MaxBet / 1000 {
		if !m.Place(Ante) {
			t.Fatal("could not bet up to the limit")
		}
	}
	if m.Place(Ante) || m.Bets[Ante] != MaxBet {
		t.Errorf("a bet holds %d", m.Bets[Ante])
	}
	// The bank must cover the ante twice over, for the play bet.
	m.Clear()
	b.Credits = 1999
	if m.Place(Ante) {
		t.Error("put down an ante of 1000 with 1999 in the bank")
	}
	b.Credits = 2000
	if !m.Place(Ante) || m.Place(PairPlus) {
		t.Error("with 2000 in the bank an ante of 1000 is all there is room for")
	}
	if m.Rebet() {
		t.Error("bet again before anything had been bet")
	}
}

// TestHand deals hands whose cards are known and plays or folds them.
func TestHand(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	m.Chip = 100
	bet := func() {
		m.Clear()
		m.Place(Ante)
		m.Place(PairPlus)
	}

	// A pair, played against a dealer who qualifies with less.
	bet()
	m.next = deck("7c 7d Kh Qs 9d 4c")
	had := b.Credits
	if !m.Deal() || !m.Dealt {
		t.Fatal("could not deal")
	}
	if m.PairPaid != 200 || b.Credits != had-200+200 || m.Total() != 0 || m.LastTotal() != 200 {
		t.Errorf("after the deal: Pair Plus paid %d, bank from %d to %d, %d on the table", m.PairPaid, had, b.Credits, m.Total())
	}
	// Nothing can be bet or dealt while the hand waits.
	if m.Place(Ante) || m.Rebet() || m.Deal() {
		t.Error("bet or dealt with a hand waiting")
	}
	r, ok := m.Play()
	if !ok || m.Dealt || r.Outcome != Won || r.Play != 100 || r.Paid != 400 || r.PairPaid != 200 {
		t.Fatalf("played: %+v", r)
	}
	if r.Player != hand("7c 7d Kh") || r.Dealer != hand("Qs 9d 4c") || r.Staked() != 300 || r.Win() != 600 {
		t.Errorf("played %v against %v: staked %d, won %d", r.Player, r.Dealer, r.Staked(), r.Win())
	}
	if b.Credits != had-300+600 {
		t.Errorf("the bank went from %d to %d", had, b.Credits)
	}
	if _, ok := m.Play(); ok {
		t.Error("played with no hand waiting")
	}
	if _, ok := m.Fold(); ok {
		t.Error("folded with no hand waiting")
	}

	// A poor hand, folded: the ante and Pair Plus are lost and no cards
	// are dealt to the dealer.
	if !m.Rebet() {
		t.Fatal("could not bet the same again")
	}
	m.next = deck("9c 5d 2h")
	had = b.Credits
	m.Deal()
	r, ok = m.Fold()
	if !ok || m.Dealt || r.Outcome != Folded || r.Play != 0 || r.Win() != 0 || b.Credits != had-200 {
		t.Errorf("folded: %+v, bank from %d to %d", r, had, b.Credits)
	}
	if len(m.next) != 0 {
		t.Error("a card was not dealt")
	}

	// The dealer's cards come from the rest of the deck: never the player's.
	for range 3000 {
		b.Credits = 10_000
		bet()
		m.Deal()
		r, _ := m.Play()
		seen := map[cards.Card]bool{}
		for _, c := range append(r.Player[:], r.Dealer[:]...) {
			if !c.Valid() || seen[c] {
				t.Fatalf("dealt %v and %v from one deck", r.Player, r.Dealer)
			}
			seen[c] = true
		}
	}

	// With too little for the play bet the hand can only be folded.
	bet()
	m.Deal()
	b.Credits = 99
	if _, ok := m.Play(); ok || !m.Dealt || b.Credits != 99 {
		t.Error("played with too little in the bank for the bet")
	}
}

func TestSaveAndRestore(t *testing.T) {
	b := bank.New()
	m := New(b, 3, 4)
	m.Chip = 50
	m.Place(Ante)
	m.Place(PairPlus)
	m.next = deck("4s 5s 6s")
	m.Deal()
	m.Chip = 500

	again := New(b, 9, 9)
	again.Restore(m.Save())
	if again.Save() != m.Save() {
		t.Errorf("restored %+v, want %+v", again.Save(), m.Save())
	}
	if !again.Dealt || again.Hand != hand("4s 5s 6s") || again.PairPaid != 50*41 || again.Chip != 500 || again.LastTotal() != 100 {
		t.Errorf("restored a hand of %v, Pair Plus paid %d, chip %d", again.Hand, again.PairPaid, again.Chip)
	}
	// It can be played out where it was left.
	had := b.Credits
	r, ok := again.Play()
	if !ok || b.Credits != had-50+r.Paid {
		t.Errorf("played the restored hand: %+v", r)
	}

	// A game with no hand waiting keeps only the chip and the bets.
	m.Fold()
	m.Chip = 10
	m.Place(Ante)
	want := Saved{Chip: 10, Bets: [Bets]int{Ante: 10}, Last: [Bets]int{PairPlus: 50, Ante: 50}}
	if got := m.Save(); got != want {
		t.Errorf("saved %+v, want %+v", got, want)
	}

	// A save that could not have come from play is put right.
	good := Saved{Chip: 100, Last: [Bets]int{Ante: 100}, Dealt: true, Stake: [Bets]int{PairPlus: 50, Ante: 100}, Hand: hand("7c 7d Kh"), PairPaid: 100}
	again.Restore(good)
	if !again.Dealt {
		t.Fatal("a good hand was not restored")
	}
	for _, tt := range []struct {
		name string
		edit func(s *Saved)
	}{
		{"a card twice", func(s *Saved) { s.Hand[1] = s.Hand[0] }},
		{"a card that is not one", func(s *Saved) { s.Hand[2] = 52 }},
		{"a hand with no ante", func(s *Saved) { s.Stake[Ante] = 0 }},
		{"an ante over the limit", func(s *Saved) { s.Stake[Ante] = MaxBet + 10 }},
		{"Pair Plus paid too much", func(s *Saved) { s.PairPaid = 5000 }},
		{"Pair Plus not paid", func(s *Saved) { s.PairPaid = 0 }},
	} {
		s := good
		tt.edit(&s)
		again.Restore(s)
		if again.Dealt || again.PairPaid != 0 || again.Chip != 100 || again.LastTotal() != 100 {
			t.Errorf("%s: restored dealt %v with Pair Plus paid %d", tt.name, again.Dealt, again.PairPaid)
		}
	}
	again.Restore(Saved{Chip: 7, Bets: [Bets]int{Ante: 15}, Last: [Bets]int{PairPlus: -10}})
	if again.Chip != ChipChoices[0] || again.Total() != 0 || again.LastTotal() != 0 {
		t.Errorf("bad bets were restored: chip %d, %d on the table, %d remembered", again.Chip, again.Total(), again.LastTotal())
	}
}
