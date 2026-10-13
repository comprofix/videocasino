package poker

import (
	"strings"
	"testing"
	"videocasino/cards"
)

// hand reads five cards written as String writes them, such as
// "As Kd 7h 7c 2s".
func hand(s string) (h [HandSize]cards.Card) {
	names := strings.Fields(s)
	if len(names) != HandSize {
		panic("not five cards: " + s)
	}
	for i, name := range names {
		rank := strings.IndexByte("23456789TJQKA", name[0])
		suit := strings.IndexByte("cdhs", name[1])
		if len(name) != 2 || rank < 0 || suit < 0 {
			panic("not a card: " + name)
		}
		h[i] = cards.New(cards.Two+cards.Rank(rank), cards.Suit(suit))
	}
	return h
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		hand string
		want Category
	}{
		{"Ts Js Qs Ks As", RoyalFlush},
		{"Ah Kh Qh Jh Th", RoyalFlush},
		{"9s Ts Js Qs Ks", StraightFlush},
		{"Ad 2d 3d 4d 5d", StraightFlush}, // the ace is low
		{"7c 7d 7h 7s 2c", FourOfAKind},
		{"2c 7d 7h 7s 7c", FourOfAKind},
		{"7c 7d 7h 2s 2c", FullHouse},
		{"7c 7d 2h 2s 2c", FullHouse},
		{"2h 9h Jh Kh Ah", Flush},
		{"2h 3h 4h 5h 7h", Flush},
		{"9s Ts Js Qs Kd", Straight},
		{"Ad 2c 3d 4d 5d", Straight},
		{"Td Jc Qd Kd Ad", Straight},
		{"7c 7d 7h Ks 2c", ThreeOfAKind},
		{"7c 7d Kh Ks 2c", TwoPair},
		{"2c 2d 3h 3s Ac", TwoPair},
		{"Jc Jd 3h 8s Ac", JacksOrBetter},
		{"Qc Qd 3h 8s Ac", JacksOrBetter},
		{"Kc Kd 3h 8s Ac", JacksOrBetter},
		{"Ac Ad 3h 8s Kc", JacksOrBetter},
		{"Tc Td 3h 8s Ac", Nothing}, // a pair too low to pay
		{"2c 2d 3h 8s Ac", Nothing},
		{"2c 4d 6h 8s Tc", Nothing},
		{"Jc Qd Kh As 2c", Nothing}, // a straight cannot run round the ace
		{"Qc Kd Ah 2s 3c", Nothing},
	}
	for _, tt := range tests {
		if got := Evaluate(hand(tt.hand)); got != tt.want {
			t.Errorf("%s makes %v, want %v", tt.hand, got, tt.want)
		}
	}
}

// TestEvaluateOrder checks that the order of the cards makes no difference.
func TestEvaluateOrder(t *testing.T) {
	for _, s := range []string{"Ts Js Qs Ks As", "Ad 2c 3d 4d 5d", "7c 7d 7h 2s 2c", "Jc Jd 3h 8s Ac", "7c 7d Kh Ks 2c"} {
		h := hand(s)
		want := Evaluate(h)
		for range HandSize {
			h = [HandSize]cards.Card{h[1], h[2], h[3], h[4], h[0]}
			if got := Evaluate(h); got != want {
				t.Errorf("%v makes %v, want %v", h, got, want)
			}
			h[0], h[2] = h[2], h[0]
			if got := Evaluate(h); got != want {
				t.Errorf("%v makes %v, want %v", h, got, want)
			}
		}
	}
}

// TestHandCounts checks how many of the 2,598,960 hands make each category
// against the well-known figures.
func TestHandCounts(t *testing.T) {
	want := [NumCategories]int{
		Nothing:       2_062_860,
		JacksOrBetter: 337_920,
		TwoPair:       123_552,
		ThreeOfAKind:  54_912,
		Straight:      10_200,
		Flush:         5_108,
		FullHouse:     3_744,
		FourOfAKind:   624,
		StraightFlush: 36,
		RoyalFlush:    4,
	}
	var got [NumCategories]int
	eachHand(func(h [HandSize]cards.Card) { got[Evaluate(h)]++ })
	for c := range NumCategories {
		if got[c] != want[c] {
			t.Errorf("%v: %d hands, want %d", c, got[c], want[c])
		}
	}
}

func TestPay(t *testing.T) {
	tests := []struct {
		made  Category
		coins int
		want  int
	}{
		{Nothing, 5, 0},
		{JacksOrBetter, 1, 1},
		{JacksOrBetter, 5, 5},
		{FullHouse, 1, 9},
		{FullHouse, 3, 27},
		{Flush, 5, 30},
		{RoyalFlush, 1, 250},
		{RoyalFlush, 4, 1000},
		{RoyalFlush, 5, 4000}, // the royal pays extra at the most coins
	}
	for _, tt := range tests {
		if got := NineSix.Pay(tt.made, tt.coins); got != tt.want {
			t.Errorf("%v for %d coins pays %d, want %d", tt.made, tt.coins, got, tt.want)
		}
	}
}
