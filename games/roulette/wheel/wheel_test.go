package wheel

import (
	"reflect"
	"slices"
	"testing"

	"videocasino/bank"
)

// spot finds the spot of the given kind on the given numbers.
func spot(t *testing.T, k Kind, numbers ...int) int {
	t.Helper()
	for i, s := range Spots {
		if s.Kind == k && slices.Equal(s.Numbers, numbers) {
			return i
		}
	}
	t.Fatalf("there is no spot of kind %d on %v", k, numbers)
	return 0
}

// TestWheel checks the wheel against what is known of a real one: every
// number is on it once, there are as many reds as blacks, and the two take
// turns all the way round from the zero.
func TestWheel(t *testing.T) {
	seen := map[int]bool{}
	for _, n := range Order {
		if n < 0 || n >= Pockets || seen[n] {
			t.Fatalf("the wheel has %d on it wrongly or twice", n)
		}
		seen[n] = true
	}
	if Order[0] != 0 || IsRed(0) || IsBlack(0) {
		t.Error("the zero is not a green pocket at the top of the wheel")
	}
	for i := 1; i < Pockets; i++ {
		if n := Order[i]; IsRed(n) != (i%2 == 1) || IsBlack(n) == IsRed(n) {
			t.Errorf("%d, in pocket %d, is the wrong colour for its place", n, i)
		}
	}
	if len(reds) != (Pockets-1)/2 {
		t.Errorf("there are %d red numbers", len(reds))
	}
	for n := range Pockets {
		if Order[Pocket(n)] != n {
			t.Errorf("%d is not in the pocket it is said to be in", n)
		}
	}
}

// TestSpots checks that the table has every bet a single-zero table has,
// each once.
func TestSpots(t *testing.T) {
	want := [Kinds]int{
		Straight: 37,
		Split:    60, // 57 on the table and 3 with the zero
		Street:   14, // 12 rows and 2 with the zero
		Corner:   23, // 22 on the table and the first four
		SixLine:  11,
		Column:   3,
		Dozen:    3,
		Low:      1, Evens: 1, Reds: 1, Blacks: 1, Odds: 1, High: 1,
	}
	covers := [Kinds]int{
		Straight: 1, Split: 2, Street: 3, Corner: 4, SixLine: 6,
		Column: 12, Dozen: 12,
		Low: 18, Evens: 18, Reds: 18, Blacks: 18, Odds: 18, High: 18,
	}
	var got [Kinds]int
	for i, s := range Spots {
		got[s.Kind]++
		if len(s.Numbers) != covers[s.Kind] || !slices.IsSorted(s.Numbers) {
			t.Errorf("%v covers %v", s, s.Numbers)
		}
		for j, o := range Spots[:i] {
			if slices.Equal(s.Numbers, o.Numbers) {
				t.Errorf("spots %d and %d are both on %v", j, i, s.Numbers)
			}
		}
		if s.String() == "" {
			t.Errorf("spot %d has no name", i)
		}
	}
	if got != want {
		t.Errorf("the table has %v spots of each kind, want %v", got, want)
	}
	for n := range Pockets {
		if s := Spots[n]; s.Kind != Straight || s.Numbers[0] != n {
			t.Errorf("spot %d is %v, not the number itself", n, s)
		}
	}

	// Some bets by name, the ones with the zero among them.
	spot(t, Split, 17, 20)
	spot(t, Split, 17, 18)
	spot(t, Split, 0, 2)
	spot(t, Street, 0, 2, 3)
	spot(t, Corner, 0, 1, 2, 3)
	spot(t, Corner, 32, 33, 35, 36)
	spot(t, SixLine, 31, 32, 33, 34, 35, 36)
	if c := Spots[spot(t, Column, 3, 6, 9, 12, 15, 18, 21, 24, 27, 30, 33, 36)]; c.String() != "COLUMN 3 TO 36" {
		t.Errorf("the top column is called %q", c)
	}
	for _, s := range Spots {
		if s.Kind == Split && s.Numbers[0] != 0 {
			if d := s.Numbers[1] - s.Numbers[0]; d != 1 && d != 3 {
				t.Errorf("%v is not two numbers side by side", s)
			}
		}
	}
}

// TestReturn checks what the game pays back: exactly 36 parts in 37 on
// every bet, whatever is bet on. It is checked twice over: from what each
// bet covers and pays, and by letting every pocket come up once.
func TestReturn(t *testing.T) {
	for i, s := range Spots {
		if len(s.Numbers)*(s.Pays()+1) != Pockets-1 {
			t.Errorf("%v covers %d numbers and pays %d to 1", s, len(s.Numbers), s.Pays())
		}
		bets := make([]int, len(Spots))
		bets[i] = 1
		paid := 0
		for n := range Pockets {
			paid += Paid(bets, n)
		}
		if paid != Pockets-1 {
			t.Errorf("%v pays back %d for %d bet over the whole wheel", s, paid, Pockets)
		}
		if s.Return() != Return() {
			t.Errorf("%v returns %v, not the %v of the game", s, s.Return(), Return())
		}
	}
	if got, want := Return(), 36.0/37; got != want {
		t.Errorf("the game returns %v, want %v", got, want)
	}
	t.Logf("return: %.4f%%", Return()*100)
}

func TestBets(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	const seventeen = 17
	red := slices.IndexFunc(Spots, func(s Spot) bool { return s.Kind == Reds })

	m.Chip = 100
	if !m.Place(seventeen) || !m.Place(seventeen) || !m.Place(red) {
		t.Fatal("could not put three chips down")
	}
	if m.Bets[seventeen] != 200 || m.Total() != 300 {
		t.Errorf("200 on 17 and 100 on red came to %d and %d in all", m.Bets[seventeen], m.Total())
	}
	if b.Credits != bank.StartCredits {
		t.Errorf("the bank was charged %d before the spin", bank.StartCredits-b.Credits)
	}

	// A chip comes off at a time, and the last of a bet whatever is left.
	m.Chip = 500
	if !m.Remove(seventeen) || m.Bets[seventeen] != 0 || m.Remove(seventeen) {
		t.Errorf("taking 500 off a bet of 200 left %d", m.Bets[seventeen])
	}

	// A spot holds no more than the limit.
	m.Chip = 100_000
	b.Credits = 10 * MaxSpot
	for range MaxSpot / 100_000 {
		if !m.Place(seventeen) {
			t.Fatal("could not bet up to the limit")
		}
	}
	if m.Place(seventeen) || m.Bets[seventeen] != MaxSpot {
		t.Errorf("a spot holds %d", m.Bets[seventeen])
	}

	// The table holds no more than the bank has.
	b.Credits = m.Total() + 999
	if m.Place(red) {
		t.Error("bet more than the bank has")
	}
	m.Chip = 500
	if !m.Place(red) {
		t.Error("could not bet what the bank has")
	}

	m.Clear()
	if m.Total() != 0 {
		t.Errorf("a cleared table has %d on it", m.Total())
	}
}

func TestSpin(t *testing.T) {
	b := bank.New()
	m := New(b, 1, 2)
	if _, ok := m.Spin(); ok {
		t.Fatal("spun with nothing bet")
	}
	if m.Rebet() {
		t.Error("bet again before anything had been bet")
	}

	// Bet on everything at once: whatever comes up, the same is paid on the
	// number, and the rest follows from it.
	m.Chip = 10
	for n := range Pockets {
		m.Place(n)
	}
	red := slices.IndexFunc(Spots, func(s Spot) bool { return s.Kind == Reds })
	m.Chip = 100
	m.Place(red)

	seen := map[int]bool{}
	for i := range 400 {
		had := b.Credits
		r, ok := m.Spin()
		if !ok {
			t.Fatal("could not spin")
		}
		seen[r.Number] = true
		want := 10 * 36
		if IsRed(r.Number) {
			want += 200
		}
		if r.Bet != 470 || r.Win != want || b.Credits != had-470+want {
			t.Fatalf("%d came up: bet %d, paid %d, bank from %d to %d", r.Number, r.Bet, r.Win, had, b.Credits)
		}
		if m.Total() != 0 || m.LastTotal() != 470 || m.History[0] != r.Number {
			t.Fatalf("after a spin: %d on the table, %d remembered, history %v", m.Total(), m.LastTotal(), m.History)
		}
		if want := min(i+1, HistoryLen); len(m.History) != want {
			t.Fatalf("%d numbers are remembered after %d spins", len(m.History), i+1)
		}
		if !m.Rebet() || m.Total() != 470 {
			t.Fatal("could not bet the same again")
		}
	}
	if len(seen) != Pockets {
		t.Errorf("only %d of the numbers came up in 400 spins", len(seen))
	}

	// With too little in the bank nothing is spun or taken.
	b.Credits = 469
	if _, ok := m.Spin(); ok || b.Credits != 469 || m.Total() != 470 {
		t.Errorf("spun for 470 with 469: bank %d, table %d", b.Credits, m.Total())
	}
	m.Clear()
	if m.Rebet() {
		t.Error("bet 470 again with 469 in the bank")
	}

	// The same seeds give the same spins.
	x, y := New(bank.New(), 5, 6), New(bank.New(), 5, 6)
	for range 50 {
		x.Place(0)
		y.Place(0)
		rx, _ := x.Spin()
		ry, _ := y.Spin()
		if rx != ry {
			t.Fatalf("the same seeds gave %+v and %+v", rx, ry)
		}
	}
}

func TestSaveAndRestore(t *testing.T) {
	b := bank.New()
	m := New(b, 3, 4)
	m.Chip = 100
	m.Place(17)
	m.Place(40)
	m.Spin()
	m.Rebet()
	m.Chip = 500
	m.Place(100)
	m.Spin()
	m.Place(5)

	again := New(b, 9, 9)
	again.Restore(m.Save())
	if !reflect.DeepEqual(again.Save(), m.Save()) {
		t.Errorf("restored %+v, want %+v", again.Save(), m.Save())
	}
	if again.Total() != 500 || again.LastTotal() != 700 || len(again.History) != 2 {
		t.Errorf("restored %d on the table, %d remembered and %v", again.Total(), again.LastTotal(), again.History)
	}

	// A save that could not have come from play is put right.
	again.Restore(Saved{
		Chip: 7,
		Bets: []Stake{
			{Spot: -1, Amount: 10}, {Spot: len(Spots), Amount: 10},
			{Spot: 1, Amount: -10}, {Spot: 2, Amount: MaxSpot + 10}, {Spot: 3, Amount: 150},
			{Spot: 4, Amount: 200}, {Spot: 4, Amount: 300},
		},
		History: []int{5, -1, 37, 36, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
	})
	if again.Chip != ChipChoices[0] {
		t.Errorf("a chip of 7 was restored as %d", again.Chip)
	}
	if got := stakes(again.Bets); !reflect.DeepEqual(got, []Stake{{4, 200}}) {
		t.Errorf("bad bets were restored as %v", got)
	}
	if again.LastTotal() != 0 {
		t.Errorf("no last bets were restored as %d", again.LastTotal())
	}
	if want := []int{5, 36, 1, 2, 3, 4, 5, 6, 7, 8}; !slices.Equal(again.History, want) {
		t.Errorf("a bad history was restored as %v", again.History)
	}
}
