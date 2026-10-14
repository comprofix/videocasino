package pokerhand

import (
	"strings"
	"testing"

	"videocasino/cards"
)

// hand makes a hand from names such as "As Kd 4c 3h 2s".
func hand(t *testing.T, names string) Hand {
	t.Helper()
	var h Hand
	fields := strings.Fields(names)
	if len(fields) != Size {
		t.Fatalf("%q is not five cards", names)
	}
	for i, f := range fields {
		r := strings.IndexByte("23456789TJQKA", f[0])
		s := strings.IndexByte("cdhs", f[1])
		if r < 0 || s < 0 {
			t.Fatalf("%q is not a card", f)
		}
		h[i] = cards.New(cards.Two+cards.Rank(r), cards.Suit(s))
	}
	return h
}

func TestCategory(t *testing.T) {
	tests := []struct {
		hand string
		want Category
		name string
	}{
		{"As Ks Qs Js Ts", RoyalFlush, "ROYAL FLUSH"},
		{"9h Kh Qh Jh Th", StraightFlush, "STRAIGHT FLUSH TO THE KING"},
		{"Ad 2d 3d 4d 5d", StraightFlush, "STRAIGHT FLUSH TO THE FIVE"},
		{"7c 7d 7h 7s 2c", FourOfAKind, "FOUR SEVENS"},
		{"3c 3d 3h Ks Kc", FullHouse, "THREES FULL OF KINGS"},
		{"2h 9h Jh 4h Kh", Flush, "FLUSH, KING HIGH"},
		{"Tc Jd Qh Ks Ac", Straight, "STRAIGHT TO THE ACE"},
		{"5c 4d 3h 2s Ac", Straight, "STRAIGHT TO THE FIVE"},
		{"8c 8d 8h Ks 2c", ThreeOfAKind, "THREE EIGHTS"},
		{"5c 5d Jh Js 2c", TwoPair, "TWO PAIR, JACKS AND FIVES"},
		{"Qc Qd 9h 4s 2c", Pair, "PAIR OF QUEENS"},
		{"Ac Kd 4h 3s 2c", HighCard, "ACE KING HIGH"},
		{"Qc Kd Ah 3s 2c", HighCard, "ACE KING HIGH"},
		{"Kc Ad Qh Js 9c", HighCard, "ACE KING HIGH"},
		{"Kc Qd Jh Ts 8c", HighCard, "KING QUEEN HIGH"},
		{"Ac Kd Qh Js 9s", HighCard, "ACE KING HIGH"},
		{"Qs Ks As 2s 3c", HighCard, "ACE KING HIGH"}, // no straight runs round the ace
	}
	for _, tt := range tests {
		s := hand(t, tt.hand).Score()
		if s.Category() != tt.want || s.String() != tt.name {
			t.Errorf("%s makes %v, %q; want %v, %q", tt.hand, s.Category(), s, tt.want, tt.name)
		}
	}
}

// TestOrder checks hands against each other: each is beaten by the one
// after it.
func TestOrder(t *testing.T) {
	order := []string{
		"7c 5d 4h 3s 2c", // the lowest hand there is
		"8c 5d 4h 3s 2c",
		"Ac Qd Jh 9s 8c",
		"Ac Kd 4h 3s 2c", // the least a dealer qualifies with
		"Ac Kd 5h 3s 2c",
		"Ac Kd Qh Js 9c",
		"2c 2d 5h 4s 3c",
		"2c 2d Ah Ks Qc",
		"3c 3d 4h 5s 6c",
		"Ac Ad 4h 3s 2c",
		"Ac Ad Kh Qs Jc",
		"3c 3d 2h 2s 4c",
		"3c 3d 2h 2s Ac",
		"Kc Kd 2h 2s 3c",
		"Ac Ad 2h 2s 3c",
		"Ac Ad Kh Ks 2c",
		"Ac Ad Kh Ks 3c",
		"2c 2d 2h 3s 4c",
		"2c 2d 2h As Kc",
		"Ac Ad Ah 2s 3c",
		"Ac 2d 3h 4s 5c", // the lowest straight
		"2c 3d 4h 5s 6c",
		"Tc Jd Qh Ks Ac",
		"2h 3h 4h 5h 7h",
		"2h 3h 4h 5h 8h",
		"Ah Kh Qh Jh 9h",
		"2c 2d 2h 3s 3c",
		"2c 2d 2h As Ac",
		"3c 3d 3h 2s 2c",
		"Ac Ad Ah Ks Kc",
		"2c 2d 2h 2s 3c",
		"2c 2d 2h 2s Ac",
		"3c 3d 3h 3s 2c",
		"Ac Ad Ah As Kc",
		"Ad 2d 3d 4d 5d",
		"9h Kh Qh Jh Th",
		"As Ks Qs Js Ts",
	}
	for i := 1; i < len(order); i++ {
		lo, hi := hand(t, order[i-1]).Score(), hand(t, order[i]).Score()
		if lo >= hi {
			t.Errorf("%s (%v) does not lose to %s (%v)", order[i-1], lo, order[i], hi)
		}
	}
}

func TestTies(t *testing.T) {
	ties := [][2]string{
		{"Ac Kd 4h 3s 2c", "Ad Kh 4s 3c 2d"},
		{"Tc Jd Qh Ks Ac", "Ts Jc Qd Kh Ad"},
		{"5c 5d Jh Js 2c", "5h 5s Jc Jd 2d"},
		{"2h 9h Jh 4h Kh", "2s 9s Js 4s Ks"},
	}
	for _, tie := range ties {
		a, b := hand(t, tie[0]).Score(), hand(t, tie[1]).Score()
		if a != b {
			t.Errorf("%s and %s do not tie", tie[0], tie[1])
		}
	}
}

// TestCount counts what every hand of five makes, against the figures any
// book on poker gives, which come from the same counting.
func TestCount(t *testing.T) {
	want := [Categories]int{
		HighCard:      1_302_540,
		Pair:          1_098_240,
		TwoPair:       123_552,
		ThreeOfAKind:  54_912,
		Straight:      10_200,
		Flush:         5_108,
		FullHouse:     3_744,
		FourOfAKind:   624,
		StraightFlush: 36,
		RoyalFlush:    4,
	}
	var got [Categories]int
	var h Hand
	for a := range cards.Card(cards.DeckSize) {
		for b := a + 1; b < cards.DeckSize; b++ {
			for c := b + 1; c < cards.DeckSize; c++ {
				for d := c + 1; d < cards.DeckSize; d++ {
					for e := d + 1; e < cards.DeckSize; e++ {
						h = Hand{a, b, c, d, e}
						got[h.Category()]++
					}
				}
			}
		}
	}
	if got != want {
		t.Errorf("the hands make %v, want %v", got, want)
	}
}

func TestLeast(t *testing.T) {
	least := Least(HighCard, cards.Ace, cards.King)
	if s := hand(t, "Ac Kd 4h 3s 2c").Score(); s < least {
		t.Errorf("ace, king, four, three, two scores %v, less than the least with an ace and a king", s)
	}
	if s := hand(t, "Ac Qd Jh 9s 8c").Score(); s >= least {
		t.Error("ace, queen high scores as much as an ace and a king")
	}
}
