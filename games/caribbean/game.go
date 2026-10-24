// Package caribbean is Caribbean Stud: a hand of five cards played against
// the dealer's, one of whose cards is face up, for pretend credits from the
// bank. Its rules are in the table package.
package caribbean

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/caribbean/table"
	"videocasino/pokerhand"
	"videocasino/shuffle"
	"videocasino/ui"
)

const (
	cardScale = 0.7
	cardW     = ui.CardW * cardScale
	cardH     = ui.CardH * cardScale
	cardStep  = 92 // how far each card of a hand sits from the one before: clear of it

	// The table takes up the left of the screen: the dealer's hand above
	// the player's, and the bets under them. What the raise pays is listed
	// down the right, with the figures.
	handX    = 300 // the middle of the hands
	dealerY  = 42
	playerY  = 186
	messageY = 314
	betsY    = 338
	betsH    = 74
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

// The table: a deep burgundy with an ivory edge, so that the game has a
// look of its own.
var (
	felt  = color.RGBA{88, 12, 32, 255}
	ivory = color.RGBA{236, 226, 198, 255}
)

// shuffleLength is the shuffle shown before every hand: a single deck, so
// the short one.
const shuffleLength = shuffle.Short

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

	m *table.Machine

	// The deck is shuffled before every hand, and that is shown before the
	// cards are dealt.
	shuffling    bool
	shuffleStart int // the tick it began at

	// The player's cards are dealt one by one, then the dealer's card that
	// is face up; the dealer's others are turned over one by one once the
	// hand is raised. The machine has settled the hand by then, and the
	// result is shown once the last card is up.
	showing     bool // cards are being dealt or turned over
	showAt      int  // the tick at which the next is shown
	playerShown int  // how many of the player's cards are on show
	upShown     bool // the dealer's card that is face up is on show
	dealerShown int  // how many of the dealer's others are face up

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
	g := &Game{snd: snd, bank: b}
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
	g.message = "PLACE YOUR BET"
	g.chipSound = ui.Tone(1480, 22, 0.14, true)
	g.riffleSound = shuffle.Sound()

	var s table.Saved
	if b.Load(saveName, &s) {
		g.m.Restore(s)
		g.resume()
	}
	g.credits = b.Credits
	return g
}

// resume puts back on screen a hand that was still to be played when the
// game was last closed.
func (g *Game) resume() {
	if g.m.Dealt {
		g.playerShown, g.upShown = table.HandSize, true
		g.message = "HAND TO FINISH - RAISE OR FOLD"
	}
}

// The places of the ante and of the raise beside it, which is made by
// raising the hand.
var (
	anteBox  = ui.Button{X: 125, Y: betsY, W: 170, H: betsH}
	raiseBox = ui.Button{X: 310, Y: betsY, W: 170, H: betsH}
)

// The buttons under the table.
func chipButton(i int) ui.Button {
	return ui.Button{X: 30 + float64(i)*76, Y: rowAY, W: 70, H: buttonH}
}

var (
	clearButton = ui.Button{X: 420, Y: rowAY, W: 80, H: buttonH}
	rebetButton = ui.Button{X: 506, Y: rowAY, W: 84, H: buttonH}
	dealButton  = ui.Button{X: 600, Y: rowAY, W: 170, H: rowBY + buttonH - rowAY} // as tall as two rows
	raiseButton = ui.Button{X: 30, Y: rowBY, W: 100, H: buttonH}
	foldButton  = ui.Button{X: 136, Y: rowBY, W: 100, H: buttonH}
	hintButton  = ui.Button{X: 242, Y: rowBY, W: 80, H: buttonH}
	coinButton  = ui.Button{X: 334, Y: rowBY, W: 170, H: buttonH}
	resetButton = ui.Button{X: 510, Y: rowBY, W: 80, H: buttonH}
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
	hit := func(b ui.Button) bool { return click && b.Hit(x, y) }

	switch {
	case hit(coinButton), ui.Pressed(ebiten.KeyC):
		g.insertCoin()
		return
	case hit(resetButton), ui.Pressed(ebiten.KeyR):
		g.askReset()
		return
	}

	// With a hand dealt there is only to raise it or fold it.
	if g.m.Dealt {
		switch {
		case hit(raiseButton), ui.Pressed(ebiten.KeyP, ebiten.KeyDigit1, ebiten.KeyNumpad1):
			g.raise()
		case hit(foldButton), ui.Pressed(ebiten.KeyF, ebiten.KeyDigit2, ebiten.KeyNumpad2):
			g.fold()
		case hit(hintButton), ui.Pressed(ebiten.KeyH):
			g.showHint()
		}
		return
	}

	// A click on the ante puts a chip down, and a click of the other button
	// takes one off.
	if anteBox.Hit(x, y) {
		switch {
		case click:
			g.place()
		case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight):
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
	}
}

// clearWin takes the last hand off the table, once the player has moved on
// from it.
func (g *Game) clearWin() {
	g.win = 0
	g.over = false
	g.hinting = false
	g.playerShown, g.dealerShown, g.upShown = 0, 0, false
}

func (g *Game) setChip(n int) {
	g.m.Chip = n
	g.snd.Play(g.snd.Click)
	g.save()
}

const outOfCredits = "OUT OF CREDITS - INSERT A COIN OR RESET"

// place puts a chip on the ante. Like the rest of the betting, it does
// nothing while a hand is waiting to be played.
func (g *Game) place() {
	if g.m.Dealt {
		return
	}
	g.clearWin()
	if !g.m.Place() {
		switch {
		case g.bank.Credits == 0:
			g.message = outOfCredits
		case g.m.Ante+g.m.Chip > table.MaxBet:
			g.message = fmt.Sprintf("NO MORE THAN %s ON THE ANTE", bank.Money(table.MaxBet))
		default:
			g.message = "NOT ENOUGH CREDITS - THE BANK MUST COVER THE RAISE TOO"
		}
		return
	}
	g.message = "PLACE YOUR BET"
	g.snd.Play(g.chipSound)
	g.save()
}

// remove takes a chip off the ante.
func (g *Game) remove() {
	if g.m.Dealt || !g.m.Remove() {
		return
	}
	g.clearWin()
	g.message = "PLACE YOUR BET"
	g.snd.Play(g.chipSound)
	g.save()
}

// clear takes the chips off the table.
func (g *Game) clear() {
	if g.m.Dealt {
		return
	}
	g.m.Clear()
	g.clearWin()
	g.message = "PLACE YOUR BET"
	g.snd.Play(g.snd.Click)
	g.save()
}

// rebet puts the ante of the last hand back on the table.
func (g *Game) rebet() {
	if g.m.Dealt {
		return
	}
	g.clearWin()
	if !g.m.Rebet() {
		g.message = "NOT ENOUGH CREDITS TO BET THE SAME AGAIN"
		if g.m.Last == 0 {
			g.message = "THERE IS NO BET TO MAKE AGAIN YET"
		}
		return
	}
	g.message = "THE SAME BET AGAIN - PRESS DEAL"
	g.snd.Play(g.chipSound)
	g.save()
}

// askReset asks the player to make sure before the credits are reset.
func (g *Game) askReset() {
	g.confirm.Ask("RESET CREDITS TO "+bank.Money(bank.StartCredits)+"?", "YOU HAVE "+bank.Dollars(g.bank.Credits))
}

// reset puts the bank's credits back to the starting amount. The ante and
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
// bet of the last hand is made again first. If no hand can be dealt there
// is no shuffle to show: it says why.
func (g *Game) pressDeal() {
	if g.m.Dealt {
		return
	}
	if g.m.Ante == 0 {
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

// deal has the machine deal the player a hand and the dealer's card that
// is face up, then shows them one by one. With too few credits nothing is
// dealt: the bank is every game's, so it is for the player to insert a coin
// or reset it.
func (g *Game) deal() {
	if g.m.Dealt {
		return
	}
	if g.m.Ante == 0 {
		g.m.Rebet()
	}
	if !g.m.Deal() {
		g.clearWin()
		switch {
		case g.bank.Credits == 0:
			g.message = outOfCredits
		case g.m.Ante == 0 && g.m.Last == 0:
			g.message = "PUT A CHIP ON THE ANTE BEFORE YOU DEAL"
		default:
			g.message = "NOT ENOUGH CREDITS - THE BANK MUST COVER THE RAISE TOO"
		}
		return
	}
	g.save()
	g.clearWin()
	g.showing = true
	g.showAt = g.ticks + dealTicks
	g.credits = g.bank.Credits
	g.message = ""
}

// raise makes the raise and has the machine deal the rest of the dealer's
// hand and settle against it, then turns the dealer's cards over one by
// one.
func (g *Game) raise() {
	r, ok := g.m.Raise()
	if !ok {
		g.message = "NOT ENOUGH CREDITS FOR THE RAISE - INSERT A COIN OR FOLD"
		return
	}
	g.save()
	g.result = r
	g.hinting = false
	g.showing = true
	g.showAt = g.ticks + dealTicks
	g.credits = g.bank.Credits - r.Paid
	g.message = ""
	g.snd.Play(g.chipSound)
}

// fold gives the hand up.
func (g *Game) fold() {
	r, ok := g.m.Fold()
	if !ok {
		return
	}
	g.save()
	g.result = r
	g.hinting = false
	g.over = true
	g.message = "YOU FOLD - THE ANTE IS LOST"
	g.snd.Play(g.snd.Click)
}

// showHint weighs up the hand against the dealer's card and marks whether
// it is better raised or folded.
func (g *Game) showHint() {
	g.advice = table.Advise(g.m.Hand, g.m.Up)
	g.hinting = true
	g.snd.Play(g.snd.Click)
	ante := float64(g.m.Stake)
	raised, folded := bank.Worth(g.advice.Raise*ante), bank.Worth(-ante)
	if g.advice.Best {
		g.message = fmt.Sprintf("BEST TO RAISE - WORTH %s ON AVERAGE, AGAINST %s FOLDED", raised, folded)
	} else {
		g.message = fmt.Sprintf("BEST TO FOLD, FOR %s - RAISED IT IS WORTH %s ON AVERAGE", folded, raised)
	}
}

// show carries on by a tick the dealing of the player's cards and the
// dealer's that is face up, or the turning over of the dealer's others, and
// says how things stand once the last is shown.
func (g *Game) show(skip bool) {
	if skip {
		if g.m.Dealt {
			g.playerShown, g.upShown = table.HandSize, true
		} else {
			g.dealerShown = table.HandSize - 1
		}
		g.snd.Play(g.snd.Card)
	} else {
		if g.ticks < g.showAt {
			return
		}
		g.showAt = g.ticks + dealTicks
		turned := true
		switch {
		case g.m.Dealt && g.playerShown < table.HandSize:
			g.playerShown++
		case g.m.Dealt && !g.upShown:
			g.upShown = true
		case !g.m.Dealt && g.dealerShown < table.HandSize-1:
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
	if g.m.Dealt {
		// The hand is dealt: it is for the player to raise or fold.
		g.message = g.m.Hand.Score().String() + " - RAISE OR FOLD"
		return
	}
	g.land()
}

// land shows how the hand went against the dealer's, now that the dealer's
// cards are up.
func (g *Game) land() {
	r := &g.result
	g.over = true
	g.win = r.Paid
	dealer := r.Dealer.Score().String()
	switch r.Outcome {
	case table.NotQualified:
		g.message = fmt.Sprintf("DEALER HAS %s AND DOES NOT QUALIFY - YOU WIN %s", dealer, bank.Money(r.Paid))
	case table.Won:
		g.message = fmt.Sprintf("YOU BEAT THE DEALER'S %s - YOU WIN %s", dealer, bank.Money(r.Paid))
	case table.Pushed:
		g.message = "THE HANDS TIE - YOUR BETS ARE HANDED BACK"
	default:
		g.message = "DEALER WINS WITH " + dealer
	}
	switch {
	case r.Outcome == table.Won && table.RaisePays[r.Player.Category()] >= table.RaisePays[pokerhand.Straight]:
		g.snd.Play(g.snd.BigWin)
	case r.Outcome == table.Won || r.Outcome == table.NotQualified:
		g.snd.Play(g.snd.Win)
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(felt)
	ui.DrawBorder(screen, ivory)

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
	ui.DrawText(screen, "CARIBBEAN STUD", ui.ScreenW/2, 70, 48, ui.Yellow, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "POKER AGAINST THE DEALER", ui.ScreenW/2, 128, 20, ui.White, ui.AlignCenter, ui.AlignStart)

	g.menu.Draw(screen)
	ui.DrawText(screen, "ARROWS move    ENTER or NUMBER choose    ESC main menu", ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "MAKE YOUR CHOICE", ui.ScreenW/2, 350, 24, ui.LightCyan, ui.AlignCenter, ui.AlignStart)

	// The best hand there is, for luck.
	const scale = 0.62
	for i := range table.HandSize {
		c := cards.New(cards.Ten+cards.Rank(i), cards.Spades)
		x := ui.ScreenW/2 + (float64(i)-2.5)*(ui.CardW*scale+10) + 5
		ui.DrawCard(screen, c, x, 420, scale)
	}
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	ui.DrawText(screen, "HOW TO PLAY", ui.ScreenW/2, 22, 30, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	p := table.RaisePays
	lines := []string{
		"Put chips on the ANTE, then deal. You are dealt five cards and the dealer",
		"five, one of them face up. FOLD and lose the ante, or RAISE for twice",
		"the ante, and the dealer's other cards are turned over.",
		"The dealer needs an ace and a king or better to qualify. If the dealer",
		"does not, the ante pays 1 to 1 and the raise is handed back. If the dealer",
		"does, the better hand wins both bets, and a tie hands them back. The ante",
		fmt.Sprintf("pays 1 to 1 and the raise by your hand: %d to 1 for a royal flush, %d for a", p[pokerhand.RoyalFlush], p[pokerhand.StraightFlush]),
		fmt.Sprintf("straight flush, %d for four of a kind, %d for a full house, %d for a flush,", p[pokerhand.FourOfAKind], p[pokerhand.FullHouse], p[pokerhand.Flush]),
		fmt.Sprintf("%d for a straight, %d for three of a kind, %d for two pair, else %d to 1.", p[pokerhand.Straight], p[pokerhand.ThreeOfAKind], p[pokerhand.TwoPair], p[pokerhand.Pair]),
		"Raise any pair or better, and fold anything less than an ace and a king.",
		"With an ace and a king it depends on the dealer's card: HINT tells you.",
		"The deck is shuffled before every hand. The bank must cover the raise.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2-285, 60+float64(i)*18, 15, ui.White, ui.AlignStart, ui.AlignStart)
	}

	keys := [][2]string{
		{"SPACE", "shuffle and deal"},
		{"ENTER / BACKSPACE", "put a chip on the ante, or take one off"},
		{"1 to 5", "choose a chip"},
		{"X, B", "clear the table; make the last bet again"},
		{"P or 1", "raise"},
		{"F or 2", "fold"},
		{"H", "hint: is the hand better raised or folded"},
		{"C, R", "insert a coin; reset your credits"},
		{"S", "sound on or off"},
		{"ESC", "back to the menu"},
	}
	for i, k := range keys {
		y := 288 + float64(i)*18
		ui.DrawText(screen, k[0], ui.ScreenW/2-70, y, 14, ui.LightRed, ui.AlignEnd, ui.AlignStart)
		ui.DrawText(screen, k[1], ui.ScreenW/2-45, y, 14, ui.White, ui.AlignStart, ui.AlignStart)
	}
	ui.DrawText(screen, "With the mouse, click the ante to put a chip down and right click to take it off.", ui.ScreenW/2, 478, 13, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 546, 20, ui.LightCyan, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawAbout(screen *ebiten.Image) {
	ui.DrawText(screen, "ABOUT THIS GAME", ui.ScreenW/2, 50, 36, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	lines := []string{
		"Caribbean Stud is written in Go with Ebitengine.",
		"",
		"Each hand is dealt from one deck, freshly shuffled.",
		"",
		"Played perfectly, the ante and the raise",
		fmt.Sprintf("pay back %.2f%% of what is bet on them,", table.Return()*100),
		fmt.Sprintf("and the ante loses %.2f%% of itself.", table.Edge()*100),
		fmt.Sprintf("That is to raise %.1f%% of hands.", table.Raised()*100),
		"",
		"It is played with pretend credits only.",
		"No real money goes in or comes out.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2, 124+float64(i)*28, 18, ui.White, ui.AlignCenter, ui.AlignStart)
	}
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 500, 20, ui.LightCyan, ui.AlignCenter, ui.AlignStart)
}

// cardX returns where the ith card of a hand lies, with the hand's middle
// at handX.
func cardX(i int) float64 {
	return handX - (cardW+float64(table.HandSize-1)*cardStep)/2 + float64(i)*cardStep
}

func (g *Game) drawPlay(screen *ebiten.Image) {
	// The hand on show is the one being played, or the one last finished.
	player, dealer := g.result.Player, g.result.Dealer
	if g.m.Dealt {
		player = g.m.Hand
		dealer = table.Hand{table.HandSize - 1: g.m.Up}
	}
	done := g.over && !g.showing

	// While the deck is shuffled the table is clear.
	if g.shuffling {
		shuffleLength.Draw(screen, g.ticks-g.shuffleStart, handX, (dealerY+playerY+cardH)/2, cardScale)
	} else {
		// The dealer's last card is the one face up.
		for i := range table.HandSize {
			up := i < g.dealerShown || (i == table.HandSize-1 && g.upShown)
			if up {
				ui.DrawCard(screen, dealer[i], cardX(i), dealerY, cardScale)
			} else {
				ui.DrawCardBack(screen, cardX(i), dealerY, cardScale)
			}
			if i < g.playerShown {
				ui.DrawCard(screen, player[i], cardX(i), playerY, cardScale)
			} else {
				ui.DrawCardBack(screen, cardX(i), playerY, cardScale)
			}
		}
	}

	label := "DEALER"
	if g.dealerShown == table.HandSize-1 {
		label = "DEALER: " + dealer.Score().String()
		if done && g.result.Outcome == table.NotQualified {
			label += " - DOES NOT QUALIFY"
		}
	}
	if !g.shuffling {
		ui.DrawText(screen, label, handX, dealerY-18, 13, ui.White, ui.AlignCenter, ui.AlignStart)
	}

	label, ink := "YOUR HAND", color.Color(ui.White)
	held := g.playerShown == table.HandSize
	if held {
		label = "YOUR HAND: " + player.Score().String()
		if done && (g.result.Outcome == table.Won || g.result.Outcome == table.NotQualified) {
			ink = ui.Yellow
		}
	}
	if !g.shuffling {
		ui.DrawText(screen, label, handX, playerY-18, 13, ink, ui.AlignCenter, ui.AlignStart)
	}

	ui.DrawText(screen, g.message, handX, messageY, 13, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	g.drawBets(screen)
	made := -1
	if held {
		made = int(player.Category())
	}
	g.drawPanel(screen, made)

	// The buttons that bet are locked while a hand is played, and those
	// that play it are locked until one is dealt.
	busy := g.showing || g.shuffling
	betting := !g.m.Dealt && !busy
	deciding := g.m.Dealt && !busy
	for i, n := range table.ChipChoices {
		ui.DrawButton(screen, chipButton(i), n == g.m.Chip, !betting, 11, bank.Money(n), "CHIP")
	}
	ui.DrawButton(screen, clearButton, false, !betting, 13, "CLEAR")
	ui.DrawButton(screen, rebetButton, false, !betting || g.m.Last == 0, 13, "REBET")
	ui.DrawButton(screen, raiseButton, g.hinting && g.advice.Best, !deciding, 14, "RAISE")
	ui.DrawButton(screen, foldButton, g.hinting && !g.advice.Best, !deciding, 14, "FOLD")
	ui.DrawButton(screen, hintButton, false, !deciding, 13, "HINT")

	fill, edge, text := color.Color(ui.Green), color.Color(ui.LightGreen), color.Color(ui.White)
	if !betting {
		fill, edge, text = ui.Black, ui.DarkGray, ui.DarkGray
	}
	ui.FillRect(screen, dealButton.X, dealButton.Y, dealButton.W, dealButton.H, fill)
	ui.FrameRect(screen, dealButton.X, dealButton.Y, dealButton.W, dealButton.H, 2, edge)
	ui.DrawText(screen, "DEAL", dealButton.X+dealButton.W/2, dealButton.Y+dealButton.H/2, 30, text, ui.AlignCenter, ui.AlignCenter)

	coinLabel := fmt.Sprintf("COIN +$%d", bank.CoinCredits/100)
	ui.DrawCoinButton(screen, coinButton, 13, coinLabel)
	ui.DrawResetButton(screen, resetButton, 13)

	sound := "ON"
	if !g.snd.On {
		sound = "OFF"
	}
	hint := fmt.Sprintf("SPACE deal   ENTER bet   BACKSPACE take off   1-5 chip   X clear   B rebet   C coin   R reset   S sound %s   ESC menu", sound)
	if g.m.Dealt {
		hint = fmt.Sprintf("P or 1 raise    F or 2 fold    H hint    C coin    R reset    S sound %s    ESC menu", sound)
	}
	ui.DrawText(screen, hint, ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
}

// drawBets draws the ante and the raise, and the chips on them.
func (g *Game) drawBets(screen *ebiten.Image) {
	// While a hand is played the bets on show are the ones it was dealt
	// for. Once it is over the ones that were paid are left on show until
	// the player moves on.
	ante, raise := g.m.Ante, 0
	var anteLit, raiseLit bool
	inHand := g.m.Dealt || g.showing
	done := g.over && !g.showing && g.m.Ante == 0
	switch {
	case g.m.Dealt:
		ante = g.m.Stake
	case inHand:
		ante, raise = g.result.Ante, g.result.Raise
	case done:
		r := &g.result
		ante, raise = 0, 0
		flash := g.ticks/15%2 == 0
		if r.Paid > 0 {
			won := r.Outcome == table.Won || r.Outcome == table.NotQualified
			ante, anteLit = r.Ante, flash && won
			raise, raiseLit = r.Raise, flash && r.Outcome == table.Won
		}
	}

	box := func(b ui.Button, name, note string, amount int, lit bool) {
		ui.FrameRect(screen, b.X, b.Y, b.W, b.H, 1, ui.LightGray)
		cx := b.X + b.W/2
		ui.DrawText(screen, name, cx, b.Y+7, 15, ui.White, ui.AlignCenter, ui.AlignStart)
		ui.DrawText(screen, note, cx, b.Y+27, 10, ui.LightGray, ui.AlignCenter, ui.AlignStart)
		if lit {
			ui.FrameRect(screen, b.X+1, b.Y+1, b.W-2, b.H-2, 4, ui.Yellow)
		}
		if amount > 0 {
			ui.DrawChip(screen, cx, b.Y+b.H-17, amount, table.ChipChoices, lit)
		}
	}
	box(anteBox, "ANTE", "AGAINST THE DEALER", ante, anteLit)
	if !inHand && !g.shuffling {
		ui.FrameRect(screen, anteBox.X+1, anteBox.Y+1, anteBox.W-2, anteBox.H-2, 2, ui.White)
	}
	box(raiseBox, "RAISE", "TWICE THE ANTE", raise, raiseLit)
}

// drawPanel draws what the raise pays down the right of the table, with
// the figures under it. The line for what the player's hand makes is lit;
// made is -1 if there is no hand to show.
func (g *Game) drawPanel(screen *ebiten.Image, made int) {
	y := 24.0
	ui.DrawText(screen, "RAISE PAYS", panelX, y, 11, ui.LightGray, ui.AlignStart, ui.AlignStart)
	y += 16
	for c := pokerhand.RoyalFlush; c >= pokerhand.HighCard; c-- {
		ink := color.Color(ui.White)
		if int(c) == made {
			ink = ui.Yellow
		}
		ui.DrawText(screen, c.String(), panelX, y, 12, ink, ui.AlignStart, ui.AlignStart)
		ui.DrawText(screen, fmt.Sprintf("%d TO 1", table.RaisePays[c]), panelX+panelW, y, 12, ink, ui.AlignEnd, ui.AlignStart)
		y += 16
	}
	y += 10
	ui.DrawText(screen, "DEALER PLAYS WITH AN", panelX, y, 10, ui.LightGray, ui.AlignStart, ui.AlignStart)
	ui.DrawText(screen, "ACE AND A KING OR BETTER", panelX, y+13, 10, ui.LightGray, ui.AlignStart, ui.AlignStart)

	// What is bet: on the table before the deal, and in play after it.
	bet := g.m.Ante
	switch {
	case g.m.Dealt:
		bet = g.m.Stake
	case g.showing:
		bet = g.result.Staked()
	}
	stat := func(i int, label, v string, c color.Color) {
		sy := 254 + float64(i)*50
		ui.DrawText(screen, label, panelX+panelW/2, sy, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
		ui.DrawText(screen, v, panelX+panelW/2, sy+13, 22, c, ui.AlignCenter, ui.AlignStart)
	}
	stat(0, "CREDITS", bank.Dollars(g.credits), ui.LightGreen)
	stat(1, "TOTAL BET", bank.Money(bet), ui.Yellow)
	stat(2, "WIN", bank.Money(g.win), ui.White)
}
