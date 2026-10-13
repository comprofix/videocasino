package machine

// The link feature. Six or more chips anywhere on the reels lock in place
// and the rest of the grid respins. Each respin every empty cell may land a
// new chip, which locks too and puts the respins back to three. When the
// respins run out, or the grid is full, every chip is paid.

// Jackpot names one of the four link jackpots.
type Jackpot int

const (
	NoJackpot Jackpot = iota
	Mini              // a fixed number of total bets
	Minor             // a fixed number of total bets
	Major             // a pot that grows with play, won on a chip
	Grand             // a pot that grows with play, won by filling the grid
)

func (j Jackpot) String() string {
	return [...]string{"", "MINI", "MINOR", "MAJOR", "GRAND"}[j]
}

// Chip is what a link chip carries: a jackpot, or a number of total bets.
type Chip struct {
	Jackpot Jackpot
	Bets    int
}

const (
	Cells       = Reels * Rows
	LinkTrigger = 6 // chips needed to start the feature
	LinkRespins = 3

	// The mini and minor jackpots, in total bets.
	MiniBets  = 20
	MinorBets = 200

	// Where the major and grand pots start, in credits. At 100 credits to
	// the dollar these are $5,000 and $20,000.
	MajorStart = 500_000
	GrandStart = 2_000_000

	// The pots are kept in thousandths of a credit, so that a small bet
	// still moves them. These are the thousandths added per credit bet:
	// 1% of every bet to the major and 0.5% to the grand.
	potUnit     = 1000
	majorGrowth = 10
	grandGrowth = 5
)

// TopChipBets is the most an ordinary chip can carry, in total bets.
var TopChipBets = chipBets[len(chipBets)-1].bets

// MaxBet is the largest total bet that can be played.
var MaxBet = LineChoices[len(LineChoices)-1] * BetChoices[len(BetChoices)-1]

// chipList is a set of chip values, in total bets, and how often each is
// dealt relative to the others.
type chipList []struct{ bets, weight int }

// worth is the average value of a chip from the list, in total bets.
func (l chipList) worth() float64 {
	weight, worth := 0, 0
	for _, c := range l {
		weight += c.weight
		worth += c.weight * c.bets
	}
	return float64(worth) / float64(weight)
}

// chipBets are the ordinary chip values, in total bets, and how often each
// is dealt relative to the others.
var chipBets = chipList{
	{1, 500}, {2, 230}, {3, 110}, {4, 60}, {5, 50},
	{10, 30}, {15, 10}, {20, 6}, {50, 3}, {100, 1},
}

// bigChipBets are the values a giant chip can carry.
var bigChipBets = chipList{
	{5, 480}, {10, 270}, {15, 110}, {20, 60}, {25, 30}, {30, 18},
	{40, 12}, {50, 9}, {60, 5}, {80, 4}, {100, 2},
}

// The chance that a chip is dealt as each jackpot. The major pot pays the
// same whatever the bet, so its chance grows with the bet to keep the
// return even: majorOdds is the chance at MaxBet.
const (
	miniOdds  = 0.015
	minorOdds = 0.0005
	majorOdds = 0.0004
)

// respinOdds is the chance that an empty cell lands a chip on a respin. It
// is the same for every cell and every respin of the feature.
const respinOdds = 0.0455

// The grand pot pays the same whatever the bet, so the chip that would
// complete the grid is let land only some of the time, in proportion to
// the bet: grandOdds is that chance at MaxBet. It never falls below
// grandFloor, which keeps the grand within reach at the smallest bets.
const (
	grandOdds  = 0.1
	grandFloor = 0.0005
)

// grandChance is the chance that a chip which would complete the grid is
// let land, at the given total bet.
func grandChance(bet int) float64 {
	return max(grandOdds*betShare(bet), grandFloor)
}

// betShare is the bet as a fraction of the largest bet. The chances of the
// two growing jackpots are scaled by it.
func betShare(bet int) float64 {
	return float64(bet) / float64(MaxBet)
}

// dealChip picks what a newly landed chip carries: a jackpot, or a value
// from the list. A feature holds at most
// one major chip.
func (m *Machine) dealChip(bet int, major bool, list chipList) Chip {
	if major && m.rng.Float64() < majorOdds*betShare(bet) {
		return Chip{Jackpot: Major}
	}
	switch x := m.rng.Float64(); {
	case x < minorOdds:
		return Chip{Jackpot: Minor}
	case x < minorOdds+miniOdds:
		return Chip{Jackpot: Mini}
	}
	total := 0
	for _, c := range list {
		total += c.weight
	}
	n := m.rng.IntN(total)
	for _, c := range list {
		if n -= c.weight; n < 0 {
			return Chip{Bets: c.bets}
		}
	}
	panic("unreachable")
}

// Major and Grand are the jackpot pots, in whole credits.
func (m *Machine) Major() int { return m.majorPot / potUnit }
func (m *Machine) Grand() int { return m.grandPot / potUnit }

// ChipValue is what a chip pays, in credits, at the given total bet.
func (m *Machine) ChipValue(c Chip, bet int) int {
	switch c.Jackpot {
	case Mini:
		return MiniBets * bet
	case Minor:
		return MinorBets * bet
	case Major:
		return m.Major()
	}
	return c.Bets * bet
}

// Feature is a link feature in progress.
type Feature struct {
	Bet     int // the total bet that started it; chip values follow it
	Held    [Reels][Rows]bool
	Chips   [Reels][Rows]Chip // what each held cell carries
	Respins int               // respins left

	// Big is set when a giant chip started the feature. Its nine cells
	// are held as one chip, whose prize is kept on the middle cell.
	Big bool `json:",omitempty"`
}

// Count is the number of chips held.
func (l *Feature) Count() int {
	n := 0
	for _, col := range l.Held {
		for _, held := range col {
			if held {
				n++
			}
		}
	}
	return n
}

// hasMajor reports whether a major chip is already held.
func (l *Feature) hasMajor() bool {
	for i, col := range l.Held {
		for row, held := range col {
			if held && l.Chips[i][row].Jackpot == Major {
				return true
			}
		}
	}
	return false
}

// LinkTotal is what the chips held so far would pay, in credits.
func (m *Machine) LinkTotal() int {
	if m.Link == nil {
		return 0
	}
	total := 0
	for i, col := range m.Link.Held {
		for row, held := range col {
			if held {
				total += m.ChipValue(m.Link.Chips[i][row], m.Link.Bet)
			}
		}
	}
	return total
}

// Respin describes one respin of the link feature.
type Respin struct {
	New  [Reels][Rows]bool // cells where a chip landed
	Link Feature           // the feature after the respin
	Done bool              // the feature is over and has been paid

	// Set when the feature is over.
	Win      int // everything paid, both pots included
	MajorWin int // the major pot, if a major chip was held
	GrandWin int // the grand pot, if the grid was filled
}

// Respin plays one respin of the link feature, and pays the feature if that
// ends it. It reports false if no feature is in progress.
func (m *Machine) Respin() (Respin, bool) {
	l := m.Link
	if l == nil {
		return Respin{}, false
	}
	var r Respin

	filled := l.Count()

	var landed [][2]int
	for i := range Reels {
		for row := range Rows {
			if !l.Held[i][row] && m.rng.Float64() < respinOdds {
				landed = append(landed, [2]int{i, row})
			}
		}
	}
	// The chip that would complete the grid is let land only some of the time;
	// see grandChance.
	if n := len(landed); n > 0 && filled+n == Cells && m.rng.Float64() >= grandChance(l.Bet) {
		landed = landed[:n-1]
	}
	for _, cell := range landed {
		i, row := cell[0], cell[1]
		l.Held[i][row] = true
		l.Chips[i][row] = m.dealChip(l.Bet, !l.hasMajor(), chipBets)
		r.New[i][row] = true
	}

	if len(landed) > 0 {
		l.Respins = LinkRespins
	} else {
		l.Respins--
	}
	full := l.Count() == Cells
	r.Link = *l
	if l.Respins > 0 && !full {
		return r, true
	}

	// The feature is over: pay the chips, and the grand pot for a full grid.
	r.Done = true
	r.Win = m.LinkTotal()
	if l.hasMajor() {
		r.MajorWin = m.Major()
		m.majorPot = MajorStart * potUnit
	}
	if full {
		r.GrandWin = m.Grand()
		r.Win += r.GrandWin
		m.grandPot = GrandStart * potUnit
	}
	m.Bank.Pay(r.Win)
	m.Link = nil
	return r, true
}
