// Package holdem is Ultimate Texas Hold'em: two cards and a shared board of
// five played against the dealer's two, betting once as the board is
// turned, for pretend credits from the bank. Its rules are in the table
// package.
package holdem

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/holdem/table"
	"videocasino/pokerhand"
	"videocasino/shuffle"
	"videocasino/ui"
)

const (
	cardScale = 0.6
	cardW     = ui.CardW * cardScale
	cardH     = ui.CardH * cardScale
	cardStep  = 84 // how far each card of a row sits from the one before: clear of it

	// The table takes up the left of the screen: the dealer's cards at the
	// top, the board across the middle, and the player's cards under it on
	// the left with the bets beside them. What the game pays is listed down
	// the right, with the figures.
	tableX   = 300 // the middle of the table
	dealerY  = 36
	boardY   = 152
	messageY = 264
	playerY  = 302
	playerX  = 158 // the middle of the player's cards
	panelX   = 590
	panelW   = 180

	// The two rows of buttons at the bottom.
	rowAY   = 448
	rowBY   = 494
	buttonH = 40

	// dealTicks is the time between one card being dealt, or turned over,
	// and the next.
	dealTicks = 16
)

// The table: a dark forest green with an old-gold edge, and the bets ringed
// in black and named in yellow, as on a real table.
var (
	felt    = color.RGBA{12, 52, 28, 255}
	oldGold = color.RGBA{212, 175, 55, 255}
	ring    = color.RGBA{0, 0, 0, 255}
	letter  = color.RGBA{255, 214, 64, 255}
)

// shuffleLength is the shuffle shown before every hand: a single deck, so
// the short one.
const shuffleLength = shuffle.Short

// Where the bets lie: Trips in a diamond, and the ante, the blind and the
// play bet in circles, the ante and the blind side by side.
const (
	tripsX, tripsY = 440.0, 306.0
	tripsW, tripsH = 46.0, 27.0 // half the diamond's width and height
	anteX, anteY   = 395.0, 355.0
	blindX, blindY = 485.0, 355.0
	playX, playY   = 350.0, 412.0
	betR           = 28.0 // a circle's radius
)

type scene int

const (
	sceneMenu scene = iota
	sceneHelp
	sceneAbout
	scenePlay
)

type Game struct {
	scene scene
	ticks int
	snd   *ui.Sounds
	bank  *bank.Bank
	menu  *ui.Menu

	m    *table.Machine
	mark table.Bet // the bet the marker is on

	// The odds after the flop are counted on the spot, from a table of
	// every hand of five that takes a moment to make. It is made in the
	// background while the program starts, and is nil until it is ready.
	odds atomic.Pointer[table.Odds]

	// Where the pointer was last, so that it moves the marker only when it
	// is itself moved.
	seen         bool
	lastX, lastY int

	// The deck is shuffled before every hand, and that is shown before the
	// cards are dealt.
	shuffling    bool
	shuffleStart int // the tick it began at

	// Cards are dealt and turned over one by one: the player's two, the
	// board as it is turned, and the dealer's two at the end. The machine
	// has dealt them by then, and the hand is settled once the last is up.
	showing     bool // cards are being dealt or turned over
	showAt      int  // the tick at which the next is shown
	playerShown int
	boardShown  int
	dealerShown int

	// result is the hand last finished, and over is set while it is on
	// show: from when it ends until the next is dealt.
	result table.Result
	over   bool

	advice  table.Advice
	hinting bool // the hint is on show

	// The sounds that are this game's own.
	chipSound   []byte
	riffleSound []byte

	credits int // credits on display; a win is added once the cards are shown
	win     int
	message string
	confirm ui.Confirm // asks before the credits are reset

	saveFailed bool
}

// New returns the game, playing from the given bank and picking up whatever
// it last kept there.
func New(b *bank.Bank, snd *ui.Sounds) *Game {
	g := &Game{snd: snd, bank: b, mark: table.Ante}
	g.menu = &ui.Menu{
		Items: []ui.MenuItem{
			{Title: "HOW TO PLAY GAME"},
			{Title: "PLAY GAME"},
			{Title: "ABOUT THIS GAME"},
			{Title: "BACK TO MAIN MENU"},
		},
		Columns: 1, Rows: 4,
		Y: 190, RowH: 36,
		Number: ui.White, Title: ui.LightRed,
	}
	g.m = table.NewRandom(b)
	g.message = "PLACE YOUR BETS"
	g.chipSound = ui.Tone(1480, 22, 0.14, true)
	g.riffleSound = shuffle.Sound()

	var s table.Saved
	if b.Load(saveName, &s) {
		g.m.Restore(s)
		g.resume()
	}
	g.credits = b.Credits

	go func() {
		g.odds.Store(table.NewOdds())
	}()
	return g
}

// resume puts back on screen a hand that was still to be played when the
// game was last closed.
func (g *Game) resume() {
	if g.m.Round != table.NoHand {
		g.playerShown, g.boardShown = table.HoleSize, g.m.Round.Shown()
		g.message = "HAND TO FINISH - " + roundChoices(g.m.Round)
	}
}

// The buttons under the table. The three that play the hand change with
// the round.
func chipButton(i int) ui.Button {
	return ui.Button{X: 30 + float64(i)*76, Y: rowAY, W: 70, H: buttonH}
}

func playButton(i int) ui.Button {
	return ui.Button{X: 30 + float64(i)*90, Y: rowBY, W: 84, H: buttonH}
}

var (
	clearButton = ui.Button{X: 420, Y: rowAY, W: 80, H: buttonH}
	rebetButton = ui.Button{X: 506, Y: rowAY, W: 84, H: buttonH}
	dealButton  = ui.Button{X: 600, Y: rowAY, W: 170, H: rowBY + buttonH - rowAY} // as tall as two rows
	hintButton  = ui.Button{X: 300, Y: rowBY, W: 70, H: buttonH}
	coinButton  = ui.Button{X: 376, Y: rowBY, W: 130, H: buttonH}
	resetButton = ui.Button{X: 512, Y: rowBY, W: 78, H: buttonH}
)

// The keys that choose each chip, along the top of the keyboard and on the
// number pad.
var chipKeys = [][2]ebiten.Key{
	{ebiten.KeyDigit1, ebiten.KeyNumpad1},
	{ebiten.KeyDigit2, ebiten.KeyNumpad2},
	{ebiten.KeyDigit3, ebiten.KeyNumpad3},
	{ebiten.KeyDigit4, ebiten.KeyNumpad4},
	{ebiten.KeyDigit5, ebiten.KeyNumpad5},
}

// play is something the player can do with a hand: check, fold, or bet so
// many antes.
type play struct {
	antes int // 0 to check or fold
	label string
}

// plays returns what can be done in a round, in the order of the buttons.
func plays(r table.Round) []play {
	switch r {
	case table.Flop:
		return []play{{0, "CHECK"}, {2, "BET 2X"}}
	case table.River:
		return []play{{0, "FOLD"}, {1, "BET 1X"}}
	}
	return []play{{0, "CHECK"}, {4, "BET 4X"}, {3, "BET 3X"}}
}

// roundChoices says what can be done in a round.
func roundChoices(r table.Round) string {
	switch r {
	case table.Flop:
		return "CHECK, OR BET 2X"
	case table.River:
		return "BET 1X, OR FOLD"
	}
	return "CHECK, OR BET 4X OR 3X"
}

// Update plays one tick. It reports true when the player has asked to go
// back to the main menu.
func (g *Game) Update() (leave bool) {
	g.ticks++

	switch g.scene {
	case sceneMenu:
		switch g.menu.Update() {
		case 0:
			g.scene = sceneHelp
		case 1:
			g.scene = scenePlay
		case 2:
			g.scene = sceneAbout
		case 3, ui.MenuBack:
			// The menu starts afresh the next time the game is chosen.
			g.menu.Reset()
			return true
		}

	case sceneHelp, sceneAbout:
		if ui.Dismissed() {
			g.scene = sceneMenu
		}

	case scenePlay:
		// While the player is asked about a reset, nothing else moves.
		if g.confirm.Open {
			if g.confirm.Update() {
				g.reset()
			}
			break
		}
		g.updatePlay()
	}
	return false
}

// onTrips and onCircle report whether a point lies on a bet's shape.
func onTrips(x, y int) bool {
	return math.Abs(float64(x)-tripsX)/tripsW+math.Abs(float64(y)-tripsY)/tripsH <= 1
}

func onCircle(x, y int, cx, cy float64) bool {
	dx, dy := float64(x)-cx, float64(y)-cy
	return dx*dx+dy*dy <= betR*betR
}

// betAt returns the bet a point lies on: the blind counts as the ante, as
// they go down together.
func betAt(x, y int) (table.Bet, bool) {
	switch {
	case onTrips(x, y):
		return table.Trips, true
	case onCircle(x, y, anteX, anteY), onCircle(x, y, blindX, blindY):
		return table.Ante, true
	}
	return 0, false
}

func (g *Game) updatePlay() {
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	skip := click || ui.Pressed(ebiten.KeySpace, ebiten.KeyEnter, ebiten.KeyNumpadEnter)
	// The deal key or a click cuts short the shuffle, and the cards.
	if g.shuffling {
		g.shuffleStep(skip)
		return
	}
	if g.showing {
		g.show(skip)
		return
	}
	// The bank may have changed while the game was not being played.
	g.credits = g.bank.Credits
	if ui.Pressed(ebiten.KeyEscape) {
		g.scene = sceneMenu
		return
	}
	x, y := ebiten.CursorPosition()
	moved := g.seen && (x != g.lastX || y != g.lastY)
	g.seen, g.lastX, g.lastY = true, x, y
	hit := func(b ui.Button) bool { return click && b.Hit(x, y) }

	switch {
	case hit(coinButton), ui.Pressed(ebiten.KeyC):
		g.insertCoin()
		return
	case hit(resetButton), ui.Pressed(ebiten.KeyR):
		g.askReset()
		return
	}

	// With a hand dealt there is only to play it.
	if r := g.m.Round; r != table.NoHand {
		for i, p := range plays(r) {
			if hit(playButton(i)) {
				g.choose(p.antes)
			}
		}
		switch {
		case r != table.River && ui.Pressed(ebiten.KeyK):
			g.choose(0)
		case r == table.River && ui.Pressed(ebiten.KeyF):
			g.choose(0)
		case hit(hintButton), ui.Pressed(ebiten.KeyH):
			g.showHint()
		}
		for antes, keys := range [][2]ebiten.Key{1: {ebiten.KeyDigit1, ebiten.KeyNumpad1}, {ebiten.KeyDigit2, ebiten.KeyNumpad2}, {ebiten.KeyDigit3, ebiten.KeyNumpad3}, {ebiten.KeyDigit4, ebiten.KeyNumpad4}} {
			if antes > 0 && ui.Pressed(keys[:]...) {
				for _, p := range plays(r) {
					if p.antes == antes {
						g.choose(antes)
					}
				}
			}
		}
		return
	}

	// The pointer moves the marker to the bet it is over. A click bets
	// there, and a click of the other button takes the bet off again.
	away := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if b, ok := betAt(x, y); ok && (moved || click || away) {
		g.mark = b
		switch {
		case click:
			g.place()
		case away:
			g.remove()
		}
	}
	for i, n := range table.ChipChoices {
		if hit(chipButton(i)) || ui.Pressed(chipKeys[i][:]...) {
			g.setChip(n)
		}
	}
	switch {
	case hit(clearButton), ui.Pressed(ebiten.KeyX):
		g.clear()
	case hit(rebetButton), ui.Pressed(ebiten.KeyB):
		g.rebet()
	case hit(dealButton), ui.Pressed(ebiten.KeySpace):
		g.pressDeal()
	case ui.Pressed(ebiten.KeyEnter, ebiten.KeyNumpadEnter):
		g.place()
	case ui.Pressed(ebiten.KeyBackspace, ebiten.KeyDelete):
		g.remove()
	case ui.Pressed(ebiten.KeyArrowLeft):
		g.mark = table.Trips
	case ui.Pressed(ebiten.KeyArrowRight):
		g.mark = table.Ante
	}
}

// clearWin takes the last hand off the table, once the player has moved on
// from it.
func (g *Game) clearWin() {
	g.win = 0
	g.over = false
	g.hinting = false
	g.playerShown, g.boardShown, g.dealerShown = 0, 0, 0
}

func (g *Game) setChip(n int) {
	g.m.Chip = n
	g.snd.Play(g.snd.Click)
	g.save()
}

const outOfCredits = "OUT OF CREDITS - INSERT A COIN OR RESET"

// place puts a chip on the bet the marker is on, and on the blind too if
// that is the ante. Like the rest of the betting, it does nothing while a
// hand is waiting to be played.
func (g *Game) place() {
	if g.m.Round != table.NoHand {
		return
	}
	g.clearWin()
	if !g.m.Place(g.mark) {
		switch {
		case g.bank.Credits == 0:
			g.message = outOfCredits
		case g.m.Bets[g.mark]+g.m.Chip > table.MaxBet:
			g.message = fmt.Sprintf("NO MORE THAN %s ON ONE BET", bank.Money(table.MaxBet))
		default:
			g.message = "NOT ENOUGH CREDITS - THE BANK MUST COVER A BET OF 4X TOO"
		}
		return
	}
	g.message = "PLACE YOUR BETS"
	g.snd.Play(g.chipSound)
	g.save()
}

// remove takes a chip off the bet the marker is on.
func (g *Game) remove() {
	if g.m.Round != table.NoHand || !g.m.Remove(g.mark) {
		return
	}
	g.clearWin()
	g.message = "PLACE YOUR BETS"
	g.snd.Play(g.chipSound)
	g.save()
}

// clear takes every chip off the table.
func (g *Game) clear() {
	if g.m.Round != table.NoHand {
		return
	}
	g.m.Clear()
	g.clearWin()
	g.message = "PLACE YOUR BETS"
	g.snd.Play(g.snd.Click)
	g.save()
}

// rebet puts the bets of the last hand back on the table.
func (g *Game) rebet() {
	if g.m.Round != table.NoHand {
		return
	}
	g.clearWin()
	if !g.m.Rebet() {
		g.message = "NOT ENOUGH CREDITS TO BET THE SAME AGAIN"
		if g.m.LastTotal() == 0 {
			g.message = "THERE ARE NO BETS TO MAKE AGAIN YET"
		}
		return
	}
	g.message = "THE SAME BETS AGAIN - PRESS DEAL"
	g.snd.Play(g.chipSound)
	g.save()
}

// askReset asks the player to make sure before the credits are reset.
func (g *Game) askReset() {
	g.confirm.Ask("RESET CREDITS TO "+bank.Money(bank.StartCredits)+"?", "YOU HAVE "+bank.Dollars(g.bank.Credits))
}

// reset puts the bank's credits back to the starting amount. The bets and
// a hand that is waiting are left as they are.
func (g *Game) reset() {
	g.bank.Reset()
	g.credits = g.bank.Credits
	g.message = "CREDITS RESET"
	g.snd.Play(g.snd.Click)
	g.save()
}

// insertCoin adds credits to the bank, so play can go on after they run
// out.
func (g *Game) insertCoin() {
	g.bank.InsertCoin()
	g.credits = g.bank.Credits
	g.message = fmt.Sprintf("COIN INSERTED - %s ADDED", bank.Money(bank.CoinCredits))
	g.snd.Play(g.snd.Coin)
	g.save()
}

// pressDeal shuffles the deck and then deals. With nothing on the table the
// bets of the last hand are made again first. If no hand can be dealt there
// is no shuffle to show: it says why.
func (g *Game) pressDeal() {
	if g.m.Round != table.NoHand {
		return
	}
	if g.m.Total() == 0 {
		g.m.Rebet()
	}
	if !g.m.CanDeal() {
		g.deal() // which says why not
		return
	}
	g.clearWin()
	g.message = "SHUFFLING"
	g.shuffling = true
	g.shuffleStart = g.ticks
}

// shuffleStep carries the shuffle on by a tick, and deals the hand when it
// is over or has been cut short.
func (g *Game) shuffleStep(skip bool) {
	t := g.ticks - g.shuffleStart
	if skip || t >= shuffleLength.Ticks() {
		g.shuffling = false
		g.deal()
		return
	}
	if shuffleLength.Falls(t) {
		g.snd.Play(g.riffleSound)
	}
}

// deal has the machine deal the player's two cards, then shows them one by
// one. With too few credits nothing is dealt: the bank is every game's, so
// it is for the player to insert a coin or reset it.
func (g *Game) deal() {
	if g.m.Round != table.NoHand {
		return
	}
	if g.m.Total() == 0 {
		g.m.Rebet()
	}
	if !g.m.Deal() {
		g.clearWin()
		switch {
		case g.bank.Credits == 0:
			g.message = outOfCredits
		case g.m.Total() == 0 && g.m.LastTotal() == 0:
			g.message = "PLACE A BET BEFORE YOU DEAL"
		case g.m.Bets[table.Ante] == 0:
			g.message = "PUT A CHIP ON THE ANTE TO BE DEALT A HAND"
		default:
			g.message = "NOT ENOUGH CREDITS - THE BANK MUST COVER A BET OF 4X TOO"
		}
		return
	}
	g.save()
	g.clearWin()
	g.startShowing()
	g.credits = g.bank.Credits
}

// startShowing sets the cards to be shown one by one from the next tick.
func (g *Game) startShowing() {
	g.showing = true
	g.showAt = g.ticks + dealTicks
	g.hinting = false
	g.message = ""
}

// choose does what the player has chosen for the hand: checks or folds for
// 0, or bets so many antes.
func (g *Game) choose(antes int) {
	r := g.m.Round
	switch {
	case antes > 0:
		res, ok := g.m.Bet(antes)
		if !ok {
			g.message = "NOT ENOUGH CREDITS FOR THAT BET - INSERT A COIN OR BET LESS"
			return
		}
		g.save()
		g.result = res
		g.startShowing()
		g.credits = g.bank.Credits - res.Win()
		g.snd.Play(g.chipSound)
	case r == table.River:
		res, ok := g.m.Fold()
		if !ok {
			return
		}
		g.save()
		g.result = res
		g.hinting = false
		g.land()
	default:
		if !g.m.Check() {
			return
		}
		g.save()
		g.startShowing()
		g.snd.Play(g.snd.Click)
	}
}

// showHint weighs up the choices open in the round and marks the best.
func (g *Game) showHint() {
	odds := g.odds.Load()
	if odds == nil {
		g.message = "STILL WORKING OUT THE ODDS - TRY AGAIN IN A MOMENT"
		return
	}
	g.advice = odds.Advise(g.m.Round, g.m.Hole, g.m.Board)
	g.hinting = true
	g.snd.Play(g.snd.Click)

	ante := float64(g.m.Stake[table.Ante])
	labels := plays(g.m.Round)
	best := g.advice.Choices[g.advice.Best]
	var others []string
	for _, c := range g.advice.Choices {
		if c != best {
			others = append(others, fmt.Sprintf("%s %s", labelOf(labels, c.Antes), bank.Worth(c.Worth*ante)))
		}
	}
	g.message = fmt.Sprintf("BEST TO %s, WORTH %s - %s", labelOf(labels, best.Antes), bank.Worth(best.Worth*ante), strings.Join(others, ", "))
}

// labelOf returns the name of the play that bets so many antes.
func labelOf(ps []play, antes int) string {
	for _, p := range ps {
		if p.antes == antes {
			return p.label
		}
	}
	return ""
}

// show carries on by a tick the dealing and turning of the cards, and says
// how things stand once the last is shown.
func (g *Game) show(skip bool) {
	// What is to be shown: the player's cards, the board as far as the
	// hand has gone, and the dealer's once it is over and was played.
	wantBoard, wantDealer := g.m.Round.Shown(), 0
	if g.m.Round == table.NoHand {
		wantBoard = table.BoardSize
		if g.result.Outcome != table.Folded {
			wantDealer = table.HoleSize
		}
	}
	if skip {
		g.playerShown, g.boardShown, g.dealerShown = table.HoleSize, wantBoard, wantDealer
		g.snd.Play(g.snd.Card)
	} else {
		if g.ticks < g.showAt {
			return
		}
		g.showAt = g.ticks + dealTicks
		turned := true
		switch {
		case g.playerShown < table.HoleSize:
			g.playerShown++
		case g.boardShown < wantBoard:
			g.boardShown++
		case g.dealerShown < wantDealer:
			g.dealerShown++
		default:
			turned = false
		}
		if turned {
			g.snd.Play(g.snd.Card)
			return
		}
	}

	g.showing = false
	g.credits = g.bank.Credits
	if r := g.m.Round; r != table.NoHand {
		g.message = handName(g.m.Hole, g.m.Board[:r.Shown()]) + " - " + roundChoices(r)
		return
	}
	g.land()
}

// land shows how the hand went, now that its cards are all up.
func (g *Game) land() {
	r := &g.result
	g.over = true
	g.win = r.Win()
	g.credits = g.bank.Credits
	dealer := "" // the dealer's hand, which a fold leaves undealt
	if r.Outcome != table.Folded {
		dealer = r.DealerScore.String()
	}
	switch {
	case r.Outcome == table.Folded:
		g.message = "YOU FOLD - THE ANTE AND THE BLIND ARE LOST"
	case r.Outcome == table.Won && !r.Qualified:
		g.message = fmt.Sprintf("DEALER HAS %s AND DOES NOT QUALIFY - YOU WIN %s", dealer, bank.Money(r.Win()))
	case r.Outcome == table.Won:
		g.message = fmt.Sprintf("YOU BEAT THE DEALER'S %s - YOU WIN %s", dealer, bank.Money(r.Win()))
	case r.Outcome == table.Pushed:
		g.message = "THE HANDS TIE - YOUR BETS ARE HANDED BACK"
	case !r.Qualified:
		g.message = "DEALER WINS WITH " + dealer + " - THE ANTE IS HANDED BACK"
	default:
		g.message = "DEALER WINS WITH " + dealer
	}
	if r.TripsPaid > 0 && r.Outcome != table.Won {
		g.message += " - TRIPS PAYS " + bank.Money(r.TripsPaid)
	}
	switch {
	case r.Outcome == table.Won && table.BlindPays[r.Player.Category()] > 0:
		g.snd.Play(g.snd.BigWin)
	case r.Outcome == table.Won || r.TripsPaid > 0:
		g.snd.Play(g.snd.Win)
	case r.Outcome == table.Folded:
		g.snd.Play(g.snd.Click)
	}
}

var (
	rankNames = [...]string{"TWO", "THREE", "FOUR", "FIVE", "SIX", "SEVEN", "EIGHT", "NINE", "TEN", "JACK", "QUEEN", "KING", "ACE"}
	manyNames = [...]string{"TWOS", "THREES", "FOURS", "FIVES", "SIXES", "SEVENS", "EIGHTS", "NINES", "TENS", "JACKS", "QUEENS", "KINGS", "ACES"}
)

// handName names what the player holds: the two cards alone until the flop
// is all turned, such as "ACE QUEEN SUITED", and the best five after it.
func handName(hole [table.HoleSize]cards.Card, board []cards.Card) string {
	if len(board) < table.FlopSize {
		hi, lo := hole[0].Rank(), hole[1].Rank()
		if hi < lo {
			hi, lo = lo, hi
		}
		if hi == lo {
			return "PAIR OF " + manyNames[hi-cards.Two]
		}
		name := rankNames[hi-cards.Two] + " " + rankNames[lo-cards.Two]
		if hole[0].Suit() == hole[1].Suit() {
			name += " SUITED"
		}
		return name
	}
	_, s := pokerhand.Best(append(hole[:], board...))
	return s.String()
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(felt)
	ui.DrawBorder(screen, oldGold)

	switch g.scene {
	case sceneMenu:
		g.drawMenu(screen)
	case sceneHelp:
		g.drawHelp(screen)
	case sceneAbout:
		g.drawAbout(screen)
	case scenePlay:
		g.drawPlay(screen)
		g.confirm.Draw(screen)
	}
}

func (g *Game) drawMenu(screen *ebiten.Image) {
	ui.DrawText(screen, "ULTIMATE TEXAS HOLD'EM", ui.ScreenW/2, 70, 44, ui.Yellow, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "POKER AGAINST THE DEALER", ui.ScreenW/2, 128, 20, ui.White, ui.AlignCenter, ui.AlignStart)

	g.menu.Draw(screen)
	ui.DrawText(screen, "ARROWS move    ENTER or NUMBER choose    ESC main menu", ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "MAKE YOUR CHOICE", ui.ScreenW/2, 350, 24, ui.LightCyan, ui.AlignCenter, ui.AlignStart)

	// The best two cards there are to start with, for luck.
	const scale = 0.62
	for i, s := range []cards.Suit{cards.Spades, cards.Hearts} {
		x := ui.ScreenW/2 + (float64(i)-1)*(ui.CardW*scale+10) + 5
		ui.DrawCard(screen, cards.New(cards.Ace, s), x, 420, scale)
	}
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	ui.DrawText(screen, "HOW TO PLAY", ui.ScreenW/2, 22, 30, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	lines := []string{
		"Put chips on the ANTE, which puts the same on the BLIND, and on TRIPS if",
		"you want it, then deal. You and the dealer get two cards each, and five",
		"are turned on the board for both. You bet once: 4X or 3X the ante before",
		"the flop, 2X on the flop, or 1X on the river, where you must bet or fold.",
		"Checking turns the next of the board. Once you bet, the rest is turned.",
		"The best five of seven win. The PLAY bet pays 1 to 1. The ANTE pays 1 to 1",
		"if the dealer has a pair or better, and is handed back if not. The BLIND",
		"pays on a straight or better, as listed, and is handed back on a lesser",
		"win. A loss loses the play bet and the blind, and the ante if the dealer",
		"qualifies. A tie hands them back. TRIPS pays on your best five, as listed,",
		"whatever the dealer holds, even if you fold. HINT gives the best play.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2-285, 60+float64(i)*18, 15, ui.White, ui.AlignStart, ui.AlignStart)
	}

	keys := [][2]string{
		{"SPACE", "shuffle and deal"},
		{"LEFT / RIGHT", "move the marker between trips and the ante"},
		{"ENTER / BACKSPACE", "put a chip down, or take one off"},
		{"1 to 5", "choose a chip"},
		{"X, B", "clear the table; make the last bets again"},
		{"K", "check"},
		{"4, 3, 2, 1", "bet that many times the ante"},
		{"F", "fold, on the river"},
		{"H", "hint: what each choice is worth"},
		{"C, R", "insert a coin; reset your credits"},
		{"S", "sound on or off"},
		{"ESC", "back to the menu"},
	}
	for i, k := range keys {
		y := 270 + float64(i)*18
		ui.DrawText(screen, k[0], ui.ScreenW/2-70, y, 14, ui.LightRed, ui.AlignEnd, ui.AlignStart)
		ui.DrawText(screen, k[1], ui.ScreenW/2-45, y, 14, ui.White, ui.AlignStart, ui.AlignStart)
	}
	ui.DrawText(screen, "With the mouse, click a bet to put a chip down and right click to take it off.", ui.ScreenW/2, 496, 13, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 546, 20, ui.LightCyan, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawAbout(screen *ebiten.Image) {
	ui.DrawText(screen, "ABOUT THIS GAME", ui.ScreenW/2, 50, 36, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	lines := []string{
		"Ultimate Texas Hold'em is written in Go with Ebitengine.",
		"",
		"Each hand is dealt from one deck, freshly shuffled.",
		"",
		"Played perfectly, the ante, the blind and the play bet",
		fmt.Sprintf("pay back %.2f%% of what is bet on them,", table.Return()*100),
		fmt.Sprintf("and the ante loses %.2f%% of itself.", table.Edge()*100),
		fmt.Sprintf("That is to bet 4X before the flop on %.1f%% of hands.", table.RaisedPreflop()*100),
		fmt.Sprintf("Trips pays back %.2f%% of what is bet on it.", table.TripsReturn()*100),
		"",
		"It is played with pretend credits only.",
		"No real money goes in or comes out.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2, 110+float64(i)*28, 18, ui.White, ui.AlignCenter, ui.AlignStart)
	}
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 500, 20, ui.LightCyan, ui.AlignCenter, ui.AlignStart)
}

// drawRow draws a row of cards with its middle at cx: as many face up as
// are shown, the rest face down. Those in the player's best five are
// outlined in yellow, and those in the dealer's in cyan, further out, so a
// card of the board that both use shows both.
func drawRow(screen *ebiten.Image, cs []cards.Card, shown int, cx, y float64, mine, theirs map[cards.Card]bool) {
	x := cx - (cardW+float64(len(cs)-1)*cardStep)/2
	for i, c := range cs {
		cardX := x + float64(i)*cardStep
		if i >= shown {
			ui.DrawCardBack(screen, cardX, y, cardScale)
			continue
		}
		ui.DrawCard(screen, c, cardX, y, cardScale)
		if theirs[c] {
			ui.FrameRect(screen, cardX-6, y-6, cardW+12, cardH+12, 2, ui.LightCyan)
		}
		if mine[c] {
			ui.FrameRect(screen, cardX-3, y-3, cardW+6, cardH+6, 3, ui.Yellow)
		}
	}
}

// bestFive returns the cards of the best five of seven, to be outlined.
func bestFive(hole [table.HoleSize]cards.Card, board [table.BoardSize]cards.Card) map[cards.Card]bool {
	five, _ := pokerhand.Best(append(hole[:], board[:]...))
	in := make(map[cards.Card]bool, len(five))
	for _, c := range five {
		in[c] = true
	}
	return in
}

func (g *Game) drawPlay(screen *ebiten.Image) {
	// The hand on show is the one being played, or the one last finished.
	hole, board, dealer := g.result.Hole, g.result.Board, g.result.Dealer
	if g.m.Round != table.NoHand {
		hole, board, dealer = g.m.Hole, g.m.Board, [table.HoleSize]cards.Card{}
	}
	done := g.over && !g.showing

	// At the end the five cards that make each hand are outlined.
	var mine, theirs map[cards.Card]bool
	if done {
		mine = bestFive(hole, board)
		if g.dealerShown == table.HoleSize {
			theirs = bestFive(dealer, board)
		}
	}

	if g.shuffling {
		// While the deck is shuffled the table is clear.
		shuffleLength.Draw(screen, g.ticks-g.shuffleStart, tableX, (dealerY+boardY+cardH)/2+20, cardScale)
	} else {
		label := "DEALER"
		if g.dealerShown == table.HoleSize {
			label = "DEALER: " + g.result.DealerScore.String()
			if !g.result.Qualified {
				label += " - DOES NOT QUALIFY"
			}
		}
		ui.DrawText(screen, label, tableX, dealerY-16, 13, ui.White, ui.AlignCenter, ui.AlignStart)
		drawRow(screen, dealer[:], g.dealerShown, tableX, dealerY, mine, theirs)
		drawRow(screen, board[:], g.boardShown, tableX, boardY, mine, theirs)

		label, ink := "YOUR HAND", color.Color(ui.White)
		if g.playerShown == table.HoleSize {
			label = "YOUR HAND: " + handName(hole, board[:g.boardShown])
			if done && g.result.Outcome == table.Won {
				ink = ui.Yellow
			}
		}
		// Lined up with the first card, so a long name runs to the right.
		ui.DrawText(screen, label, playerX-(cardW+cardStep)/2, playerY-16, 13, ink, ui.AlignStart, ui.AlignStart)
		drawRow(screen, hole[:], g.playerShown, playerX, playerY, mine, theirs)
	}
	ui.DrawText(screen, g.message, tableX, messageY, 13, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	g.drawBets(screen)
	g.drawPanel(screen)
	g.drawButtons(screen)
}

// drawBets draws Trips in its diamond and the other three in their circles,
// with the chips on them and the marker.
func (g *Game) drawBets(screen *ebiten.Image) {
	// While a hand is played the bets on show are the ones it was dealt
	// for. Once it is over the ones that were paid or handed back are left
	// on show until the player moves on, those that won lit.
	var trips, ante, blind, play int
	var tripsLit, anteLit, blindLit, playLit bool
	inHand := g.m.Round != table.NoHand || g.showing
	done := g.over && !g.showing && g.m.Total() == 0
	switch {
	case g.m.Round != table.NoHand:
		trips, ante = g.m.Stake[table.Trips], g.m.Stake[table.Ante]
		blind = ante
	case inHand:
		r := &g.result
		trips, ante, play = r.Stake[table.Trips], r.Stake[table.Ante], r.Play
		blind = ante
	case done:
		r := &g.result
		flash := g.ticks/15%2 == 0
		if r.TripsPaid > 0 {
			trips, tripsLit = r.Stake[table.Trips], flash
		}
		if r.Paid > 0 {
			won := r.Outcome == table.Won
			ante, anteLit = r.Stake[table.Ante], flash && won && r.Qualified
			if r.Outcome != table.Lost {
				blind, blindLit = r.Stake[table.Ante], flash && won && table.BlindPays[r.Player.Category()] > 0
				play, playLit = r.Play, flash && won
			}
		}
	default:
		trips, ante = g.m.Bets[table.Trips], g.m.Bets[table.Ante]
		blind = ante
	}
	marking := !inHand && !g.shuffling

	// Trips: a diamond.
	pts := [4][2]float64{{tripsX, tripsY - tripsH}, {tripsX + tripsW, tripsY}, {tripsX, tripsY + tripsH}, {tripsX - tripsW, tripsY}}
	edge := color.Color(ui.White)
	if tripsLit {
		edge = ui.Yellow
	}
	for i := range pts {
		a, b := pts[i], pts[(i+1)%len(pts)]
		ui.DrawLine(screen, a[0], a[1], b[0], b[1], 2, edge)
		if marking && g.mark == table.Trips {
			ui.DrawLine(screen, a[0]+sign(a[0]-tripsX)*5, a[1]+sign(a[1]-tripsY)*5, b[0]+sign(b[0]-tripsX)*5, b[1]+sign(b[1]-tripsY)*5, 2, ui.White)
		}
	}
	ui.DrawText(screen, "TRIPS", tripsX, tripsY-14, 11, letter, ui.AlignCenter, ui.AlignStart)
	if trips > 0 {
		ui.DrawChip(screen, tripsX, tripsY+10, trips, table.ChipChoices, tripsLit)
	}

	circle := func(x, y float64, name string, amount int, lit, mark bool) {
		ui.FrameCircle(screen, x, y, betR, 3, ring)
		if lit {
			ui.FrameCircle(screen, x, y, betR-3, 3, ui.Yellow)
		}
		if mark {
			ui.FrameCircle(screen, x, y, betR+3, 2, ui.White)
		}
		ui.DrawText(screen, name, x, y-17, 11, letter, ui.AlignCenter, ui.AlignStart)
		if amount > 0 {
			ui.DrawChip(screen, x, y+9, amount, table.ChipChoices, lit)
		}
	}
	anteMarked := marking && g.mark == table.Ante
	circle(anteX, anteY, "ANTE", ante, anteLit, anteMarked)
	circle(blindX, blindY, "BLIND", blind, blindLit, anteMarked)
	circle(playX, playY, "PLAY", play, playLit, false)

	// The "=" between the ante and the blind, which are always the same.
	mid := (anteX + blindX) / 2
	for _, dy := range []float64{-4, 4} {
		ui.DrawLine(screen, mid-8, anteY+dy, mid+8, anteY+dy, 3, ui.White)
	}
}

// sign returns -1, 0 or 1 by the sign of v.
func sign(v float64) float64 {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}

// drawPanel draws what the blind and Trips pay down the right of the table,
// with the figures under them. The line for what the player's hand makes is
// lit once it is known.
func (g *Game) drawPanel(screen *ebiten.Image) {
	made := pokerhand.Category(-1)
	switch {
	case g.m.Round == table.River && !g.showing:
		_, s := pokerhand.Best(append(g.m.Hole[:], g.m.Board[:]...))
		made = s.Category()
	case g.over && !g.showing:
		made = g.result.Player.Category()
	}

	y := 24.0
	list := func(title string, pays func(pokerhand.Category) string) {
		ui.DrawText(screen, title, panelX, y, 11, ui.LightGray, ui.AlignStart, ui.AlignStart)
		y += 16
		for c := pokerhand.RoyalFlush; c >= pokerhand.HighCard; c-- {
			p := pays(c)
			if p == "" {
				continue
			}
			ink := color.Color(ui.White)
			if c == made {
				ink = ui.Yellow
			}
			ui.DrawText(screen, c.String(), panelX, y, 12, ink, ui.AlignStart, ui.AlignStart)
			ui.DrawText(screen, p, panelX+panelW, y, 12, ink, ui.AlignEnd, ui.AlignStart)
			y += 16
		}
		y += 10
	}
	list("BLIND PAYS", func(c pokerhand.Category) string {
		switch h := table.BlindPays[c]; {
		case h == 0:
			return ""
		case h%2 == 1:
			return fmt.Sprintf("%d TO 2", h)
		default:
			return fmt.Sprintf("%d TO 1", h/2)
		}
	})
	list("TRIPS PAYS", func(c pokerhand.Category) string {
		if p := table.TripsPays[c]; p > 0 {
			return fmt.Sprintf("%d TO 1", p)
		}
		return ""
	})
	ui.DrawText(screen, "DEALER NEEDS A PAIR", panelX, y, 10, ui.LightGray, ui.AlignStart, ui.AlignStart)
	ui.DrawText(screen, "OR THE ANTE IS HANDED BACK", panelX, y+13, 10, ui.LightGray, ui.AlignStart, ui.AlignStart)

	// What is bet: on the table before the deal, and in play after it.
	bet := g.m.Total()
	switch {
	case g.m.Round != table.NoHand:
		bet = g.m.Stake[table.Trips] + 2*g.m.Stake[table.Ante]
	case g.showing:
		bet = g.result.Staked()
	}
	cx := panelX + panelW/2.0
	ui.DrawText(screen, "CREDITS", cx, 322, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, bank.Dollars(g.credits), cx, 335, 22, ui.LightGreen, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "TOTAL BET", panelX+45, 378, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, bank.Money(bet), panelX+45, 391, 20, ui.Yellow, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "WIN", panelX+panelW-45, 378, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, bank.Money(g.win), panelX+panelW-45, 391, 20, ui.White, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawButtons(screen *ebiten.Image) {
	// The buttons that bet are locked while a hand is played, and those
	// that play it are locked until one is dealt.
	busy := g.showing || g.shuffling
	betting := g.m.Round == table.NoHand && !busy
	deciding := g.m.Round != table.NoHand && !busy
	for i, n := range table.ChipChoices {
		ui.DrawButton(screen, chipButton(i), n == g.m.Chip, !betting, 11, bank.Money(n), "CHIP")
	}
	ui.DrawButton(screen, clearButton, false, !betting, 13, "CLEAR")
	ui.DrawButton(screen, rebetButton, false, !betting || g.m.LastTotal() == 0, 13, "REBET")

	for i, p := range plays(g.m.Round) {
		lit := g.hinting && deciding && g.advice.Choices[g.advice.Best].Antes == p.antes
		ui.DrawButton(screen, playButton(i), lit, !deciding, 13, p.label)
	}
	ui.DrawButton(screen, hintButton, false, !deciding, 13, "HINT")
	ui.DrawCoinButton(screen, coinButton, 13, fmt.Sprintf("COIN +$%d", bank.CoinCredits/100))
	ui.DrawResetButton(screen, resetButton, 13)

	fill, edge, text := color.Color(ui.Green), color.Color(ui.LightGreen), color.Color(ui.White)
	if !betting {
		fill, edge, text = ui.Black, ui.DarkGray, ui.DarkGray
	}
	ui.FillRect(screen, dealButton.X, dealButton.Y, dealButton.W, dealButton.H, fill)
	ui.FrameRect(screen, dealButton.X, dealButton.Y, dealButton.W, dealButton.H, 2, edge)
	ui.DrawText(screen, "DEAL", dealButton.X+dealButton.W/2, dealButton.Y+dealButton.H/2, 30, text, ui.AlignCenter, ui.AlignCenter)

	sound := "ON"
	if !g.snd.On {
		sound = "OFF"
	}
	keys := fmt.Sprintf("SPACE deal   LEFT/RIGHT move   ENTER bet   BACKSPACE take off   1-5 chip   X clear   B rebet   C coin   R reset   S sound %s   ESC menu", sound)
	switch g.m.Round {
	case table.Preflop:
		keys = fmt.Sprintf("K check    4 bet 4x    3 bet 3x    H hint    C coin    R reset    S sound %s    ESC menu", sound)
	case table.Flop:
		keys = fmt.Sprintf("K check    2 bet 2x    H hint    C coin    R reset    S sound %s    ESC menu", sound)
	case table.River:
		keys = fmt.Sprintf("1 bet 1x    F fold    H hint    C coin    R reset    S sound %s    ESC menu", sound)
	}
	ui.DrawText(screen, keys, ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
}
