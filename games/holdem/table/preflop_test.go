package table

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"videocasino/cards"
	"videocasino/pokerhand"
)

// The pre-flop odds are counted over every board there is, which takes
// minutes. They are counted only with -long; -update writes what is counted
// into preflop.go.
var (
	long   = flag.Bool("long", false, "run the slow calculations, which take minutes")
	update = flag.Bool("update", false, "with -long, write the counted figures into preflop.go")
)

// flopIndex returns the place of a flop, its cards in order, among every
// three cards of the deck.
func flopIndex(a, b, c cards.Card) int {
	return int(a) + int(b)*int(b-1)/2 + int(c)*int(c-1)*int(c-2)/6
}

// allFlops is how many flops there are in a whole deck.
const allFlops = cards.DeckSize * (cards.DeckSize - 1) * (cards.DeckSize - 2) / 6

// The flops of a board: every three of its five.
var boardFlops = func() (f [10][FlopSize]int) {
	n := 0
	for i := range BoardSize {
		for j := i + 1; j < BoardSize; j++ {
			for k := j + 1; k < BoardSize; k++ {
				f[n] = [FlopSize]int{i, j, k}
				n++
			}
		}
	}
	return f
}()

// lowerBound returns how many of the scores, which are in order, are less
// than x.
func lowerBound(s []int32, x int32) int {
	lo, hi := 0, len(s)
	for lo < hi {
		mid := (lo + hi) / 2
		if s[mid] < x {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// boardCounter counts one board at a time against every kind of starting
// hand. It holds what it needs from board to board, so it is not shared.
type boardCounter struct {
	t      pokerhand.Table
	reps   [Classes][HoleSize]cards.Card
	rest   [cards.DeckSize - BoardSize]cards.Card
	at     [cards.DeckSize]int // where each card is in rest, or -1
	rank   [cards.DeckSize - BoardSize][cards.DeckSize - BoardSize]int32
	scores []int32 // every hand of two from rest, in order
	sevens [pokerhand.Categories]int
}

func newBoardCounter(t pokerhand.Table) *boardCounter {
	bc := &boardCounter{t: t}
	for c := range Classes {
		bc.reps[c], _ = ClassHand(c)
	}
	return bc
}

// count works out, for a board, what a play bet is worth to each kind of
// starting hand that is not on it: k*a + b halves of an ante over the
// dealer's hands. It reports which kinds it counted. It also counts what
// every hand of seven the board is in makes, each once for each two of the
// seven that could be the player's.
func (bc *boardCounter) count(board [BoardSize]cards.Card, a, b *[Classes]int, ok *[Classes]bool) {
	br := newBoardRanks(bc.t, board)
	for i := range bc.at {
		bc.at[i] = -1
	}
	n := 0
	for c := range cards.Card(cards.DeckSize) {
		if !onBoard(board, c) {
			bc.rest[n], bc.at[c] = c, n
			n++
		}
	}
	bc.scores = bc.scores[:0]
	for i := range n {
		for j := i + 1; j < n; j++ {
			s := br.hole(bc.rest[i], bc.rest[j])
			bc.rank[i][j], bc.rank[j][i] = s, s
			bc.scores = append(bc.scores, s)
			bc.sevens[pokerhand.Score(s).Category()]++
		}
	}
	slices.Sort(bc.scores)

	q := int32(DealerNeeds)
	for c, rep := range bc.reps {
		p1, p2 := bc.at[rep[0]], bc.at[rep[1]]
		if ok[c] = p1 >= 0 && p2 >= 0; !ok[c] {
			continue
		}
		player := bc.rank[p1][p2]
		low, high := min(player, q), max(player+1, q)
		below := lowerBound(bc.scores, player)
		upTo := lowerBound(bc.scores, player+1)
		belowLow := lowerBound(bc.scores, low)
		fromHigh := len(bc.scores) - lowerBound(bc.scores, high)

		// The dealer cannot hold either of the player's cards.
		drop := func(d int32) {
			if d < player {
				below--
			}
			if d <= player {
				upTo--
			}
			if d < low {
				belowLow--
			}
			if d >= high {
				fromHigh--
			}
		}
		for j := range n {
			if j != p1 {
				drop(bc.rank[p1][j])
			}
			if j != p1 && j != p2 {
				drop(bc.rank[p2][j])
			}
		}
		win := below
		lose := dealerHoles - upTo
		a[c], b[c] = tally(player, win, win-belowLow, lose, fromHigh)
	}
}

func onBoard(board [BoardSize]cards.Card, c cards.Card) bool {
	for _, b := range board {
		if b == c {
			return true
		}
	}
	return false
}

// preflopCount is what counting every board gives.
type preflopCount struct {
	preflop [Classes][3]int64
	net     int64 // over every starting hand, in halves of an ante
	staked  int64 // in antes
	raised  int   // starting hands best bet 4 antes before the flop
	sevens  [pokerhand.Categories]int
}

// countPreflop counts every board against every kind of starting hand, and
// from that what each choice before the flop is worth.
func countPreflop(t *testing.T) preflopCount {
	table := pokerhand.NewTable()

	// For each flop and kind of starting hand, added up over every turn and
	// river: a and b, what checking on the flop is worth if the river is
	// then played as well as it can be, and on how many rivers it is bet.
	const fields = 4
	acc := make([]int64, allFlops*Classes*fields)

	type job struct{ d, e cards.Card }
	jobs := make(chan job)
	go func() {
		for e := cards.Card(cards.DeckSize - 1); e >= 4; e-- {
			for d := e - 1; d >= 3; d-- {
				jobs <- job{d, e}
			}
		}
		close(jobs)
	}()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var sevens [pokerhand.Categories]int
	start := time.Now()
	for range runtime.NumCPU() {
		wg.Go(func() {
			bc := newBoardCounter(table)
			var a, b [Classes]int
			var ok [Classes]bool
			for j := range jobs {
				for c := range j.d {
					for bb := range c {
						for aa := range bb {
							board := [BoardSize]cards.Card{aa, bb, c, j.d, j.e}
							bc.count(board, &a, &b, &ok)
							var fl [10]int
							for i, f := range boardFlops {
								fl[i] = flopIndex(board[f[0]], board[f[1]], board[f[2]])
							}
							for k := range Classes {
								if !ok[k] {
									continue
								}
								v := a[k] + b[k]
								m, raised := int64(max(v, foldHalves)), int64(0)
								if v > foldHalves {
									raised = 1
								}
								for _, f := range fl {
									at := (f*Classes + k) * fields
									atomic.AddInt64(&acc[at], int64(a[k]))
									atomic.AddInt64(&acc[at+1], int64(b[k]))
									atomic.AddInt64(&acc[at+2], m)
									atomic.AddInt64(&acc[at+3], raised)
								}
							}
						}
					}
				}
			}
			mu.Lock()
			for i, n := range bc.sevens {
				sevens[i] += n
			}
			mu.Unlock()
		})
	}
	wg.Wait()
	t.Logf("every board counted in %v", time.Since(start).Round(time.Second))

	var pc preflopCount
	for i, n := range sevens {
		// Each hand of seven was counted once for each two of it that could
		// be the player's.
		if n%21 != 0 {
			t.Fatalf("%d hands of seven make %v, which does not divide by 21", n, pokerhand.Category(i))
		}
		pc.sevens[i] = n / 21
	}

	const allBoards = int64(turnRivers) * dealerHoles
	for k := range Classes {
		hole, like := ClassHand(k)
		var sumA, sumB, check, checkStaked int64
		for c := range cards.Card(cards.DeckSize) {
			for bb := range c {
				for aa := range bb {
					if hole[0] == aa || hole[0] == bb || hole[0] == c || hole[1] == aa || hole[1] == bb || hole[1] == c {
						continue
					}
					at := (flopIndex(aa, bb, c)*Classes + k) * fields
					fa, fb, fm, fr := acc[at], acc[at+1], acc[at+2], acc[at+3]
					sumA += fa
					sumB += fb
					// On the flop: bet 2 antes, or check and play the river.
					if bet := 2*fa + fb; bet > fm {
						check += bet
						checkStaked += (2 + 2) * allBoards
					} else {
						check += fm
						checkStaked += dealerHoles * ((2+1)*fr + 2*(turnRivers-fr))
					}
				}
			}
		}
		bet3, bet4 := 3*sumA+sumB, 4*sumA+sumB
		pc.preflop[k] = [3]int64{check, bet3, bet4}

		all := int64(flops) * allBoards
		best, staked := check, checkStaked
		if bet3 > best {
			best, staked = bet3, (2+3)*all
		}
		if bet4 >= best {
			best, staked = bet4, (2+4)*all
			pc.raised += like
		}
		pc.net += int64(like) * best
		pc.staked += int64(like) * staked
	}
	return pc
}

// TestBoardCounts checks the counting that TestPreflop does against
// boardWorth, which counts the dealer's hands in the plain way, for a few
// boards.
func TestBoardCounts(t *testing.T) {
	table := pokerhand.NewTable()
	bc := newBoardCounter(table)
	boards := [][BoardSize]cards.Card{
		{0, 9, 20, 33, 51},
		{48, 49, 50, 51, 44},
		{4, 5, 6, 7, 30},
		{12, 16, 20, 24, 28},
	}
	for _, board := range boards {
		var a, b [Classes]int
		var ok [Classes]bool
		bc.count(board, &a, &b, &ok)
		for k := 0; k < Classes; k += 7 {
			if !ok[k] {
				continue
			}
			hole, _ := ClassHand(k)
			wa, wb := boardWorth(table, hole, board)
			if a[k] != wa || b[k] != wb {
				t.Errorf("%v with %v: counted %d and %d, want %d and %d", hole, board, a[k], b[k], wa, wb)
			}
		}
	}
}

// TestPreflop counts every board there is and checks the figures kept in
// preflop.go, which the game uses. It runs only with -long; -update writes
// what it counts into preflop.go.
func TestPreflop(t *testing.T) {
	if !*long {
		t.Skip("takes minutes: run with -long -timeout 0")
	}
	pc := countPreflop(t)

	edge := -float64(pc.net) / (2 * float64(returnHands))
	ret := float64(2*pc.staked+pc.net) / float64(2*pc.staked)
	t.Logf("edge %.4f%% of the ante, return %.4f%%, %.2f%% of hands bet 4 antes before the flop",
		100*edge, 100*ret, 100*float64(pc.raised)/1326)
	t.Logf("hands of seven: %v", pc.sevens)

	// The hands of seven are the figures any book on poker gives.
	want := [pokerhand.Categories]int{
		pokerhand.HighCard:      23_294_460,
		pokerhand.Pair:          58_627_800,
		pokerhand.TwoPair:       31_433_400,
		pokerhand.ThreeOfAKind:  6_461_620,
		pokerhand.Straight:      6_180_020,
		pokerhand.Flush:         4_047_644,
		pokerhand.FullHouse:     3_473_184,
		pokerhand.FourOfAKind:   224_848,
		pokerhand.StraightFlush: 37_260,
		pokerhand.RoyalFlush:    4_324,
	}
	if pc.sevens != want {
		t.Errorf("hands of seven make %v, want %v", pc.sevens, want)
	}

	if *update {
		writePreflop(t, pc)
		return
	}
	if pc.preflop != preflop || pc.net != returnNet || pc.staked != returnStaked || pc.raised != raisedPreflop || pc.sevens != sevens {
		t.Error("the figures kept in preflop.go are not the ones counted: run with -long -update to write them")
	}
}

// writePreflop writes what was counted into preflop.go.
func writePreflop(t *testing.T, pc preflopCount) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by TestPreflop with -long -update. DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package table\n\nimport \"videocasino/pokerhand\"\n\n")
	fmt.Fprintf(&b, "// What TestPreflop counted, kept so the game need not count it again:\n")
	fmt.Fprintf(&b, "// what the player wins over every hand, in halves of an ante; what is\n")
	fmt.Fprintf(&b, "// staked over them, in antes; and how many starting hands are best bet 4\n")
	fmt.Fprintf(&b, "// antes before the flop.\n")
	fmt.Fprintf(&b, "const (\n\treturnNet = %d\n\treturnStaked = %d\n\traisedPreflop = %d\n)\n\n", pc.net, pc.staked, pc.raised)
	fmt.Fprintf(&b, "// sevens is how many hands of seven make each category.\n")
	fmt.Fprintf(&b, "var sevens = [pokerhand.Categories]int{")
	for _, n := range pc.sevens {
		fmt.Fprintf(&b, "%d, ", n)
	}
	fmt.Fprintf(&b, "}\n\n")
	fmt.Fprintf(&b, "// preflop holds, for each kind of starting hand, what checking, betting 3\n")
	fmt.Fprintf(&b, "// antes and betting 4 antes are worth, out of preflopDen.\n")
	fmt.Fprintf(&b, "var preflop = [Classes][3]int64{\n")
	for k, p := range pc.preflop {
		hole, _ := ClassHand(k)
		fmt.Fprintf(&b, "\t{%d, %d, %d}, // %v %v\n", p[0], p[1], p[2], hole[0], hole[1])
	}
	fmt.Fprintf(&b, "}\n")
	src, err := format.Source(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("preflop.go", src, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Log("wrote preflop.go")
}
