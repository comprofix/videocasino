package table

import (
	"flag"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"videocasino/cards"
	"videocasino/pokerhand"
)

// The return of the game is counted over every deal there is, which takes
// most of a minute. It is counted only with -long; the totals it gives are
// kept in odds.go.
var long = flag.Bool("long", false, "run the slow calculations, which take most of a minute")

// binom holds how many ways k things can be taken from n, for the numbers
// a hand of five needs.
var binom = func() (b [cards.DeckSize + 1][HandSize + 1]int) {
	for n := range b {
		b[n][0] = 1
		for k := 1; k <= HandSize && k <= n; k++ {
			b[n][k] = b[n-1][k-1] + b[n-1][k]
		}
	}
	return b
}()

// scoreTable returns the score of every hand of five, each at its own
// place: a hand of cards a < b < c < d < e is at
// C(a,1) + C(b,2) + C(c,3) + C(d,4) + C(e,5).
func scoreTable() []int32 {
	scores := make([]int32, binom[cards.DeckSize][HandSize])
	for e := range cards.Card(cards.DeckSize) {
		for d := range e {
			for c := range d {
				for b := range c {
					for a := range b {
						i := binom[a][1] + binom[b][2] + binom[c][3] + binom[d][4] + binom[e][5]
						scores[i] = int32(Hand{a, b, c, d, e}.Score())
					}
				}
			}
		}
	}
	return scores
}

// raiseTotals does for every card the dealer could show at once what
// raiseTotal does for one: it returns the cards left once the player's are
// dealt, and for each of them face up what raising wins or loses, in antes,
// over every hand the dealer can hold with it. Each hand of the dealer's is
// looked at only once, and counted for each of its cards.
func raiseTotals(scores []int32, h Hand) (rest [upCards]cards.Card, net [upCards]int) {
	var out [cards.DeckSize]bool
	for _, c := range h {
		out[c] = true
	}
	n := 0
	for c := range cards.Card(cards.DeckSize) {
		if !out[c] {
			rest[n] = c
			n++
		}
	}

	player := int32(h.Score())
	qualifies := int32(DealerNeeds)
	win := 1 + RaiseSize*RaisePays[h.Category()]
	const lose = -(1 + RaiseSize)

	// The dealer's cards are taken from the highest down, so the last is
	// taken from the lowest up and its scores lie side by side.
	for i4 := 4; i4 < upCards; i4++ {
		at4, sum4 := binom[rest[i4]][5], 0
		for i3 := 3; i3 < i4; i3++ {
			at3, sum3 := at4+binom[rest[i3]][4], 0
			for i2 := 2; i2 < i3; i2++ {
				at2, sum2 := at3+binom[rest[i2]][3], 0
				for i1 := 1; i1 < i2; i1++ {
					at1, sum1 := at2+binom[rest[i1]][2], 0
					for i0 := range i1 {
						var v int
						switch dealer := scores[at1+int(rest[i0])]; {
						case dealer < qualifies:
							v = 1
						case player > dealer:
							v = win
						case player == dealer:
							v = 0
						default:
							v = lose
						}
						net[i0] += v
						sum1 += v
					}
					net[i1] += sum1
					sum2 += sum1
				}
				net[i2] += sum2
				sum3 += sum2
			}
			net[i3] += sum3
			sum4 += sum3
		}
		net[i4] += sum4
	}
	return rest, net
}

// classes returns one hand for each kind the player can be dealt, with how
// many hands there are of its kind. Hands that differ only by their suits,
// as a pair of aces in clubs and diamonds and one in hearts and spades do,
// fare the same in every way, so each kind need be counted only once.
func classes() (reps []Hand, weights []int) {
	// Every way of changing the suits round.
	var perms [][cards.NumSuits]cards.Suit
	for a := range cards.Suit(cards.NumSuits) {
		for b := range cards.Suit(cards.NumSuits) {
			for c := range cards.Suit(cards.NumSuits) {
				d := 6 - a - b - c
				if a != b && a != c && b != c {
					perms = append(perms, [cards.NumSuits]cards.Suit{a, b, c, d})
				}
			}
		}
	}

	// A hand's kind is known by the least of what it becomes under each.
	key := func(h Hand, p [cards.NumSuits]cards.Suit) uint64 {
		var m Hand
		for i, c := range h {
			m[i] = cards.New(c.Rank(), p[c.Suit()])
			for j := i; j > 0 && m[j] < m[j-1]; j-- {
				m[j], m[j-1] = m[j-1], m[j]
			}
		}
		var k uint64
		for _, c := range m {
			k = k<<6 | uint64(c)
		}
		return k
	}
	seen := make(map[uint64]int)
	for e := range cards.Card(cards.DeckSize) {
		for d := range e {
			for c := range d {
				for b := range c {
					for a := range b {
						h := Hand{a, b, c, d, e}
						least := ^uint64(0)
						for _, p := range perms {
							least = min(least, key(h, p))
						}
						at, ok := seen[least]
						if !ok {
							at = len(reps)
							seen[least] = at
							reps = append(reps, h)
							weights = append(weights, 0)
						}
						weights[at]++
					}
				}
			}
		}
	}
	return reps, weights
}

// totals is what counting every deal gives.
type totals struct {
	net, staked, raised int

	// What a player who followed the rule of thumb on the help screen
	// would get wrong: a pair or better folded, or a hand less than an ace
	// and a king raised. Each is counted for every card the dealer could
	// show.
	pairsFolded, weakRaised int
}

func (t *totals) add(u totals) {
	t.net += u.net
	t.staked += u.staked
	t.raised += u.raised
	t.pairsFolded += u.pairsFolded
	t.weakRaised += u.weakRaised
}

// count adds up a hand, of which there are so many like it, against every
// card the dealer could show.
func count(scores []int32, h Hand, like int) (t totals) {
	_, net := raiseTotals(scores, h)
	player := h.Score()
	for _, n := range net {
		if n > -dealerHands {
			t.net += like * n
			t.staked += like * (1 + RaiseSize) * dealerHands
			t.raised += like
			if player < DealerNeeds {
				t.weakRaised += like
			}
		} else {
			t.net -= like * dealerHands
			t.staked += like * dealerHands
			if player.Category() >= pokerhand.Pair {
				t.pairsFolded += like
			}
		}
	}
	return t
}

// TestRaiseTotals checks the counting that TestReturn does against Advise,
// which counts the dealer's hands for each face-up card in the plain way,
// for a few hands of every sort.
func TestRaiseTotals(t *testing.T) {
	scores := scoreTable()
	for _, h := range []Hand{
		hand(t, "Ac Kd 9h 6s 2c"),
		hand(t, "As Kh Qd 3c 2s"),
		hand(t, "7c 7d Ah 9s 4c"),
		hand(t, "8h 6d 5c 4s 2h"),
		hand(t, "Th Jh Qh Kh Ah"),
	} {
		rest, net := raiseTotals(scores, h)
		for i, up := range rest {
			if i%6 != 0 {
				continue // a few of the cards the dealer could show are enough
			}
			if want := raiseTotal(h, up); net[i] != want {
				t.Errorf("%v with %v up: counted %d, want %d", h, up, net[i], want)
			}
		}
	}
}

// TestClasses checks that the kinds of hand cover every hand once.
func TestClasses(t *testing.T) {
	if !*long {
		t.Skip("takes a few seconds: run with -long")
	}
	reps, weights := classes()
	sum := 0
	for _, w := range weights {
		sum += w
	}
	if sum != playerHands || len(reps) != 134_459 {
		t.Errorf("%d kinds of hand cover %d hands, want 134,459 covering %d", len(reps), sum, playerHands)
	}
}

// TestReturn counts every deal there is and checks the totals kept in
// odds.go, which the game shows. It takes most of a minute, so it runs only
// with -long; -timeout 0 lifts the ten minutes a test run is allowed, for a
// slower machine.
func TestReturn(t *testing.T) {
	if !*long {
		t.Skip("takes most of a minute: run with -long -timeout 0")
	}
	start := time.Now()
	scores := scoreTable()
	reps, weights := classes()
	t.Logf("%d kinds of hand, ready in %v", len(reps), time.Since(start).Round(time.Second))

	var (
		next int64 = -1
		mu   sync.Mutex
		all  totals
		wg   sync.WaitGroup
	)
	for range runtime.NumCPU() {
		wg.Go(func() {
			var mine totals
			for {
				i := int(atomic.AddInt64(&next, 1))
				if i >= len(reps) {
					break
				}
				mine.add(count(scores, reps[i], weights[i]))
			}
			mu.Lock()
			all.add(mine)
			mu.Unlock()
		})
	}
	wg.Wait()
	t.Logf("counted in %v", time.Since(start).Round(time.Second))

	t.Logf("returnNet    = %d", all.net)
	t.Logf("returnStaked = %d", all.staked)
	t.Logf("raisedHands  = %d", all.raised)
	t.Logf("edge %.4f%% of the ante, return %.4f%%, %.2f%% raised",
		-100*float64(all.net)/deals,
		100*float64(all.staked+all.net)/float64(all.staked),
		100*float64(all.raised)/(playerHands*upCards))

	if all.net != returnNet || all.staked != returnStaked || all.raised != raisedHands {
		t.Errorf("the totals kept in odds.go are %d, %d and %d; put in the ones above",
			returnNet, returnStaked, raisedHands)
	}
	// The rule of thumb on the help screen holds for every hand.
	if all.pairsFolded != 0 || all.weakRaised != 0 {
		t.Errorf("the rule of thumb is wrong: %d pairs or better are best folded, and %d hands less than an ace and a king best raised",
			all.pairsFolded, all.weakRaised)
	}
}
