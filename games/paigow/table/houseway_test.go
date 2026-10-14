package table

import (
	"slices"
	"testing"

	"videocasino/cards"
)

// TestHouseWay checks the house way, rule by rule, on hands chosen to show
// each one: what goes low.
func TestHouseWay(t *testing.T) {
	tests := []struct {
		rule, hand, low string
	}{
		{"1 no pair", "As Kd 9h 7c 5s 3d 2h", "Kd 9h"},
		{"1 no pair, the joker as an ace", "Jk Kd 9h 7c 5s 3d 2h", "Kd 9h"},
		{"2 one pair", "9s 9d Ah Kc 7s 4d 2h", "Ah Kc"},
		{"2 one pair, aces with the joker", "Jk Ad 9h 7c 5s 3d 2h", "9h 7c"},

		{"3 two pairs with aces, split", "As Ad 5h 5c Kc 8d 2h", "5h 5c"},
		{"3 a high pair, split even with an ace", "Ks Kd 4h 4c Ac 8d 2h", "4h 4c"},
		{"3 a middle pair and an ace, kept", "9s 9d 4h 4c Ac 8d 2h", "Ac 8d"},
		{"3 a middle pair and no ace, split", "9s 9d 4h 4c Kc 8d 2h", "4h 4c"},
		{"3 low pairs and a king, kept", "6s 6d 4h 4c Kc 8d 2h", "Kc 8d"},
		{"3 low pairs and no ace or king, split", "6s 6d 4h 4c Qc 8d 2h", "4h 4c"},

		{"4 three pairs", "9s 9d 5h 5c 3c 3d Ah", "9s 9d"},

		{"5 three of a kind", "8s 8d 8h Kc 9d 4c 2h", "Kc 9d"},
		{"5 three aces", "As Ad Ah Kc 9d 4c 2h", "As Kc"},
		{"5 two threes of a kind", "8s 8d 8h 5c 5d 5h Ah", "8s 8d"},

		{"6 a six-card straight, the best low it leaves", "Ts 9d 8h 7c 6d 5h 2c", "Ts 2c"},
		{"6 a flush before a straight", "Kh 8h 6h 5h 4h 7c 3d", "7c 3d"},
		{"6 a straight and a pair that can go low", "Ks Kd 9h 8c 7d 6h 5c", "Ks Kd"},
		{"6 a straight and a pair that cannot", "9s 9d 8h 7c 6d 5h Ac", "Ac 8h"},
		{"6 a straight and three of a kind", "7s 7d 7h 8c 6d 5h 4c", "7s 7d"},
		{"6 the joker in a straight", "Jk 9d 8h 7c 6s Kd 2h", "Kd 2h"},

		{"7 a full house, split", "Ks Kd Kh 4c 4d 9h 2c", "4c 4d"},
		{"7 a full house and a pair", "9s 9d 9h 5c 5d 3h 3c", "5c 5d"},

		{"8 four 5s, kept", "5s 5d 5h 5c Kd 9h 2c", "Kd 9h"},
		{"8 four 5s, kept with small cards", "5s 5d 5h 5c 4d 3h 2c", "4d 3h"},
		{"8 four 9s and no ace or king, split", "9s 9d 9h 9c Qd 8h 2c", "9s 9d"},
		{"8 four 9s and a king, kept", "9s 9d 9h 9c Kd 8h 2c", "Kd 8h"},
		{"8 four queens and a king, split", "Qs Qd Qh Qc Kd 8h 2c", "Qs Qd"},
		{"8 four queens and an ace, kept", "Qs Qd Qh Qc Ad 8h 2c", "Ad 8h"},
		{"8 four aces, split", "As Ad Ah Ac Kd 8h 2c", "As Ad"},
		{"8 four aces and 8s, kept", "As Ad Ah Ac 8d 8h 2c", "8d 8h"},
		{"8 four aces and 5s, split", "As Ad Ah Ac 5d 5h 2c", "As Ad"},
		{"8 four of a kind and a pair", "9s 9d 9h 9c 5d 5h Kc", "5d 5h"},

		{"9 five aces and kings", "As Ad Ah Ac Jk Kd Kh", "Kd Kh"},
		{"9 five aces alone", "As Ad Ah Ac Jk 9d 2h", "As Ad"},
	}
	var r Ranker
	for _, tt := range tests {
		hand := hand7(t, tt.hand)
		low := r.HouseWay(hand)
		got := []cards.Card{hand[low[0]], hand[low[1]]}
		want := deck(t, tt.low)
		if !sameRanks(got, want) {
			t.Errorf("rule %s: %s sets %s %s low, want %s", tt.rule, tt.hand, Name(got[0]), Name(got[1]), tt.low)
		}
		if _, ok := r.Set(hand, low); !ok {
			t.Errorf("rule %s: %s is set to foul", tt.rule, tt.hand)
		}
	}
}

// sameRanks reports whether two low hands hold the same ranks: which of
// two cards of a rank goes low does not matter.
func sameRanks(a, b []cards.Card) bool {
	ra := []cards.Rank{rankOf(a[0]), rankOf(a[1])}
	rb := []cards.Rank{rankOf(b[0]), rankOf(b[1])}
	slices.Sort(ra)
	slices.Sort(rb)
	return slices.Equal(ra, rb)
}
