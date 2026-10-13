package machine

// Free games. Three or more dice win six of them. In a free game reels 2, 3
// and 4 turn as one and show a single giant symbol, three reels wide and
// three rows high, which counts as that symbol in each of its nine places.
// Reels 1 and 5 turn on strips of their own.
//
// A giant dice pays a prize and adds more free games. A giant chip starts
// the link feature with its nine places already held.

const (
	// The reels the giant symbol covers, and the one in the middle of it.
	BigFirst = 1
	BigLast  = 3
	BigReel  = 2

	// What a giant dice pays, per credit of the total bet, and the free
	// games it adds.
	BigScatterPay   = 1
	BigScatterGames = 3
)

// FreePays is the paytable for free games. The three middle reels always
// match, so only four and five in a row can win. The joker does not double
// a win in a free game.
var FreePays = [NumSymbols][Reels + 1]int{
	Ten:    {4: 5, 5: 15},
	Jack:   {4: 5, 5: 15},
	Queen:  {4: 5, 5: 15},
	King:   {4: 5, 5: 15},
	Ace:    {4: 5, 5: 20},
	Cherry: {4: 10, 5: 25},
	Plum:   {4: 10, 5: 25},
	Bell:   {4: 10, 5: 50},
	Dollar: {4: 10, 5: 100},
}

// FreeStrips is the order of symbols on reels 1 and 5 in a free game; the
// reels between them show the giant symbol instead. Neither strip has dice
// or chips, which come only as giant symbols, and the joker is on reel 5
// alone. No symbol comes twice within three stops.
var FreeStrips = [Reels][]Symbol{
	0:         {tn, ac, jk, ch, qn, pl, kg, be, tn, do, jk, ac, qn, ch, kg, pl, tn, be, jk, do, qn, ac, kg, ch, tn, pl, jk, qn, kg},
	Reels - 1: {kg, ch, qn, ac, jk, pl, tn, be, kg, wd, qn, ch, jk, ac, tn, pl, kg, do, qn, be, jk, ch, tn, ac, kg, pl, qn, jk, tn},
}

// BigWeights is how often each symbol comes up as the giant symbol, out of
// a thousand free games.
var BigWeights = [NumSymbols]int{
	Ten:     135,
	Jack:    135,
	Queen:   135,
	King:    135,
	Ace:     110,
	Cherry:  70,
	Plum:    70,
	Bell:    50,
	Dollar:  45,
	Wild:    30,
	Scatter: 60,
	Link:    25,
}

// bigTotal is what BigWeights adds up to.
func bigTotal() int {
	total := 0
	for _, w := range BigWeights {
		total += w
	}
	return total
}

// FreeWindow returns the three symbols showing on reel 1 or 5 in a free
// game, top to bottom, when it rests at the given stop.
func FreeWindow(reel, stop int) [Rows]Symbol {
	return window(FreeStrips[reel], stop)
}

// dealBig picks the giant symbol for a free game.
func (m *Machine) dealBig() Symbol {
	n := m.rng.IntN(bigTotal())
	for sym, w := range BigWeights {
		if n -= w; n < 0 {
			return Symbol(sym)
		}
	}
	panic("unreachable")
}

// freeGame plays one free game: reels 1 and 5 stop at random, and a giant
// symbol is dealt across the reels between them.
func (m *Machine) freeGame() Result {
	r := Result{Free: true, Big: m.dealBig()}
	bet := m.TotalBet()
	for i := range Reels {
		if i >= BigFirst && i <= BigLast {
			for row := range Rows {
				r.Grid[i][row] = r.Big
			}
			continue
		}
		r.Stops[i] = m.rng.IntN(len(FreeStrips[i]))
		r.Grid[i] = FreeWindow(i, r.Stops[i])
	}
	m.payLines(&r, &FreePays, 1)

	switch r.Big {
	case Scatter:
		r.Scatters = (BigLast - BigFirst + 1) * Rows
		r.ScatterWin = BigScatterPay * bet
		r.Win += r.ScatterWin
		r.FreeSpinsWon = BigScatterGames
	case Link:
		// The giant chip carries one prize, kept on its middle cell; its
		// other cells are held but carry nothing.
		link := Feature{Bet: bet, Respins: LinkRespins, Big: true}
		for i := BigFirst; i <= BigLast; i++ {
			for row := range Rows {
				link.Held[i][row] = true
			}
		}
		link.Chips[BigReel][Middle] = m.dealChip(bet, true, bigChipBets)
		r.Chips = link.Chips
		r.LinkStarted = true
		m.Link = &link
	}

	m.Bank.Pay(r.Win)
	m.FreeSpins += r.FreeSpinsWon
	return r
}
