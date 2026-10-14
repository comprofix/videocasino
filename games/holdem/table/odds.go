package table

import (
	"slices"

	"videocasino/cards"
	"videocasino/pokerhand"
)

// The odds of the game, worked out from the rules rather than by playing.
//
// Everything is counted in halves of an ante, as the blind pays 3 to 2 on a
// flush. Once the board is all turned, a play bet of k antes is worth
// k*a + b halves over the dealer's 990 hands, for the a and b that
// boardWorth gives: the play bet goes up and down with the hands, while the
// ante and the blind do not depend on its size.

// How many ways the rest of a hand can come, once the player's two cards
// are known: the flop from 50 cards, the turn and river from 47, and the
// dealer's two from 45.
const (
	flops       = 50 * 49 * 48 / 6
	turnRivers  = 47 * 46 / 2
	dealerHoles = 45 * 44 / 2

	// foldHalves is what folding loses over the dealer's hands: the ante
	// and the blind, in halves.
	foldHalves = -2 * 2 * dealerHoles
)

// boardRanks ranks hands of two cards with a full board quickly: the best
// five of the seven is the board itself, four of it with one of the two,
// or three of it with both.
type boardRanks struct {
	t       pokerhand.Table
	base    int32                    // the board alone
	one     [cards.DeckSize]int32    // the best with one card added
	triples [10][FlopSize]cards.Card // every three of the board, in order
}

func newBoardRanks(t pokerhand.Table, board [BoardSize]cards.Card) *boardRanks {
	slices.Sort(board[:])
	br := &boardRanks{t: t, base: t[pokerhand.Index(board)]}
	n := 0
	for i := range BoardSize {
		for j := i + 1; j < BoardSize; j++ {
			for k := j + 1; k < BoardSize; k++ {
				br.triples[n] = [FlopSize]cards.Card{board[i], board[j], board[k]}
				n++
			}
		}
	}
	for c := range cards.Card(cards.DeckSize) {
		if slices.Contains(board[:], c) {
			continue
		}
		best := br.base
		for leave := range BoardSize {
			var h pokerhand.Hand
			n := 0
			for i, b := range board {
				if i != leave {
					h[n] = b
					n++
				}
			}
			h[4] = c
			best = max(best, t[pokerhand.Index(sorted(h))])
		}
		br.one[c] = best
	}
	return br
}

// sorted puts a hand in order, lowest first.
func sorted(h pokerhand.Hand) pokerhand.Hand {
	for i := 1; i < len(h); i++ {
		for j := i; j > 0 && h[j] < h[j-1]; j-- {
			h[j], h[j-1] = h[j-1], h[j]
		}
	}
	return h
}

// hole returns the score of the best five of the board and two cards,
// which must not be on it.
func (br *boardRanks) hole(x, y cards.Card) int32 {
	best := max(br.base, br.one[x], br.one[y])
	for _, tr := range br.triples {
		h := sorted(pokerhand.Hand{tr[0], tr[1], tr[2], x, y})
		best = max(best, br.t[pokerhand.Index(h)])
	}
	return best
}

// worth returns, for a player's score, what a play bet is worth over the
// given scores of the dealer's hands: a play bet of k antes wins k*a + b
// halves of an ante in all.
func worth(player int32, dealer []int32) (a, b int) {
	var win, winQ, lose, loseQ int
	q := int32(DealerNeeds)
	for _, d := range dealer {
		switch {
		case d < player:
			win++
			if d >= q {
				winQ++
			}
		case d > player:
			lose++
			if d >= q {
				loseQ++
			}
		}
	}
	return tally(player, win, winQ, lose, loseQ)
}

// tally turns the count of the dealer's hands the player beats and loses
// to, and of those where the dealer qualifies, into what a play bet is
// worth: k*a + b halves of an ante for a play bet of k antes.
//
// A win pays the play bet 1 to 1, the ante 1 to 1 if the dealer qualifies,
// and the blind by BlindPays. A loss loses the play bet and the blind, and
// the ante if the dealer qualifies.
func tally(player int32, win, winQ, lose, loseQ int) (a, b int) {
	a = 2 * (win - lose)
	b = 2*winQ - 2*loseQ - 2*lose + BlindPays[pokerhand.Score(player).Category()]*win
	return a, b
}

// boardWorth returns what a play bet is worth once the board is all
// turned: k*a + b halves of an ante over the dealer's 990 hands, for a play
// bet of k antes.
func boardWorth(t pokerhand.Table, hole [HoleSize]cards.Card, board [BoardSize]cards.Card) (a, b int) {
	br := newBoardRanks(t, board)
	player := br.hole(hole[0], hole[1])
	out := make(map[cards.Card]bool, HoleSize+BoardSize)
	for _, c := range seven(hole, board) {
		out[c] = true
	}
	var rest []cards.Card
	for c := range cards.Card(cards.DeckSize) {
		if !out[c] {
			rest = append(rest, c)
		}
	}
	dealer := make([]int32, 0, dealerHoles)
	for i, x := range rest {
		for _, y := range rest[i+1:] {
			dealer = append(dealer, br.hole(x, y))
		}
	}
	return worth(player, dealer)
}

// Choice is one of the things the player can do, and what it is worth.
type Choice struct {
	// Antes is the play bet the choice makes, or 0 for a check, or on the
	// river a fold.
	Antes int
	// Worth is what the player wins or loses on average with it, for each
	// credit of the ante, on the ante, the blind and the play bet. Trips is
	// not counted: it pays on the cards, whatever the player does.
	Worth float64
}

// Advice is what each choice open to the player is worth, with the best of
// them.
type Advice struct {
	Choices []Choice
	Best    int // the place of the best in Choices
}

// best marks the choice worth the most. Of two worth the same the first is
// taken.
func (a *Advice) best() {
	for i, c := range a.Choices {
		if c.Worth > a.Choices[a.Best].Worth {
			a.Best = i
		}
	}
}

// Odds weighs up the choices of a hand. Before the flop it looks up what
// was counted beforehand; after it, it counts on the spot, which needs the
// score of every hand of five to hand.
type Odds struct {
	t pokerhand.Table
}

// NewOdds makes the score table the odds count with. It takes a moment.
func NewOdds() *Odds {
	return &Odds{t: pokerhand.NewTable()}
}

// Advise weighs up the choices open in the given round, with the player's
// cards and the board as far as it is turned. It is exact: every way the
// rest of the board and the dealer's cards can come is counted.
func (o *Odds) Advise(r Round, hole [HoleSize]cards.Card, board [BoardSize]cards.Card) Advice {
	var a Advice
	switch r {
	case Preflop:
		p := preflop[Class(hole)]
		a.Choices = []Choice{
			{0, float64(p[0]) / preflopDen},
			{4, float64(p[2]) / preflopDen},
			{3, float64(p[1]) / preflopDen},
		}
	case Flop:
		bet, check := o.flopWorth(hole, [FlopSize]cards.Card(board[:FlopSize]))
		const den = 2 * dealerHoles * turnRivers
		a.Choices = []Choice{{0, float64(check) / den}, {2, float64(bet) / den}}
	case River:
		pa, pb := boardWorth(o.t, hole, board)
		const den = 2 * dealerHoles
		a.Choices = []Choice{{0, float64(foldHalves) / den}, {1, float64(pa+pb) / den}}
	}
	a.best()
	return a
}

// flopWorth returns what betting 2 antes on the flop is worth, and what
// checking is worth if the river is then played as well as it can be, in
// halves of an ante over every turn, river and hand of the dealer's.
func (o *Odds) flopWorth(hole [HoleSize]cards.Card, flop [FlopSize]cards.Card) (bet, check int) {
	var rest []cards.Card
	for c := range cards.Card(cards.DeckSize) {
		if !slices.Contains(hole[:], c) && !slices.Contains(flop[:], c) {
			rest = append(rest, c)
		}
	}
	board := [BoardSize]cards.Card{flop[0], flop[1], flop[2]}
	for i, turn := range rest {
		for _, river := range rest[i+1:] {
			board[3], board[4] = turn, river
			a, b := boardWorth(o.t, hole, board)
			bet += 2*a + b
			check += max(a+b, foldHalves)
		}
	}
	return bet, check
}

// Classes is how many kinds of starting hand there are. Two cards fare
// alike if they differ only by their suits, so all that counts is their
// ranks and whether they are suited: 13 pairs, and 78 pairs of ranks each
// suited or not.
const Classes = cards.NumRanks * cards.NumRanks

// Class returns the kind of a starting hand, from 0 to Classes-1: in a
// square of the ranks, a pair lies on the diagonal, a suited hand below it
// and one not suited above it.
func Class(hole [HoleSize]cards.Card) int {
	hi, lo := int(hole[0].Rank()-cards.Two), int(hole[1].Rank()-cards.Two)
	if hi < lo {
		hi, lo = lo, hi
	}
	if hole[0].Suit() == hole[1].Suit() {
		return hi*cards.NumRanks + lo
	}
	return lo*cards.NumRanks + hi
}

// ClassHand returns a starting hand of the given kind, and how many
// starting hands there are of that kind.
func ClassHand(class int) (hole [HoleSize]cards.Card, like int) {
	row, col := cards.Rank(class/cards.NumRanks)+cards.Two, cards.Rank(class%cards.NumRanks)+cards.Two
	switch {
	case row == col:
		return [HoleSize]cards.Card{cards.New(row, cards.Clubs), cards.New(row, cards.Diamonds)}, 6
	case row > col:
		return [HoleSize]cards.Card{cards.New(row, cards.Clubs), cards.New(col, cards.Clubs)}, 4
	}
	return [HoleSize]cards.Card{cards.New(col, cards.Clubs), cards.New(row, cards.Diamonds)}, 12
}

// preflopDen is what the counts kept for each starting hand are out of:
// halves of an ante, over every flop, turn, river and hand of the dealer's.
const preflopDen = 2.0 * flops * turnRivers * dealerHoles

// returnHands is how many ways a hand can be dealt in all: every starting
// hand, flop, turn, river and hand of the dealer's.
const returnHands = cards.DeckSize * (cards.DeckSize - 1) / 2 * flops * turnRivers * dealerHoles

// Edge returns what the player loses on average for each credit of the
// ante, on the ante, the blind and the play bet, played perfectly.
func Edge() float64 {
	return -float64(returnNet) / (2 * float64(returnHands))
}

// Return returns the fraction of what is staked on the ante, the blind and
// the play bet that is paid back over a long run, played perfectly.
func Return() float64 {
	staked := returnStaked
	return float64(2*staked+returnNet) / float64(2*staked)
}

// RaisedPreflop returns the fraction of starting hands worth betting 4
// antes on before the flop.
func RaisedPreflop() float64 {
	return float64(raisedPreflop) / (cards.DeckSize * (cards.DeckSize - 1) / 2)
}

// TripsReturn returns the fraction of what is bet on Trips that it pays
// back.
func TripsReturn() float64 {
	var paid, all int
	for c, n := range sevens {
		all += n
		if pays := TripsPays[c]; pays > 0 {
			paid += n * (pays + 1)
		}
	}
	return float64(paid) / float64(max(all, 1))
}
