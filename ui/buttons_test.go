package ui

import "testing"

func TestStep(t *testing.T) {
	v := []int{1, 5, 10, 25, 100}
	if got := Step(v, v[0], -1); got != v[0] {
		t.Errorf("stepped down from the lowest choice to %d", got)
	}
	if got := Step(v, v[len(v)-1], 1); got != v[len(v)-1] {
		t.Errorf("stepped up from the highest choice to %d", got)
	}
	if got := Step(v, v[1], 1); got != v[2] {
		t.Errorf("stepped up from %d to %d", v[1], got)
	}
}

func TestButtonHit(t *testing.T) {
	b := Button{X: 10, Y: 20, W: 30, H: 40}
	for _, p := range [][2]int{{10, 20}, {39, 59}, {25, 40}} {
		if !b.Hit(p[0], p[1]) {
			t.Errorf("a click at %v missed the button", p)
		}
	}
	for _, p := range [][2]int{{9, 20}, {10, 19}, {40, 40}, {25, 60}} {
		if b.Hit(p[0], p[1]) {
			t.Errorf("a click at %v hit the button", p)
		}
	}
}

func TestChipText(t *testing.T) {
	for amount, want := range map[int]string{
		100: "$1", 500: "$5", 2500: "$25", 99_900: "$999",
		100_000: "1K", 250_000: "2.5K", 500_000: "5K",
	} {
		if got := chipText(amount); got != want {
			t.Errorf("%d credits are written on a chip as %q, want %q", amount, got, want)
		}
	}
}
