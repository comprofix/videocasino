// Package machine holds the slot machine rules: reel strips, paylines, the
// paytable, free spins and the link feature. The credits come from the bank.
// It has no graphics so it can be tested on its own.
package machine

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"

	"videocasino/bank"
)

// Symbol is one picture on a reel.
type Symbol int

// The paying symbols come first, from the lowest paying to the highest,
// followed by the special symbols.
const (
	Ten Symbol = iota
	Jack
	Queen
	King
	Ace
	Cherry
	Plum
	Bell
	Dollar
	Wild    // joker: stands in for any paying symbol and doubles the win
	Scatter // dice: three or more anywhere award free spins
	Link    // poker chip: six or more anywhere start the link feature
	NumSymbols
)

func (s Symbol) String() string {
	return [...]string{
		"Ten", "Jack", "Queen", "King", "Ace", "Cherry", "Plum", "Bell",
		"Dollar", "Wild", "Scatter", "Link",
	}[s]
}

// Paying reports whether s is an ordinary symbol that wins along paylines.
func (s Symbol) Paying() bool {
	return s <= Dollar
}

const (
	Reels = 5
	Rows  = 3 // symbols showing on each reel
)

// Short names keep the strips below readable.
const (
	tn = Ten
	jk = Jack
	qn = Queen
	kg = King
	ac = Ace
	ch = Cherry
	pl = Plum
	be = Bell
	do = Dollar
	wd = Wild
	sc = Scatter
	lk = Link
)

// Strips is the order of symbols on each reel. How often a symbol appears
// on a strip sets how likely it is to land, so changing these changes the
// odds; TestRTP reports the resulting return to player.
//
// The wild is kept off the first reel, so it can never start a win. The link
// chips sit together in a stack of three, so a reel can show a column of
// them. No other symbol comes twice within three stops, so a reel never
// shows two of the same.
var Strips = [Reels][]Symbol{
	{qn, tn, sc, do, kg, qn, pl, do, jk, qn, kg, tn, ch, jk, tn, kg, be, qn, ch, pl, ac, qn, jk, pl, ac, tn, jk, qn, be, kg, do, ch, lk, lk, lk, jk, tn, kg, do, sc, be, qn, jk, ac},
	{jk, ac, do, jk, be, kg, wd, tn, qn, jk, tn, pl, kg, do, qn, ch, kg, pl, wd, ac, do, qn, jk, lk, lk, lk, pl, jk, ch, ac, wd, be, qn, kg, sc, do, be, ch, sc, tn, qn, jk, wd, kg},
	{qn, pl, tn, qn, jk, tn, qn, kg, pl, jk, wd, do, be, qn, kg, ch, jk, do, ac, kg, jk, wd, qn, tn, jk, ch, sc, qn, tn, jk, be, qn, ac, ch, be, pl, tn, qn, lk, lk, lk, kg, ac, jk},
	{tn, ac, kg, pl, wd, jk, ch, pl, jk, wd, qn, jk, ch, wd, pl, qn, kg, tn, jk, qn, tn, be, do, ac, kg, qn, tn, sc, wd, be, ch, qn, do, jk, qn, lk, lk, lk, tn, kg, sc, ac, do, be},
	{kg, do, ac, be, jk, tn, qn, jk, ch, be, sc, qn, tn, wd, pl, ac, do, tn, ac, qn, jk, sc, kg, qn, ac, ch, qn, pl, wd, kg, tn, jk, pl, tn, qn, wd, kg, be, ch, lk, lk, lk, do, jk},
}

// Row numbers within a reel's window.
const (
	Top = iota
	Middle
	Bottom
)

// Paylines gives, for each payline, the row it crosses on each reel from
// left to right. A player choosing N lines plays the first N.
var Paylines = [...][Reels]int{
	{1, 1, 1, 1, 1},
	{0, 0, 0, 0, 0},
	{2, 2, 2, 2, 2},
	{0, 1, 2, 1, 0},
	{2, 1, 0, 1, 2},
	{0, 0, 1, 0, 0},
	{2, 2, 1, 2, 2},
	{1, 2, 2, 2, 1},
	{1, 0, 0, 0, 1},
	{0, 1, 1, 1, 0},
	{2, 1, 1, 1, 2},
	{0, 1, 0, 1, 0},
	{2, 1, 2, 1, 2},
	{1, 0, 1, 0, 1},
	{1, 2, 1, 2, 1},
	{1, 1, 0, 1, 1},
	{1, 1, 2, 1, 1},
	{0, 2, 0, 2, 0},
	{2, 0, 2, 0, 2},
	{1, 0, 2, 0, 1},
	{1, 2, 0, 2, 1},
	{0, 0, 2, 0, 0},
	{2, 2, 0, 2, 2},
	{0, 2, 2, 2, 0},
	{2, 0, 0, 0, 2},
	{0, 2, 1, 2, 0},
	{2, 0, 1, 0, 2},
	{1, 1, 1, 1, 2},
	{0, 0, 1, 2, 2},
	{2, 2, 1, 0, 0},
	{0, 1, 1, 1, 2},
	{2, 1, 1, 1, 0},
	{0, 1, 2, 1, 2},
	{2, 1, 0, 1, 0},
	{0, 0, 0, 0, 1},
	{2, 2, 2, 2, 1},
	{0, 1, 0, 1, 2},
	{2, 1, 2, 1, 0},
	{1, 0, 1, 2, 1},
	{1, 2, 1, 0, 1},
	{1, 1, 0, 0, 0},
	{1, 1, 2, 2, 2},
	{1, 0, 0, 1, 2},
	{1, 2, 2, 1, 0},
	{1, 0, 1, 2, 2},
	{1, 2, 1, 0, 0},
	{2, 1, 0, 0, 1},
	{0, 1, 2, 2, 1},
	{0, 0, 1, 2, 1},
	{2, 2, 1, 0, 1},
}

// The numbers of lines and the bets per line a player can choose from.
var (
	LineChoices = []int{1, 5, 10, 25, 50}
	BetChoices  = []int{1, 2, 5, 10, 20}
)

// Pays is the payout per credit bet on a line, by symbol and by how many of
// it run along the payline from the left reel. Only the dollar pays for
// two; nothing pays for one. The prizes are kept small and close together,
// so that line wins come steadily and the big money is left to the chips.
var Pays = [NumSymbols][Reels + 1]int{
	Ten:    {0, 0, 0, 5, 10, 50},
	Jack:   {0, 0, 0, 5, 10, 50},
	Queen:  {0, 0, 0, 5, 10, 50},
	King:   {0, 0, 0, 5, 10, 50},
	Ace:    {0, 0, 0, 5, 20, 75},
	Cherry: {0, 0, 0, 5, 25, 100},
	Plum:   {0, 0, 0, 5, 25, 100},
	Bell:   {0, 0, 0, 5, 30, 100},
	Dollar: {0, 0, 2, 10, 50, 150},
}

// Scatters pay wherever they land, not along paylines. By how many are
// showing, ScatterPays is the prize per credit of the total bet and
// FreeSpinAwards the free games won: the same for three, four or five.
var (
	ScatterPays    = [Reels + 1]int{3: 2, 4: 15, 5: 100}
	FreeSpinAwards = [Reels + 1]int{3: 6, 4: 6, 5: 6}
)

// WildMultiplier multiplies a line win that a wild helps to make. Several
// wilds in one win still multiply it only once.
const WildMultiplier = 2

// Evaluate returns the payout per credit bet for the symbols along one
// payline, the symbol that won, and how many reels (counted from the left)
// make up the win. Wilds count as the symbol the line starts with, and a
// win with a wild in it is multiplied by WildMultiplier.
func Evaluate(line [Reels]Symbol) (pay int, sym Symbol, matched int) {
	return evaluate(line, &Pays, WildMultiplier)
}

// evaluate works a payline out against a paytable, multiplying a win that a
// wild helps to make.
func evaluate(line [Reels]Symbol, pays *[NumSymbols][Reels + 1]int, wildMultiplier int) (pay int, sym Symbol, matched int) {
	// The win belongs to the first symbol that is not a wild.
	sym = Wild
	for _, s := range line {
		if s != Wild {
			sym = s
			break
		}
	}
	if !sym.Paying() {
		return 0, 0, 0
	}
	run, wild := 0, false
	for run < Reels && (line[run] == sym || line[run] == Wild) {
		if line[run] == Wild {
			wild = true
		}
		run++
	}
	pay = pays[sym][run]
	if pay == 0 {
		return 0, 0, 0
	}
	if wild {
		pay *= wildMultiplier
	}
	return pay, sym, run
}

// Window returns the three symbols showing on a reel, top to bottom, when
// it rests at the given stop.
func Window(reel, stop int) [Rows]Symbol {
	return window(Strips[reel], stop)
}

func window(s []Symbol, stop int) [Rows]Symbol {
	n := len(s)
	return [Rows]Symbol{s[(stop+1)%n], s[stop], s[(stop-1+n)%n]}
}

// LineWin is a win on one payline.
type LineWin struct {
	Line    int    // index into Paylines
	Symbol  Symbol // the winning symbol
	Matched int    // winning reels, counted from the left
	Doubled bool   // a wild helped, multiplying the win
	Win     int    // credits won
}

// Result describes one spin.
type Result struct {
	Stops [Reels]int
	Grid  [Reels][Rows]Symbol // what each reel shows, top to bottom
	Wins  []LineWin           // winning paylines, in payline order

	Free         bool   // this was a free game
	Big          Symbol // on a free game, the giant symbol across reels 2 to 4
	Scatters     int    // scatters showing
	ScatterWin   int    // credits won by the scatters
	FreeSpinsWon int    // free spins this spin awarded

	Chips       [Reels][Rows]Chip // what each link chip showing carries; a giant chip's prize is on its middle cell
	LinkStarted bool              // enough chips landed to start the link feature

	Win int // total credits won
}

// Machine is the state of a game in progress. It has no credits of its own:
// bets are taken from the bank and wins paid into it.
type Machine struct {
	Bank       *bank.Bank
	Lines      int // paylines being played
	BetPerLine int
	FreeSpins  int      // free spins waiting to be played
	Link       *Feature // the link feature in progress, if there is one
	Stops      [Reels]int

	majorPot, grandPot int // in thousandths of a credit

	// Every chance in the game is drawn from rng, which reads from src.
	// reseedIn counts the spins until src is seeded afresh; it stays at
	// zero for a machine made with fixed seeds, which is never reseeded.
	src      *rand.ChaCha8
	rng      *rand.Rand
	reseedIn int
}

// reseedEvery is how many spins a machine made by NewRandom plays between
// fresh seeds.
const reseedEvery = 100

// New returns a machine that plays from the given bank. The seeds fix the
// sequence of spins, which the tests rely on.
func New(b *bank.Bank, seed1, seed2 uint64) *Machine {
	var seed [32]byte
	binary.LittleEndian.PutUint64(seed[:], seed1)
	binary.LittleEndian.PutUint64(seed[8:], seed2)
	return newMachine(b, seed)
}

// NewRandom returns a machine for play: one whose spins cannot be
// predicted. Its generator is a cryptographic one, seeded by the operating
// system and seeded afresh as play goes on.
func NewRandom(b *bank.Bank) *Machine {
	m := newMachine(b, freshSeed())
	m.reseedIn = reseedEvery
	return m
}

func newMachine(b *bank.Bank, seed [32]byte) *Machine {
	src := rand.NewChaCha8(seed)
	return &Machine{
		Bank:       b,
		Lines:      1,
		BetPerLine: 1,
		majorPot:   MajorStart * potUnit,
		grandPot:   GrandStart * potUnit,
		src:        src,
		rng:        rand.New(src),
	}
}

// freshSeed asks the operating system for a seed.
func freshSeed() [32]byte {
	var seed [32]byte
	crand.Read(seed[:]) // never fails: the program stops if it cannot be read
	return seed
}

// stir counts a spin towards the next fresh seed, and takes one when it is
// due.
func (m *Machine) stir() {
	if m.reseedIn == 0 {
		return
	}
	if m.reseedIn--; m.reseedIn == 0 {
		m.src.Seed(freshSeed())
		m.reseedIn = reseedEvery
	}
}

// TotalBet is the cost of one spin.
func (m *Machine) TotalBet() int {
	return m.Lines * m.BetPerLine
}

// Spin stops each reel at random and pays the wins on every payline being
// played, along with any scatter prize. It plays a free game if any are
// waiting; otherwise it takes the bet, and reports false and changes
// nothing if there are too few credits. It also reports false while a link
// feature is in progress: that is played out with Respin.
//
// Lines and BetPerLine must not be changed while free games are waiting or
// a link feature is in progress: they are played at the bet that won them.
func (m *Machine) Spin() (Result, bool) {
	if m.Link != nil {
		return Result{}, false
	}
	m.stir()
	if m.FreeSpins > 0 {
		m.FreeSpins--
		return m.freeGame(), true
	}
	var r Result
	bet := m.TotalBet()
	if !m.Bank.Take(bet) {
		return Result{}, false
	}
	// A cut of every bet goes into the two jackpot pots.
	m.majorPot += bet * majorGrowth
	m.grandPot += bet * grandGrowth

	link := Feature{Bet: bet, Respins: LinkRespins}
	for i := range Reels {
		r.Stops[i] = m.rng.IntN(len(Strips[i]))
		r.Grid[i] = Window(i, r.Stops[i])
		for row, s := range r.Grid[i] {
			switch s {
			case Scatter:
				r.Scatters++
			case Link:
				link.Chips[i][row] = m.dealChip(bet, !link.hasMajor(), chipBets)
				link.Held[i][row] = true
			}
		}
	}
	m.payLines(&r, &Pays, WildMultiplier)
	r.ScatterWin = ScatterPays[min(r.Scatters, Reels)] * bet
	r.Win += r.ScatterWin
	r.FreeSpinsWon = FreeSpinAwards[min(r.Scatters, Reels)]

	// The chips pay nothing by themselves, but enough of them lock in
	// place and start the link feature.
	r.Chips = link.Chips
	if link.Count() >= LinkTrigger {
		r.LinkStarted = true
		m.Link = &link
	}

	m.Stops = r.Stops
	m.Bank.Pay(r.Win)
	m.FreeSpins += r.FreeSpinsWon
	return r, true
}

// payLines works out the wins on the paylines being played from the grid
// in r, and adds them to it.
func (m *Machine) payLines(r *Result, pays *[NumSymbols][Reels + 1]int, wildMultiplier int) {
	for l := range m.Lines {
		var line [Reels]Symbol
		for i, row := range Paylines[l] {
			line[i] = r.Grid[i][row]
		}
		if pay, sym, matched := evaluate(line, pays, wildMultiplier); pay > 0 {
			win := pay * m.BetPerLine
			doubled := pay > pays[sym][matched]
			r.Wins = append(r.Wins, LineWin{Line: l, Symbol: sym, Matched: matched, Doubled: doubled, Win: win})
			r.Win += win
		}
	}
}
