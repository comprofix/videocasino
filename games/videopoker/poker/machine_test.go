package poker

import (
	"testing"

	"videocasino/bank"
	"videocasino/cards"
)

// distinct reports whether a hand is five different cards.
func distinct(h [HandSize]cards.Card) bool {
	return validHand(h)
}

func TestNew(t *testing.T) {
	m := New(bank.New(), 1, 2)
	if m.Bank.Credits != bank.StartCredits || m.Coins != 1 || m.CoinValue != CoinValues[0] || m.Dealt {
		t.Errorf("a new machine starts with %+v", m.Save())
	}
	if m.Bet() != 1 {
		t.Errorf("the smallest bet is %d", m.Bet())
	}
}

func TestDeal(t *testing.T) {
	m := New(bank.New(), 1, 2)
	m.Coins, m.CoinValue = 3, 25
	made, ok := m.Deal()
	if !ok {
		t.Fatal("could not deal")
	}
	if m.Bank.Credits != bank.StartCredits-75 {
		t.Errorf("credits after a bet of 75 are %d", m.Bank.Credits)
	}
	if !m.Dealt || m.Held != [HandSize]bool{} {
		t.Errorf("after the deal: dealt %v, held %v", m.Dealt, m.Held)
	}
	if !distinct(m.Hand) {
		t.Errorf("dealt %v", m.Hand)
	}
	if made != Evaluate(m.Hand) {
		t.Errorf("deal reported %v for %v", made, m.Hand)
	}

	// A second deal has to wait for the draw.
	before := m.Save()
	if _, ok := m.Deal(); ok || m.Save() != before {
		t.Error("dealt again before the draw")
	}
}

func TestDealTooFewCredits(t *testing.T) {
	m := New(bank.New(), 1, 2)
	m.Coins, m.CoinValue = 5, 100
	m.Bank.Credits = 499
	if _, ok := m.Deal(); ok || m.Bank.Credits != 499 || m.Dealt {
		t.Errorf("dealt a hand of 500 for 499 credits: %+v", m.Save())
	}
	m.Bank.Credits = 500
	if _, ok := m.Deal(); !ok || m.Bank.Credits != 0 {
		t.Errorf("could not bet the last 500 credits: %+v", m.Save())
	}
}

func TestDrawNeedsDeal(t *testing.T) {
	m := New(bank.New(), 1, 2)
	if _, ok := m.Draw(); ok {
		t.Error("drew with no hand dealt")
	}
	m.ToggleHold(0)
	if m.Held[0] {
		t.Error("held a card with no hand dealt")
	}
}

func TestToggleHold(t *testing.T) {
	m := New(bank.New(), 1, 2)
	m.Deal()
	m.ToggleHold(1)
	m.ToggleHold(4)
	m.ToggleHold(4)
	if m.Held != [HandSize]bool{1: true} {
		t.Errorf("held %v", m.Held)
	}
}

// TestDraw plays many hands, holding a different set of cards each time.
func TestDraw(t *testing.T) {
	m := New(bank.New(), 3, 4)
	m.Coins, m.CoinValue = 5, 5
	for n := range 20_000 {
		m.Bank.Credits = bank.StartCredits
		if _, ok := m.Deal(); !ok {
			t.Fatal("could not deal")
		}
		dealt := m.Hand
		for i := range HandSize {
			if n>>i&1 == 1 {
				m.ToggleHold(i)
			}
		}
		held := m.Held
		r, ok := m.Draw()
		if !ok {
			t.Fatal("could not draw")
		}

		if m.Dealt {
			t.Fatal("still waiting for a draw after the draw")
		}
		if r.Hand != m.Hand || !distinct(r.Hand) {
			t.Fatalf("drew to %v, machine has %v", r.Hand, m.Hand)
		}
		for i, c := range r.Hand {
			if held[i] && c != dealt[i] {
				t.Fatalf("held card %v changed to %v", dealt[i], c)
			}
			// A card that was let go is out of the deck.
			for j, old := range dealt {
				if !held[i] && c == old {
					t.Fatalf("drew %v, which was dealt at place %d (hand %v, held %v)", c, j, dealt, held)
				}
			}
		}
		if r.Made != Evaluate(r.Hand) {
			t.Fatalf("%v reported as %v", r.Hand, r.Made)
		}
		if want := NineSix.Pay(r.Made, 5) * 5; r.Win != want {
			t.Fatalf("%v won %d, want %d", r.Made, r.Win, want)
		}
		if m.Bank.Credits != bank.StartCredits-25+r.Win {
			t.Fatalf("credits are %d after a bet of 25 and a win of %d", m.Bank.Credits, r.Win)
		}
	}
}

// TestSeeds checks that fixed seeds fix the cards, and that different
// seeds give different ones.
func TestSeeds(t *testing.T) {
	play := func(seed1, seed2 uint64) (hands [50][HandSize]cards.Card) {
		m := New(bank.New(), seed1, seed2)
		for i := range hands {
			m.Deal()
			r, _ := m.Draw()
			hands[i] = r.Hand
		}
		return hands
	}
	if play(5, 6) != play(5, 6) {
		t.Error("the same seeds gave different cards")
	}
	if play(5, 6) == play(5, 7) {
		t.Error("different seeds gave the same cards")
	}
}

// TestNewRandom checks that a machine for play is seeded afresh as it goes.
func TestNewRandom(t *testing.T) {
	a, b := NewRandom(bank.New()), NewRandom(bank.New())
	same := true
	for range 3 * reseedEvery {
		a.Deal()
		b.Deal()
		ra, _ := a.Draw()
		rb, _ := b.Draw()
		if !distinct(ra.Hand) {
			t.Fatalf("drew to %v", ra.Hand)
		}
		if ra.Hand != rb.Hand {
			same = false
		}
	}
	if same {
		t.Error("two machines for play were dealt the same cards")
	}
	if a.reseedIn < 1 || a.reseedIn > reseedEvery {
		t.Errorf("%d hands until the next seed", a.reseedIn)
	}
}
