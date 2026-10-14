package shuffle

import "testing"

func TestFalls(t *testing.T) {
	// The same number of cards fall in every riffle, and none before the
	// shuffle or after it.
	var falls [riffles]int
	for tick := -10; tick < Ticks+10; tick++ {
		if !Falls(tick) {
			continue
		}
		if tick < 0 || tick >= Ticks {
			t.Fatalf("a card falls at tick %d of a shuffle of %d", tick, Ticks)
		}
		falls[tick/riffleTicks]++
	}
	for i, n := range falls {
		if n != riffleCards {
			t.Errorf("%d cards fall in riffle %d, want %d", n, i+1, riffleCards)
		}
	}
}

func TestDone(t *testing.T) {
	if Done(-5) != 0 || Done(0) != 0 || Done(Ticks/2) != 0.5 || Done(Ticks) != 1 || Done(Ticks+50) != 1 {
		t.Errorf("the shuffle is %v, %v, %v, %v and %v done", Done(-5), Done(0), Done(Ticks/2), Done(Ticks), Done(Ticks+50))
	}
}
