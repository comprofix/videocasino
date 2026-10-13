package table

import (
	"slices"
	"strings"
	"testing"

	"videocasino/bank"
	"videocasino/cards"
)

// deck reads cards written as String writes them, such as "As Kd 7h".
func deck(s string) []cards.Card {
	var out []cards.Card
	for _, name := range strings.Fields(s) {
		rank := strings.IndexByte("23456789TJQKA", name[0])
		suit := strings.IndexByte("cdhs", name[1])
		if len(name) != 2 || rank < 0 || suit < 0 {
			panic("not a card: " + name)
		}
		out = append(out, cards.New(cards.Two+cards.Rank(rank), cards.Suit(suit)))
	}
	return out
}

// stacked returns a machine with the given bets whose next cards are the
// ones named. They are dealt in order: the player's first card, the
// dealer's, the player's second, and then whatever the play calls for.
func stacked(bet, side int, next string) *Machine {
	m := New(bank.New(), 1, 2)
	m.Bet, m.SideBet = bet, side
	m.next = deck(next)
	return m
}

// settled checks that the hand is over, that every card stacked was dealt
// and no more, and what was paid.
func settled(t *testing.T, m *Machine, paid int, outcomes ...Outcome) {
	t.Helper()
	if m.Dealt {
		t.Fatalf("the hand is still to be played: %+v", m.Hands)
	}
	if len(m.next) != 0 {
		t.Errorf("%d of the cards stacked were not dealt", len(m.next))
	}
	if m.Last.Paid != paid || !slices.Equal(m.Last.Outcomes, outcomes) {
		t.Errorf("paid %d for %v, want %d for %v (dealer %v, hands %+v)",
			m.Last.Paid, m.Last.Outcomes, paid, outcomes, m.Dealer, m.Hands)
	}
}

// credits checks what the bank holds against what it started with.
func credits(t *testing.T, m *Machine, change int) {
	t.Helper()
	if got := m.Bank.Credits - bank.StartCredits; got != change {
		t.Errorf("the bank is %+d credits from where it started, want %+d", got, change)
	}
}

func TestTotal(t *testing.T) {
	tests := []struct {
		hand  string
		total int
		soft  bool
	}{
		{"Ts 7d", 17, false},
		{"As 6d", 17, true},
		{"As 6d Kc", 17, false}, // the ace has to count as 1
		{"As Ad", 12, true},
		{"As Ad 9c", 21, true},
		{"As Ad 9c Kd", 21, false},
		{"Ks Qd Jc", 30, false},
		{"As Kd", 21, true},
		{"2s 3d", 5, false},
	}
	for _, tt := range tests {
		total, soft := Total(deck(tt.hand))
		if total != tt.total || soft != tt.soft {
			t.Errorf("%s adds up to %d (soft %v), want %d (soft %v)", tt.hand, total, soft, tt.total, tt.soft)
		}
	}
	if !IsBlackjack(deck("As Kd")) || IsBlackjack(deck("As 5d 5c")) || IsBlackjack(deck("Ks Qd")) {
		t.Error("IsBlackjack has it wrong")
	}
}

func TestNew(t *testing.T) {
	m := New(bank.New(), 1, 2)
	if m.Bet != BetChoices[0] || m.SideBet != 0 || m.Dealt {
		t.Errorf("a new machine starts with %+v", m.Save())
	}
	// Nothing can be played before the deal.
	if m.Hit() || m.Stand() || m.Double() || m.Split() || m.CanDouble() || m.CanSplit() {
		t.Error("played a hand that was not dealt")
	}
	// Every bet pays whole credits for a blackjack.
	for _, bet := range BetChoices {
		if bet%2 != 0 {
			t.Errorf("a bet of %d cannot be paid 3 to 2 in whole credits", bet)
		}
	}
}

func TestDeal(t *testing.T) {
	m := stacked(20, 0, "Ts 7h 9s")
	if !m.Deal() {
		t.Fatal("could not deal")
	}
	credits(t, m, -20)
	if !m.Dealt || m.Stake != 20 || len(m.Hands) != 1 || m.Active != 0 {
		t.Fatalf("after the deal: %+v", m.Save())
	}
	if !slices.Equal(m.Hands[0].Cards, deck("Ts 9s")) || !slices.Equal(m.Dealer, deck("7h")) {
		t.Errorf("dealt %v against %v", m.Hands[0].Cards, m.Dealer)
	}

	// A second deal has to wait for the hand to be played.
	if m.Deal() {
		t.Error("dealt again before the hand was played")
	}
	credits(t, m, -20)
}

func TestDealTooFewCredits(t *testing.T) {
	m := stacked(20, 10, "Ts 7h 9s")
	m.Bank.Credits = 29
	if m.Deal() || m.Dealt || m.Bank.Credits != 29 {
		t.Errorf("dealt a hand of 30 for 29 credits")
	}
	m.Bank.Credits = 30
	if !m.Deal() || m.Bank.Credits != 0 {
		t.Errorf("could not bet the last 30 credits")
	}
}

func TestStand(t *testing.T) {
	// 19 beats the dealer's 17.
	m := stacked(20, 0, "Ts 7h 9s Td")
	m.Deal()
	m.Stand()
	settled(t, m, 40, Won)
	credits(t, m, +20)

	// The dealer draws to 21.
	m = stacked(20, 0, "Ts 6h 8s 5d Kc")
	m.Deal()
	m.Stand()
	settled(t, m, 0, Lost)
	credits(t, m, -20)

	// The dealer stands on a soft 17.
	m = stacked(20, 0, "Ts 6h 8s Ad")
	m.Deal()
	m.Stand()
	settled(t, m, 40, Won)

	// The same total is a push.
	m = stacked(20, 0, "Ts 7h 7s Kd")
	m.Deal()
	m.Stand()
	settled(t, m, 20, Pushed)
	credits(t, m, 0)

	// The dealer goes bust.
	m = stacked(20, 0, "Ts 6h 2s Kd 9c")
	m.Deal()
	m.Stand()
	settled(t, m, 40, Won)
}

func TestHit(t *testing.T) {
	// A hand that goes bust is lost there and then: the dealer has no need
	// to draw.
	m := stacked(20, 0, "Ts 7h 6s Kd")
	m.Deal()
	if !m.Hit() {
		t.Fatal("could not hit")
	}
	settled(t, m, 0, Lost)
	credits(t, m, -20)
	if len(m.Dealer) != 1 {
		t.Errorf("the dealer drew to %v against a bust hand", m.Dealer)
	}

	// A hand that reaches 21 stands by itself.
	m = stacked(20, 0, "5s 7h 6s Kd 9c Td")
	m.Deal()
	m.Hit()
	settled(t, m, 40, Won)

	// A hand short of 21 can go on.
	m = stacked(20, 0, "2s 7h 3s 4d 5c Kd")
	m.Deal()
	m.Hit()
	m.Hit()
	if !m.Dealt || len(m.Hands[0].Cards) != 4 {
		t.Fatalf("after two cards: %+v", m.Hands)
	}
	m.Stand()
	settled(t, m, 0, Lost) // 14 against 17
}

func TestBlackjack(t *testing.T) {
	// It pays 3 to 2 without being played. A dealer who cannot have one
	// does not draw.
	m := stacked(10, 0, "As 9c Kd")
	m.Deal()
	settled(t, m, 25, WonBlackjack)
	credits(t, m, +15)
	if len(m.Dealer) != 1 {
		t.Errorf("the dealer drew to %v", m.Dealer)
	}

	// A dealer who could have one takes a card to see.
	m = stacked(10, 0, "As Ah Kd 5c")
	m.Deal()
	settled(t, m, 25, WonBlackjack)
	if len(m.Dealer) != 2 {
		t.Errorf("the dealer has %v", m.Dealer)
	}

	// Two blackjacks are a push.
	m = stacked(10, 0, "As Ah Kd Qc")
	m.Deal()
	settled(t, m, 10, Pushed)
	credits(t, m, 0)
	if !m.Last.DealerBlackjack {
		t.Error("the dealer's blackjack was not reported")
	}

	// A dealer's blackjack beats 21 in three cards.
	m = stacked(10, 0, "5s Kh 6d Tc Ad")
	m.Deal()
	m.Hit()
	settled(t, m, 0, Lost)
}

func TestDouble(t *testing.T) {
	// Two bets ride on one more card.
	m := stacked(20, 0, "6s 6h 5d 9c Td 8s")
	m.Deal()
	if !m.CanDouble() || !m.Double() {
		t.Fatal("could not double")
	}
	if h := m.Hands[0]; h.Bet != 40 || !h.Doubled || len(h.Cards) != 3 {
		t.Errorf("the doubled hand is %+v", h)
	}
	settled(t, m, 80, Won)
	credits(t, m, +40)

	// A dealer's blackjack takes only the bet first made.
	m = stacked(20, 0, "6s Ah 5d 9c Kd")
	m.Deal()
	m.Double()
	settled(t, m, 20, Lost)
	credits(t, m, -20)

	// Only a hard 9, 10 or 11 can be doubled.
	doubles := []struct {
		hand string
		can  bool
	}{
		{"5s 4d", true},
		{"6s 4d", true},
		{"9s 2d", true},
		{"5s 3d", false},    // 8
		{"7s 5d", false},    // 12
		{"Ts 6d", false},    // 16
		{"As 8d", false},    // a soft 19, not a 9
		{"As 9d", false},    // a soft 20, not a 10
		{"As Ad", false},    // a soft 12
		{"5s 4d 2c", false}, // 11, but in three cards
	}
	for _, tt := range doubles {
		if got := CanDoubleOn(deck(tt.hand)); got != tt.can {
			t.Errorf("%s can be doubled: %v, want %v", tt.hand, got, tt.can)
		}
	}
	m = stacked(20, 0, "Ts 6h 6d")
	m.Deal()
	if m.CanDouble() || m.Double() {
		t.Error("doubled a hand of 16")
	}
	credits(t, m, -20)
	m = stacked(20, 0, "As 6h 8d")
	m.Deal()
	if m.CanDouble() || m.Double() {
		t.Error("doubled a soft 19")
	}

	// Only the first two cards can be doubled.
	m = stacked(20, 0, "2s 7h 3d 4c")
	m.Deal()
	m.Hit()
	if m.CanDouble() || m.Double() {
		t.Error("doubled a hand of three cards")
	}

	// The bank has to cover the second bet.
	m = stacked(20, 0, "6s 6h 5d")
	m.Bank.Credits = 39
	m.Deal()
	if m.CanDouble() || m.Double() || m.Bank.Credits != 19 {
		t.Errorf("doubled a bet of 20 with %d credits", m.Bank.Credits)
	}
}

func TestSplit(t *testing.T) {
	// Each card starts a hand of its own, at the same bet. The second hand
	// is dealt its card when its turn comes, and can be doubled.
	m := stacked(20, 0, "8s 6h 8d 3c Tc 2h 9d Kd 7s")
	m.Deal()
	if !m.CanSplit() || !m.Split() {
		t.Fatal("could not split")
	}
	credits(t, m, -40)
	if len(m.Hands) != 2 || m.Active != 0 || !slices.Equal(m.Hands[0].Cards, deck("8s 3c")) || !slices.Equal(m.Hands[1].Cards, deck("8d")) {
		t.Fatalf("after the split: %+v", m.Hands)
	}
	if m.CanSplit() {
		t.Error("a hand can be split a second time")
	}
	m.Hit() // 21: on to the second hand
	if m.Active != 1 || !slices.Equal(m.Hands[1].Cards, deck("8d 2h")) {
		t.Fatalf("after the first hand: active %d, hands %+v", m.Active, m.Hands)
	}
	if !m.Double() {
		t.Fatal("could not double after a split")
	}
	settled(t, m, 120, Won, Won) // 40 for the first hand, 80 for the doubled one
	credits(t, m, +60)

	// Any two cards worth ten can be split, but not two that differ.
	m = stacked(20, 0, "Ks 6h Qd")
	m.Deal()
	if !m.CanSplit() {
		t.Error("a king and a queen cannot be split")
	}
	m = stacked(20, 0, "8s 6h 9d")
	m.Deal()
	if m.CanSplit() || m.Split() {
		t.Error("split an eight and a nine")
	}

	// The bank has to cover the second bet.
	m = stacked(20, 0, "8s 6h 8d")
	m.Bank.Credits = 39
	m.Deal()
	if m.CanSplit() || m.Split() {
		t.Error("split a bet of 20 with 19 credits left")
	}
}

func TestSplitAces(t *testing.T) {
	// Split aces get one card each. A ten on one makes 21, which pays even
	// money: it is not a blackjack.
	m := stacked(20, 0, "As 6h Ad Kc 5d Td 9s")
	m.Deal()
	if !m.Split() {
		t.Fatal("could not split aces")
	}
	settled(t, m, 80, Won, Won)
	credits(t, m, +40)
	if len(m.Hands[0].Cards) != 2 || len(m.Hands[1].Cards) != 2 {
		t.Errorf("split aces were dealt %+v", m.Hands)
	}
}

func TestSplitAgainstBlackjack(t *testing.T) {
	// A dealer's blackjack takes only the bet first made.
	m := stacked(20, 0, "8s Ah 8d 3c 2h Kd")
	m.Deal()
	m.Split()
	m.Stand()
	m.Stand()
	settled(t, m, 20, Lost, Lost)
	credits(t, m, -20)

	// Even when both hands went bust.
	m = stacked(20, 0, "8s Ah 8d Tc Kc Td Qc Jd")
	m.Deal()
	m.Split()
	m.Hit()
	m.Hit()
	settled(t, m, 20, Lost, Lost)
	credits(t, m, -20)

	// With no blackjack two bust hands lose both bets.
	m = stacked(20, 0, "8s Ah 8d Tc Kc Td Qc 5d")
	m.Deal()
	m.Split()
	m.Hit()
	m.Hit()
	settled(t, m, 0, Lost, Lost)
	credits(t, m, -40)

	// One hand bust and one standing: the dealer plays for the one left.
	m = stacked(20, 0, "8s 6h 8d Tc Kc Td 4s 8c")
	m.Deal()
	m.Split()
	m.Hit()   // the first hand busts
	m.Stand() // the second stands on 18, and so does the dealer
	settled(t, m, 20, Lost, Pushed)
	credits(t, m, -20)
}

func TestPerfectPairs(t *testing.T) {
	tests := []struct {
		cards   string
		pair    PairKind
		sideWin int
	}{
		{"Kh 5c Kh", PerfectPair, 250},
		{"Kh 5c Kd", ColouredPair, 120},
		{"Kh 5c Ks", MixedPair, 60},
		{"Kh 5c Qh", NoPair, 0},
	}
	for _, tt := range tests {
		m := stacked(20, 10, tt.cards)
		m.Deal()
		if m.Pair != tt.pair || m.SideWin != tt.sideWin {
			t.Errorf("%s: %v paying %d, want %v paying %d", tt.cards, m.Pair, m.SideWin, tt.pair, tt.sideWin)
		}
		// The side bet is paid at the deal, with the hand still to play.
		want := -30
		if tt.sideWin > 0 {
			want += 10 + tt.sideWin
		}
		credits(t, m, want)
		if !m.Dealt {
			t.Errorf("%s: the hand was settled with the side bet", tt.cards)
		}
	}

	// With nothing bet on it a pair pays nothing.
	m := stacked(20, 0, "Kh 5c Kh")
	m.Deal()
	if m.Pair != PerfectPair || m.SideWin != 0 {
		t.Errorf("with no side bet: %v paying %d", m.Pair, m.SideWin)
	}
	credits(t, m, -20)
}

// TestPlay plays many hands at random and checks the cards and the money.
func TestPlay(t *testing.T) {
	m := New(bank.New(), 3, 4)
	m.Bet, m.SideBet = 20, 10
	for n := range 50_000 {
		m.Bank.Credits = bank.StartCredits
		if !m.Deal() {
			t.Fatal("could not deal")
		}
		side := 0
		if m.Pair != NoPair {
			side = m.Side + m.SideWin
		}
		for step := 0; m.Dealt; step++ {
			// A different choice each time, falling back on what is allowed.
			switch choice := (n + step*7) % 5; {
			case choice == 0 && m.Split():
			case choice == 1 && m.Double():
			case choice <= 3:
				m.Hit()
			default:
				m.Stand()
			}
			if step > 40 {
				t.Fatal("the hand never ended")
			}
		}

		var seen [cards.DeckSize]int
		for _, c := range m.Dealer {
			seen[c]++
		}
		staked := 0
		for _, h := range m.Hands {
			staked += h.Bet
			for _, c := range h.Cards {
				seen[c]++
			}
			if !h.Done {
				t.Fatalf("a hand was left unfinished: %+v", m.Hands)
			}
		}
		for c, k := range seen {
			if k > Decks {
				t.Fatalf("%v was dealt %d times from %d decks", cards.Card(c), k, Decks)
			}
		}
		r := m.Last
		if r.Staked != staked || len(r.Outcomes) != len(m.Hands) {
			t.Fatalf("settled %d hands for %d staked: %+v", len(r.Outcomes), staked, r)
		}
		if want := bank.StartCredits - 10 - staked + side + r.Paid; m.Bank.Credits != want {
			t.Fatalf("the bank has %d credits, want %d (staked %d, side %d, paid %d)", m.Bank.Credits, want, staked, side, r.Paid)
		}
		// The dealer never stops short when a hand is still in.
		alive := false
		for i := range m.Hands {
			alive = alive || !m.Hands[i].Bust()
		}
		natural := len(m.Hands) == 1 && IsBlackjack(m.Hands[0].Cards)
		if total, _ := Total(m.Dealer); alive && !natural && total < DealerStands {
			t.Fatalf("the dealer stood on %d: %v", total, m.Dealer)
		}
	}
}

// TestSeeds checks that fixed seeds fix the cards, and that different
// seeds give different ones.
func TestSeeds(t *testing.T) {
	play := func(seed1, seed2 uint64) (dealt [50][]cards.Card) {
		m := New(bank.New(), seed1, seed2)
		for i := range dealt {
			m.Deal()
			for m.Dealt {
				m.Stand()
			}
			dealt[i] = append(slices.Clone(m.Hands[0].Cards), m.Dealer...)
		}
		return dealt
	}
	same := func(a, b [50][]cards.Card) bool {
		for i := range a {
			if !slices.Equal(a[i], b[i]) {
				return false
			}
		}
		return true
	}
	if !same(play(5, 6), play(5, 6)) {
		t.Error("the same seeds gave different cards")
	}
	if same(play(5, 6), play(5, 7)) {
		t.Error("different seeds gave the same cards")
	}
}

// TestShoe plays through many shoes and checks that hands are dealt from
// one until the cut card comes out, and that it is shuffled then and only
// then.
func TestShoe(t *testing.T) {
	m := New(bank.New(), 5, 6)
	if m.CardsLeft() != ShoeSize || m.ToCut < cutMin || m.ToCut > cutMax {
		t.Fatalf("a new shoe has %d cards with the cut card %d in", m.CardsLeft(), m.ToCut)
	}
	if cutMin != ShoeSize*2/3 || cutMax != ShoeSize*5/6 {
		t.Errorf("the cut card goes between %d and %d cards in", cutMin, cutMax)
	}

	shuffles, shallowest, deepest := 0, ShoeSize, 0
	for range 30_000 {
		m.Bank.Credits = bank.StartCredits
		left, toCut, due := m.CardsLeft(), m.ToCut, m.CutOut()
		if !m.Deal() {
			t.Fatal("could not deal")
		}
		for m.Dealt {
			if total, _ := m.Hands[m.Active].Total(); total < 15 {
				m.Hit()
			} else {
				m.Stand()
			}
		}
		dealt := len(m.Dealer)
		for _, h := range m.Hands {
			dealt += len(h.Cards)
		}

		if due {
			// The cut card came out last hand: this one is from a new shoe.
			shuffles++
			cut := m.ToCut + dealt
			shallowest, deepest = min(shallowest, cut), max(deepest, cut)
			if !m.Shuffled || m.CardsLeft() != ShoeSize-dealt || cut < cutMin || cut > cutMax {
				t.Fatalf("after a shuffle: shuffled %v, %d cards left of %d dealt, cut card %d in", m.Shuffled, m.CardsLeft(), dealt, cut)
			}
		} else {
			if m.Shuffled != (left == ShoeSize) || m.CardsLeft() != left-dealt || m.ToCut != toCut-dealt {
				t.Fatalf("within a shoe: shuffled %v, cards left from %d to %d and to the cut card from %d to %d, with %d dealt",
					m.Shuffled, left, m.CardsLeft(), toCut, m.ToCut, dealt)
			}
		}
		for c, k := range m.Shoe {
			if k < 0 || k > Decks {
				t.Fatalf("%d of %v left in the shoe", k, cards.Card(c))
			}
		}
		// At least a deck is never dealt, less what the last hand ran on.
		if m.CardsLeft() < ShoeSize-cutMax-40 {
			t.Fatalf("the shoe was dealt down to %d cards", m.CardsLeft())
		}
	}
	t.Logf("%d shuffles, with the cut card from %d to %d cards in", shuffles, shallowest, deepest)
	if shuffles < 300 {
		t.Errorf("only %d shuffles in 30000 hands", shuffles)
	}
	if shallowest > cutMin+5 || deepest < cutMax-5 {
		t.Errorf("the cut card only ever went from %d to %d cards in", shallowest, deepest)
	}
}
