package pokerhand

import (
	"slices"
	"strings"
	"testing"

	"videocasino/cards"
)

// several makes cards from names such as "As Kd 4c".
func several(t *testing.T, names string) []cards.Card {
	t.Helper()
	var cs []cards.Card
	for _, f := range strings.Fields(names) {
		r := strings.IndexByte("23456789TJQKA", f[0])
		s := strings.IndexByte("cdhs", f[1])
		if len(f) != 2 || r < 0 || s < 0 {
			t.Fatalf("%q is not a card", f)
		}
		cs = append(cs, cards.New(cards.Two+cards.Rank(r), cards.Suit(s)))
	}
	return cs
}

func TestBest(t *testing.T) {
	tests := []struct {
		cards, best, name string
	}{
		{"Ah Qh As Kd 7c 2h Qc", "Ah Qh As Kd Qc", "TWO PAIR, ACES AND QUEENS"},
		{"9s 9d As Kd 7c 2h Qc", "9s 9d As Kd Qc", "PAIR OF NINES"},
		{"Ah 2h 3h 4h 5h 6h 7h", "3h 4h 5h 6h 7h", "STRAIGHT FLUSH TO THE SEVEN"},
		{"Ac Ad Ah Ks Kc Kd 2c", "Ac Ad Ah Ks Kc", "ACES FULL OF KINGS"},
		{"2c 3d 4h 5s 9c Jd Ah", "2c 3d 4h 5s Ah", "STRAIGHT TO THE FIVE"},
		{"Ts Js Qs Ks As 9s 8s", "Ts Js Qs Ks As", "ROYAL FLUSH"},
		{"7c 7d 7h 7s 2c 2d Kh", "7c 7d 7h 7s Kh", "FOUR SEVENS"},
		{"2c 4d 6h 8s Tc Qd Ah", "8s Tc Qd Ah 6h", "ACE QUEEN HIGH"},
	}
	table := NewTable()
	for _, tt := range tests {
		cs := several(t, tt.cards)
		best, s := Best(cs)
		want := several(t, tt.best)
		got := best[:]
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) || s.String() != tt.name {
			t.Errorf("the best of %s is %v, %q; want %s, %q", tt.cards, best, s, tt.best, tt.name)
		}
		if ts := table.Best(cs); ts != s {
			t.Errorf("the table makes the best of %s %v, not %v", tt.cards, ts, s)
		}
		if five := Best5(t, best); table.Score(five) != s {
			t.Errorf("the table scores %v as %v, not %v", five, table.Score(five), s)
		}
	}
}

// Best5 returns the five cards as a hand, for the table to score.
func Best5(t *testing.T, h Hand) Hand {
	t.Helper()
	slices.Reverse(h[:]) // any order will do
	return h
}

// TestTable checks every place in the table against the hand there.
func TestTable(t *testing.T) {
	table := NewTable()
	if len(table) != 2_598_960 {
		t.Fatalf("the table holds %d hands", len(table))
	}
	seen := 0
	for e := range cards.Card(cards.DeckSize) {
		for d := range e {
			for c := range d {
				for b := range c {
					for a := range b {
						if a%7 != 0 {
							continue // a share of them is enough
						}
						h := Hand{a, b, c, d, e}
						if Score(table[Index(h)]) != h.Score() {
							t.Fatalf("%v is at %d with the score of another hand", h, Index(h))
						}
						seen++
					}
				}
			}
		}
	}
	if seen == 0 {
		t.Error("no hands were checked")
	}
}
