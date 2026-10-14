package shuffle

import "testing"

func TestFalls(t *testing.T) {
	// The same number of cards fall in every riffle, and none before the
	// shuffle or after it.
	for _, l := range []Length{Full, Short} {
		falls := make([]int, l)
		for tick := -10; tick < l.Ticks()+10; tick++ {
			if !l.Falls(tick) {
				continue
			}
			if tick < 0 || tick >= l.Ticks() {
				t.Fatalf("a card falls at tick %d of a shuffle of %d", tick, l.Ticks())
			}
			falls[tick/riffleTicks]++
		}
		for i, n := range falls {
			if n != riffleCards {
				t.Errorf("%d cards fall in riffle %d of %d, want %d", n, i+1, l, riffleCards)
			}
		}
	}
}

func TestDone(t *testing.T) {
	for _, l := range []Length{Full, Short} {
		n := l.Ticks()
		if l.Done(-5) != 0 || l.Done(0) != 0 || l.Done(n/2) != 0.5 || l.Done(n) != 1 || l.Done(n+50) != 1 {
			t.Errorf("a shuffle of %d is %v, %v, %v, %v and %v done", n, l.Done(-5), l.Done(0), l.Done(n/2), l.Done(n), l.Done(n+50))
		}
	}
}

// TestShort checks that the short shuffle takes half as long as the full.
func TestShort(t *testing.T) {
	if 2*Short.Ticks() != Full.Ticks() {
		t.Errorf("the short shuffle takes %d ticks and the full %d", Short.Ticks(), Full.Ticks())
	}
}
