// Package slot is the Slot Machine: a five-reel slot machine, played for
// pretend credits from the bank. Its rules are in the machine package.
package slot

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"videocasino/bank"
	"videocasino/games/slot/machine"
	"videocasino/ui"
)

const (
	reelsX  = 34
	reelsY  = 90
	reelW   = 100
	reelGap = 6
	rowH    = 90
	reelsW  = machine.Reels*reelW + (machine.Reels-1)*reelGap
	reelsH  = machine.Rows * rowH

	reelScale = 0.9 // symbol size that fits a reel cell

	panelX = 582
	panelW = 198
	panelH = 354 // the paytable runs down beside the reels and the figures

	// The two rows of buttons under the reels.
	betRowY  = 448
	lineRowY = 494
	buttonH  = 40
)

type scene int

const (
	sceneMenu scene = iota
	sceneHelp
	sceneAbout
	scenePlay
)

// reel is the animation state of one reel. pos is the strip index on the
// middle row; it is fractional while the reel is moving.
type reel struct {
	pos      float64
	from     float64
	dist     float64
	tick     int
	dur      int
	to       int // the strip index it will stop at
	spinning bool
}

type Game struct {
	scene scene
	ticks int
	snd   *ui.Sounds
	bank  *bank.Bank
	menu  *ui.Menu

	// The sounds that are this game's own.
	startSound, stopSound []byte

	// What the game pays back takes a moment to work out, so that is done
	// in the background while the program starts. Until it is ready this is
	// nil.
	rtp atomic.Pointer[float64]

	m        *machine.Machine
	reels    [machine.Reels]reel
	result   machine.Result
	spinning bool
	credits  int // credits on display; a win is added once the reels stop
	win      int
	message  string
	confirm  ui.Confirm // asks before the credits are reset

	stopTick  int // when the reels last stopped, to time the win display
	showLines int // ticks left to show the lines being played

	free freeSpins
	link linkView
	spun bool // a spin has been played, so the chips showing were dealt

	// freeView is set while the reels on screen are those of a free game:
	// reels 1 and 5 on their free-game strips and a giant symbol between.
	freeView bool

	auto bool // spins are being played automatically

	saveFailed bool
}

// freeSpins tracks a run of free games, from the spin that wins them until
// the last one has been played.
type freeSpins struct {
	active bool
	total  int // spins awarded so far, counting any won along the way
	played int
	win    int // credits won by the free games
}

// New returns the game, playing from the given bank and picking up whatever
// it last kept there.
func New(b *bank.Bank, snd *ui.Sounds) *Game {
	g := &Game{snd: snd, bank: b}
	g.startSound = append(ui.Tone(330, 40, 0.2, true), ui.Tone(440, 60, 0.2, true)...)
	g.stopSound = ui.Tone(110, 70, 0.4, true)
	g.m = machine.NewRandom(b)
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
	g.message = "CHOOSE YOUR BET AND PRESS SPIN"

	var s machine.Saved
	if b.Load(saveName, &s) {
		g.m.Restore(s)
		g.resume()
	}
	g.credits = b.Credits

	go func() {
		rtp := machine.RTP(len(machine.Paylines))
		g.rtp.Store(&rtp)
	}()
	return g
}

// resume puts back on screen a feature that was still being played when
// the game was last closed.
func (g *Game) resume() {
	if n := g.m.FreeSpins; n > 0 {
		g.free = freeSpins{active: true, total: n}
		g.message = fmt.Sprintf("%d FREE GAMES TO PLAY", n)
	}
	if g.m.Link != nil {
		g.link = linkView{
			show:  true,
			timer: linkIntroTicks,
			state: *g.m.Link,
			total: g.m.LinkTotal(),
		}
		g.message = "LINK FEATURE TO FINISH"
	}
}

// clearWin takes the last spin's win off the display.
func (g *Game) clearWin() {
	g.result.Wins = nil
	g.result.Scatters = 0
	g.link = linkView{}
	g.win = 0
	g.message = ""
}

// The bet and line buttons sit under the reels, one per reel column.
func betButton(i int) ui.Button {
	return ui.Button{X: reelsX + float64(i)*(reelW+reelGap), Y: betRowY, W: reelW, H: buttonH}
}

func lineButton(i int) ui.Button {
	return ui.Button{X: reelsX + float64(i)*(reelW+reelGap), Y: lineRowY, W: reelW, H: buttonH}
}

// The other four buttons sit under the paytable, in line with the two rows:
// spin and auto spin beside the bets, coin and reset beside the lines.
const (
	sideW = 64                       // auto spin and reset
	mainW = panelW - sideW - reelGap // spin and coin
	sideX = panelX + mainW + reelGap
)

var (
	spinButton  = ui.Button{X: panelX, Y: betRowY, W: mainW, H: buttonH}
	autoButton  = ui.Button{X: sideX, Y: betRowY, W: sideW, H: buttonH}
	coinButton  = ui.Button{X: panelX, Y: lineRowY, W: mainW, H: buttonH}
	resetButton = ui.Button{X: sideX, Y: lineRowY, W: sideW, H: buttonH}
)

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
	// Auto spin can be turned on or off at any time, even in mid spin.
	if ui.Pressed(ebiten.KeyA) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && autoButton.Hit(ebiten.CursorPosition()) {
		g.auto = !g.auto
		g.snd.Play(g.snd.Click)
	}
	if ui.Pressed(ebiten.KeyEscape) {
		g.auto = false
	}
	if g.spinning {
		g.advanceReels()
		return
	}
	// The bank may have changed while the game was not being played. A
	// link feature shows what it paid only once its last respin has stopped.
	if !g.linkBusy() {
		g.credits = g.bank.Credits
	}
	if g.showLines > 0 {
		g.showLines--
	}
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	x, y := ebiten.CursorPosition()
	if ui.Pressed(ebiten.KeyC) || click && coinButton.Hit(x, y) {
		g.insertCoin()
	}

	// A link feature runs to its end before anything else can happen.
	if g.linkBusy() {
		g.updateLink(click && spinButton.Hit(x, y))
		return
	}

	// Free spins play themselves at the bet that won them, after a pause to
	// show the last result. Pressing spin skips the pause.
	if g.m.FreeSpins > 0 {
		pause := 50
		switch {
		case g.link.show, g.result.FreeSpinsWon > 0:
			pause = 180
		case g.win > 0:
			pause = 110
		}
		switch {
		case ui.Pressed(ebiten.KeyEscape):
			g.scene = sceneMenu
		case g.ticks-g.stopTick > pause, click && spinButton.Hit(x, y),
			ui.Pressed(ebiten.KeySpace, ebiten.KeyEnter, ebiten.KeyNumpadEnter):
			g.spin()
		}
		return
	}

	// Auto spin plays the next spin by itself, after a pause to show the
	// last result, until it is turned off, a feature is won or the credits
	// run short.
	if g.auto {
		pause := 35
		switch {
		case g.link.show:
			pause = 200
		case g.win > 0:
			pause = 100
		}
		switch {
		case g.bank.Credits < g.m.TotalBet():
			g.auto = false
			g.message = "AUTO SPIN STOPPED - NOT ENOUGH CREDITS"
		case g.ticks-g.stopTick > pause:
			g.spin()
			return
		}
	}

	if click {
		for i, n := range machine.BetChoices {
			if betButton(i).Hit(x, y) {
				g.setBet(n)
			}
		}
		for i, n := range machine.LineChoices {
			if lineButton(i).Hit(x, y) {
				g.setLines(n)
			}
		}
		if spinButton.Hit(x, y) {
			g.spin()
		}
		if resetButton.Hit(x, y) {
			g.askReset()
		}
	}

	switch {
	case ui.Pressed(ebiten.KeyEscape):
		g.scene = sceneMenu
	case ui.Pressed(ebiten.KeyArrowUp):
		g.setBet(ui.Step(machine.BetChoices, g.m.BetPerLine, 1))
	case ui.Pressed(ebiten.KeyArrowDown):
		g.setBet(ui.Step(machine.BetChoices, g.m.BetPerLine, -1))
	case ui.Pressed(ebiten.KeyArrowRight):
		g.setLines(ui.Step(machine.LineChoices, g.m.Lines, 1))
	case ui.Pressed(ebiten.KeyArrowLeft):
		g.setLines(ui.Step(machine.LineChoices, g.m.Lines, -1))
	case ui.Pressed(ebiten.KeyR):
		g.askReset()
	case ui.Pressed(ebiten.KeyM):
		g.maxBet()
	case ui.Pressed(ebiten.KeySpace, ebiten.KeyEnter, ebiten.KeyNumpadEnter):
		g.spin()
	}
}

func (g *Game) setBet(n int) {
	g.m.BetPerLine = n
	g.clearWin()
	g.snd.Play(g.snd.Click)
	g.save()
}

func (g *Game) setLines(n int) {
	g.m.Lines = n
	g.clearWin()
	g.showLines = 120
	g.snd.Play(g.snd.Click)
	g.save()
}

// maxBet plays every line at the highest bet per line the credits cover.
func (g *Game) maxBet() {
	lines := len(machine.Paylines)
	bet := machine.BetChoices[0]
	for _, n := range machine.BetChoices {
		if lines*n <= g.bank.Credits {
			bet = n
		}
	}
	g.setLines(lines)
	g.m.BetPerLine = bet
	g.save()
}

const outOfCredits = "OUT OF CREDITS - INSERT A COIN OR RESET"

// askReset asks the player to make sure before the credits are reset.
func (g *Game) askReset() {
	g.confirm.Ask("RESET CREDITS TO "+bank.Money(bank.StartCredits)+"?", "YOU HAVE "+bank.Dollars(g.bank.Credits))
}

// reset puts the bank's credits back to the starting amount. The bet and
// the jackpot pots are left as they are.
func (g *Game) reset() {
	g.bank.Reset()
	g.credits = g.bank.Credits
	g.auto = false
	g.clearWin()
	g.message = "CREDITS RESET - PRESS SPIN"
	g.snd.Play(g.snd.Click)
	g.save()
}

// insertCoin adds credits to the bank, so play can go on after they run
// out. A coin that would go over the limit is refused.
func (g *Game) insertCoin() {
	if !g.bank.InsertCoin() {
		g.message = fmt.Sprintf("COIN REFUSED - LIMIT IS %d CREDITS", bank.CoinLimit)
		return
	}
	g.credits = g.bank.Credits
	g.message = fmt.Sprintf("COIN INSERTED - %d CREDITS ADDED", bank.CoinCredits)
	g.snd.Play(g.snd.Coin)
	g.save()
}

// bigOrder is the order the giant symbols pass in as the middle reels turn
// in a free game. It is for show: the giant symbol is dealt, not read off a
// strip.
var bigOrder = []machine.Symbol{
	machine.King, machine.Cherry, machine.Ten, machine.Link, machine.Queen,
	machine.Bell, machine.Ace, machine.Wild, machine.Jack, machine.Plum,
	machine.Scatter, machine.Dollar,
}

// strip is the strip a reel is turning on: its own, or in a free game the
// free-game strip for reels 1 and 5 and the giant symbols for the one in
// the middle. The other two middle reels are hidden behind the giant
// symbol and have none.
func (g *Game) strip(i int) []machine.Symbol {
	switch {
	case !g.freeView:
		return machine.Strips[i]
	case i == machine.BigReel:
		return bigOrder
	}
	return machine.FreeStrips[i]
}

// spin decides the outcome straight away, then sets each reel turning
// towards its stop. Reels further right travel further and stop later.
// With too few credits nothing is spun: the bank is every game's, so it is
// for the player to insert a coin or reset it.
func (g *Game) spin() {
	res, ok := g.m.Spin()
	if !ok {
		g.clearWin()
		g.message = "NOT ENOUGH CREDITS - INSERT A COIN OR BET LESS"
		if g.bank.Credits == 0 {
			g.message = outOfCredits
		}
		return
	}
	// The machine has already settled the spin, so it is safe to save now
	// even though the reels have yet to show it.
	g.save()
	if res.Free {
		g.free.played++
	} else {
		g.free = freeSpins{}
	}
	g.clearWin()
	g.result = res
	g.spun = true
	g.spinning = true
	g.freeView = res.Free
	g.showLines = 0
	g.credits = g.bank.Credits - res.Win
	g.snd.Play(g.startSound)

	for i := range g.reels {
		r := &g.reels[i]
		strip := g.strip(i)
		if strip == nil {
			r.spinning = false
			continue
		}
		r.to = res.Stops[i]
		if g.freeView && i == machine.BigReel {
			r.to = slices.Index(bigOrder, res.Big)
		}
		n := len(strip)
		// A reel may have been left on a longer strip than the one it is
		// about to turn on.
		r.pos = math.Mod(r.pos, float64(n))
		ahead := ((r.to-int(r.pos))%n + n) % n
		r.from = r.pos
		r.dist = float64(n*(i+1) + ahead)
		if g.freeView && i == machine.BigReel {
			// The giant symbols are three rows high, so fewer need to pass.
			r.dist = float64(n + ahead)
		}
		r.tick = 0
		r.dur = 60 + 25*i
		r.spinning = true
	}
}

func (g *Game) advanceReels() {
	for i := range g.reels {
		r := &g.reels[i]
		if !r.spinning {
			continue
		}
		r.tick++
		if r.tick >= r.dur {
			r.pos = float64(r.to)
			r.spinning = false
			g.snd.Play(g.stopSound)
			continue
		}
		// Ease out: fast at first, slowing into the stop.
		u := float64(r.tick) / float64(r.dur)
		r.pos = r.from + r.dist*(1-(1-u)*(1-u))
	}
	if g.reels[machine.Reels-1].spinning {
		return
	}

	g.spinning = false
	g.stopTick = g.ticks
	g.credits = g.bank.Credits
	g.win = g.result.Win
	if g.result.Free {
		g.free.win += g.win
	}
	if won := g.result.FreeSpinsWon; won > 0 {
		g.free.active = true
		g.free.total += won
	}

	switch {
	case g.result.Free && g.result.FreeSpinsWon > 0:
		g.message = fmt.Sprintf("GIANT DICE! %d MORE FREE GAMES AND YOU WIN %d!", g.result.FreeSpinsWon, g.win)
	case g.result.FreeSpinsWon > 0:
		g.message = fmt.Sprintf("%d DICE! %d FREE GAMES", g.result.Scatters, g.result.FreeSpinsWon)
		if g.win > 0 {
			g.message += fmt.Sprintf(" AND YOU WIN %d!", g.win)
		}
	case g.win > 0:
		g.message = fmt.Sprintf("YOU WIN %d!", g.win)
	case g.bank.Credits == 0 && g.m.FreeSpins == 0:
		g.message = outOfCredits
	default:
		g.message = "NO WIN"
	}
	// The last free game: report what the whole run won. A giant chip on it
	// leaves that until the link feature is over.
	if g.free.active && g.m.FreeSpins == 0 && !g.result.LinkStarted {
		g.free.active = false
		g.message = fmt.Sprintf("FREE GAMES OVER - THEY WON %d CREDITS", g.free.win)
	}

	switch {
	case g.result.FreeSpinsWon > 0, g.win >= 10*g.m.TotalBet():
		g.snd.Play(g.snd.BigWin)
	case g.win > 0:
		g.snd.Play(g.snd.Win)
	}
	// A feature turns auto spin off; it has to be started again by hand.
	if g.result.FreeSpinsWon > 0 || g.result.LinkStarted {
		g.auto = false
	}
	if g.result.LinkStarted {
		g.startLink()
	}
}

// shownWin is the winning line on display. The wins take turns, about a
// second each.
func (g *Game) shownWin() *machine.LineWin {
	if g.spinning || len(g.result.Wins) == 0 {
		return nil
	}
	i := (g.ticks - g.stopTick) / 60 % len(g.result.Wins)
	return &g.result.Wins[i]
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(ui.Black)
	ui.DrawBorder(screen, ui.Green)

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
	ui.DrawText(screen, "SLOT MACHINE", ui.ScreenW/2, 90, 40, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	g.menu.Draw(screen)
	ui.DrawText(screen, "ARROWS move    ENTER or NUMBER choose    ESC main menu", ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "MAKE YOUR CHOICE", ui.ScreenW/2, 355, 24, ui.LightBlue, ui.AlignCenter, ui.AlignStart)

	for s := machine.Ten; s < machine.NumSymbols; s++ {
		drawSymbol(screen, s, ui.ScreenW/2-286+float64(s)*52, 470, 0.42)
	}
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	ui.DrawText(screen, "HOW TO PLAY", ui.ScreenW/2, 24, 32, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	top := machine.BetChoices[len(machine.BetChoices)-1]
	lines := []string{
		"Choose a bet per line and how many lines to play, then spin.",
		"A spin costs the bet per line times the number of lines.",
		"Each line is a path across the five reels, read left to right.",
		"Three or more of a symbol in a row from the left reel win.",
		"Four or five in a row pay more. Two dollars in a row pay too.",
		"A win on a line is multiplied by the bet per line.",
		"The joker is wild on reels 2 to 5 and doubles any win it helps make.",
		fmt.Sprintf("Three or more dice anywhere win %d free games. In a free game reels", machine.FreeSpinAwards[3]),
		"2 to 4 show one giant symbol and the joker does not double.",
		fmt.Sprintf("A giant dice adds %d games. A giant chip starts the link feature.", machine.BigScatterGames),
		fmt.Sprintf("%d or more chips lock in place and respin. Each chip pays 1 to", machine.LinkTrigger),
		fmt.Sprintf("%d times the total bet, or MINI (x%d), MINOR (x%d) or the MAJOR.",
			machine.TopChipBets, machine.MiniBets, machine.MinorBets),
		fmt.Sprintf("Fill all %d places to win the GRAND as well.", machine.Cells),
		"The MAJOR and GRAND pay the same at any bet, so they are won more",
		fmt.Sprintf("often the more you bet: most often at %d a line on %d lines.", top, len(machine.Paylines)),
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2-280, 66+float64(i)*19, 15, ui.White, ui.AlignStart, ui.AlignStart)
	}

	keys := [][2]string{
		{"SPACE or ENTER", "spin the reels"},
		{"UP / DOWN", "change the bet per line"},
		{"LEFT / RIGHT", "change the number of lines"},
		{"M", "bet the maximum"},
		{"A", "auto spin on or off"},
		{"C", "insert a coin for more credits"},
		{"R", "reset your credits"},
		{"S", "sound on or off"},
		{"ESC", "back to the menu"},
	}
	for i, k := range keys {
		y := 360 + float64(i)*18
		ui.DrawText(screen, k[0], ui.ScreenW/2-70, y, 14, ui.LightRed, ui.AlignEnd, ui.AlignStart)
		ui.DrawText(screen, k[1], ui.ScreenW/2-45, y, 14, ui.White, ui.AlignStart, ui.AlignStart)
	}
	ui.DrawText(screen, "The buttons on screen can be clicked with the mouse.", ui.ScreenW/2, 527, 13, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 550, 20, ui.LightBlue, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawAbout(screen *ebiten.Image) {
	ui.DrawText(screen, "ABOUT THIS GAME", ui.ScreenW/2, 60, 36, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	returns := "Working out what it pays back..."
	if rtp := g.rtp.Load(); rtp != nil {
		returns = fmt.Sprintf("Over a long run it pays back %.0f%% of what is bet.", *rtp*100)
	}
	lines := []string{
		"Slot Machine is written in Go with Ebitengine.",
		"The reels are random and wins are paid in credits.",
		"",
		returns,
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2, 170+float64(i)*28, 18, ui.White, ui.AlignCenter, ui.AlignStart)
	}
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 480, 20, ui.LightBlue, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawPlay(screen *ebiten.Image) {
	g.drawJackpots(screen)
	inFree := g.free.active || g.m.FreeSpins > 0
	g.drawStatus(screen, inFree)

	// The magenta frame shows through the gaps between the reels.
	ui.FillRect(screen, reelsX-5, reelsY-5, reelsW+10, reelsH+10, ui.Magenta)
	var win *machine.LineWin
	if g.link.show {
		g.drawLinkGrid(screen)
	} else {
		for i := range g.reels {
			g.drawReel(screen, i)
		}
		win = g.shownWin()
		g.drawWins(screen, win)
	}

	g.drawPaytable(screen, win)

	drawStat(screen, "CREDITS", g.credits, 0, ui.LightGreen)
	// A credit is a cent: the balance in dollars goes under it.
	cx, _ := cellCentre(0, 0)
	ui.DrawText(screen, bank.Dollars(g.credits), cx, 413, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	drawStat(screen, "BET / LINE", g.m.BetPerLine, 1, ui.White)
	drawStat(screen, "LINES", g.m.Lines, 2, ui.White)
	if inFree {
		drawStat(screen, "FREE GAMES WIN", g.free.win, 3, ui.Yellow)
	} else {
		drawStat(screen, "TOTAL BET", g.m.TotalBet(), 3, ui.White)
	}
	drawStat(screen, "WIN", g.win, 4, ui.Yellow)

	message := g.message
	if win != nil && len(g.result.Wins) > 1 {
		message += fmt.Sprintf("   LINE %d PAYS %d", win.Line+1, win.Win)
	}
	if win != nil && win.Doubled {
		message += fmt.Sprintf("   JOKER x%d", machine.WildMultiplier)
	}
	ui.DrawText(screen, message, reelsX+reelsW/2, 420, 16, ui.LightRed, ui.AlignCenter, ui.AlignStart)

	// The bet cannot be changed while a feature is being played.
	locked := g.m.FreeSpins > 0 || g.linkBusy()
	for i, n := range machine.BetChoices {
		ui.DrawButton(screen, betButton(i), n == g.m.BetPerLine, locked, 11, fmt.Sprintf("PLAY %d", n), "PER LINE")
	}
	for i, n := range machine.LineChoices {
		label := fmt.Sprintf("%d LINES", n)
		if n == 1 {
			label = "1 LINE"
		}
		ui.DrawButton(screen, lineButton(i), n == g.m.Lines, locked, 14, label)
	}
	spinLabel := "SPIN"
	switch {
	case g.linkBusy():
		spinLabel = "RESPIN"
	case locked:
		spinLabel = "FREE GAME"
	}
	ui.FillRect(screen, spinButton.X, spinButton.Y, spinButton.W, spinButton.H, ui.Green)
	ui.FrameRect(screen, spinButton.X, spinButton.Y, spinButton.W, spinButton.H, 2, ui.LightGreen)
	spinSize := 26.0
	if len(spinLabel) > 4 {
		spinSize = 18
	}
	ui.DrawText(screen, spinLabel, spinButton.X+spinButton.W/2, spinButton.Y+spinButton.H/2, spinSize, ui.White, ui.AlignCenter, ui.AlignCenter)

	autoLabel := "SPIN"
	if g.auto {
		autoLabel = "ON"
	}
	ui.DrawButton(screen, autoButton, g.auto, false, 13, "AUTO", autoLabel)

	coinLabel := fmt.Sprintf("INSERT COIN +$%d", bank.CoinCredits/100)
	ui.DrawCoinButton(screen, coinButton, 12, coinLabel)
	ui.DrawResetButton(screen, resetButton, 12)

	sound := "ON"
	if !g.snd.On {
		sound = "OFF"
	}
	hint := fmt.Sprintf("SPACE spin    UP/DOWN bet per line    LEFT/RIGHT lines    M max bet    A auto    C coin    R reset    S sound %s    ESC menu", sound)
	ui.DrawText(screen, hint, ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
}

// drawWins marks what the last spin won over the reels: the payline on
// show, or the lines being played after a change of bet.
func (g *Game) drawWins(screen *ebiten.Image, win *machine.LineWin) {
	switch {
	case win != nil:
		drawPayline(screen, win.Line, 4)
		for i := range win.Matched {
			boxCell(screen, i, machine.Paylines[win.Line][i], lineColor(win.Line))
		}
	case g.showLines > 0 && !g.spinning:
		for l := range g.m.Lines {
			drawPayline(screen, l, 2)
		}
	}
	// Winning dice flash wherever they landed; a giant one as a whole.
	if !g.spinning && g.result.FreeSpinsWon > 0 && g.ticks/15%2 == 0 {
		if g.result.Free {
			r := bigRect()
			ui.FrameRect(screen, float64(r.Min.X)+2, float64(r.Min.Y)+2, float64(r.Dx())-4, float64(r.Dy())-4, 4, ui.White)
			return
		}
		for i, col := range g.result.Grid {
			for row, sym := range col {
				if sym == machine.Scatter {
					boxCell(screen, i, row, ui.White)
				}
			}
		}
	}
}

// boxCell outlines the cell at the given reel and row.
func boxCell(screen *ebiten.Image, reel, row int, c color.Color) {
	x, y := cellCentre(reel, row)
	ui.FrameRect(screen, x-reelW/2+2, y-rowH/2+2, reelW-4, rowH-4, 4, c)
}

// cellCentre is the middle of the cell at the given reel and row.
func cellCentre(reel, row int) (x, y float64) {
	return reelsX + float64(reel)*(reelW+reelGap) + reelW/2,
		reelsY + float64(row)*rowH + rowH/2
}

// lineColor gives each payline its own colour. Stepping the hue by a large
// odd amount keeps neighbouring lines well apart.
func lineColor(line int) color.Color {
	h := float64(line*137%360) / 60
	x := uint8(255 * (1 - math.Abs(math.Mod(h, 2)-1)))
	switch int(h) {
	case 0:
		return color.RGBA{255, x, 0, 255}
	case 1:
		return color.RGBA{x, 255, 0, 255}
	case 2:
		return color.RGBA{0, 255, x, 255}
	case 3:
		return color.RGBA{0, x, 255, 255}
	case 4:
		return color.RGBA{x, 0, 255, 255}
	}
	return color.RGBA{255, 0, x, 255}
}

// drawPayline draws a payline's path across the reels, with its number at
// the left-hand end.
func drawPayline(screen *ebiten.Image, line int, width float64) {
	c := lineColor(line)
	rows := machine.Paylines[line]

	px, py := cellCentre(0, rows[0])
	px = reelsX - 5
	seg := func(x, y float64) {
		ui.DrawLine(screen, px, py, x, y, width+2, ui.Black)
		ui.DrawLine(screen, px, py, x, y, width, c)
		px, py = x, y
	}
	for i, row := range rows {
		seg(cellCentre(i, row))
	}
	seg(reelsX+reelsW+5, py)

	_, y := cellCentre(0, rows[0])
	ui.DrawText(screen, fmt.Sprint(line+1), reelsX-7, y, 10, c, ui.AlignEnd, ui.AlignCenter)
}

// drawReel draws the part of a reel's strip that shows in its window. The
// sub-image clips symbols that are part-way in or out.
func (g *Game) drawReel(screen *ebiten.Image, i int) {
	strip := g.strip(i)
	if strip == nil {
		return // hidden behind the giant symbol
	}
	if g.freeView && i == machine.BigReel {
		g.drawBigReel(screen)
		return
	}
	cx, cy := cellCentre(i, machine.Middle)
	x := int(cx) - reelW/2
	window := screen.SubImage(image.Rect(x, reelsY, x+reelW, reelsY+reelsH)).(*ebiten.Image)
	window.Fill(reelBack)

	n := len(strip)
	base := math.Floor(g.reels[i].pos)
	frac := g.reels[i].pos - base
	for k := -2; k <= 2; k++ {
		stop := ((int(base)+k)%n + n) % n
		y := cy - (float64(k)-frac)*rowH
		if strip[stop] == machine.Link {
			drawChip(window, cx, y, reelScale, g.chipLabel(g.chipAt(i, stop), g.m.TotalBet()))
			continue
		}
		drawSymbol(window, strip[stop], cx, y, reelScale)
	}
}

// bigScale is the size of a giant symbol: one that fills reels 2 to 4.
const bigScale = 2.6

// bigRect is the part of the screen the giant symbol covers.
func bigRect() image.Rectangle {
	left, _ := cellCentre(machine.BigFirst, machine.Middle)
	right, _ := cellCentre(machine.BigLast, machine.Middle)
	return image.Rect(int(left)-reelW/2, reelsY, int(right)+reelW/2, reelsY+reelsH)
}

// drawBigReel draws the middle reels of a free game: one reel of giant
// symbols, each as high as the window.
func (g *Game) drawBigReel(screen *ebiten.Image) {
	rect := bigRect()
	window := screen.SubImage(rect).(*ebiten.Image)
	window.Fill(reelBack)

	r := &g.reels[machine.BigReel]
	cx, cy := cellCentre(machine.BigReel, machine.Middle)
	n := len(bigOrder)
	base := math.Floor(r.pos)
	frac := r.pos - base
	for k := -1; k <= 1; k++ {
		at := ((int(base)+k)%n + n) % n
		y := cy - (float64(k)-frac)*reelsH
		// Only the giant chip that has landed shows what it was dealt.
		chip := passingChips[at%len(passingChips)]
		if !r.spinning && k == 0 {
			chip = g.result.Chips[machine.BigReel][machine.Middle]
		}
		g.drawBig(window, bigOrder[at], chip, cx, y)
	}
}

// drawBig draws a giant symbol centred on (cx, cy). chip is what it carries
// if it is a chip.
func (g *Game) drawBig(dst *ebiten.Image, sym machine.Symbol, chip machine.Chip, cx, cy float64) {
	if sym == machine.Link {
		drawChip(dst, cx, cy, bigScale, g.chipLabel(chip, g.m.TotalBet()))
		return
	}
	drawSymbol(dst, sym, cx, cy, bigScale)
}

// chipAt is what the chip at a reel stop shows. The chips the last spin
// landed carry what they were dealt; any others, seen only as the reel
// turns, are dressed up from a fixed list.
func (g *Game) chipAt(reel, stop int) machine.Chip {
	if g.spun {
		n := len(machine.Strips[reel])
		for row := range machine.Rows {
			// Rows run top to bottom, against the order of the strip.
			if stop == ((g.result.Stops[reel]+1-row)%n+n)%n {
				return g.result.Chips[reel][row]
			}
		}
	}
	return passingChips[(reel*5+stop)%len(passingChips)]
}

var passingChips = []machine.Chip{
	{Bets: 1}, {Bets: 2}, {Bets: 1}, {Jackpot: machine.Mini}, {Bets: 3},
	{Bets: 1}, {Bets: 5}, {Bets: 2}, {Jackpot: machine.Minor}, {Bets: 1},
	{Bets: 10}, {Bets: 2}, {Jackpot: machine.Major}, {Bets: 4}, {Bets: 20},
	{Bets: 1}, {Bets: 50}, {Bets: 3}, {Bets: 15}, {Bets: 100},
}

// chipLabel is the text on a link chip: the name of its jackpot, or its
// value in credits at the given total bet.
func (g *Game) chipLabel(c machine.Chip, bet int) string {
	if c.Jackpot != machine.NoJackpot {
		return c.Jackpot.String()
	}
	return fmt.Sprint(g.m.ChipValue(c, bet))
}

// drawPaytable draws what each win pays at the current bet, picking out the
// entry for win if there is one.
func (g *Game) drawPaytable(screen *ebiten.Image, win *machine.LineWin) {
	const (
		top     = reelsY - 4
		rowsY   = top + 42
		rowStep = 22
		icon    = 0.22
	)

	// How far the right-hand edge of the column for a run of n symbols is
	// from the edge of the panel. Columns for longer runs are wider, as
	// their payouts have more digits.
	edges := [machine.Reels + 1]float64{2: 126, 3: 91, 4: 49, 5: 0}
	col := func(n int) float64 { return panelX + panelW - 12 - edges[n] }

	// A free game has a paytable of its own. It stays up while free games
	// are being played, and afterwards for as long as the last one's win
	// is on show.
	pays, title := &machine.Pays, "PAYS"
	free := g.freeView && (g.free.active || g.m.FreeSpins > 0 || win != nil)
	if free {
		pays, title = &machine.FreePays, "FREE GAMES PAY"
	}

	ui.FrameRect(screen, panelX, top, panelW, panelH, 2, ui.Magenta)
	ui.DrawText(screen, title, panelX+panelW/2, top+5, 15, ui.Yellow, ui.AlignCenter, ui.AlignStart)
	for n := 2; n <= machine.Reels; n++ {
		if free && n < 4 {
			continue // nothing pays for fewer than four in a free game
		}
		ui.DrawText(screen, fmt.Sprintf("x%d", n), col(n), top+25, 12, ui.Yellow, ui.AlignEnd, ui.AlignStart)
	}

	// The symbols are listed from the best paying down.
	i := 0
	for sym := machine.Dollar; sym >= machine.Ten; sym-- {
		cy := float64(rowsY + i*rowStep + rowStep/2)
		lit := win != nil && win.Symbol == sym
		if lit {
			ui.FillRect(screen, panelX+3, cy-rowStep/2, panelW-6, rowStep, ui.DarkGray)
		}
		drawSymbol(screen, sym, panelX+22, cy, icon)
		for n := 2; n <= machine.Reels; n++ {
			pay := pays[sym][n] * g.m.BetPerLine
			if pay == 0 {
				continue
			}
			c := color.Color(ui.White)
			if lit && n == win.Matched {
				c = ui.Yellow
			}
			ui.DrawText(screen, fmt.Sprint(pay), col(n), cy, 12, c, ui.AlignEnd, ui.AlignCenter)
		}
		i++
	}

	y := float64(rowsY + i*rowStep + 4)
	note := fmt.Sprintf("AT %d PER LINE", g.m.BetPerLine)
	ui.DrawText(screen, note, panelX+panelW-12, y, 10, ui.LightGray, ui.AlignEnd, ui.AlignStart)

	bet := g.m.TotalBet()
	notes := []string{
		fmt.Sprintf("JOKER: WILD ON REELS 2-5, PAYS x%d", machine.WildMultiplier),
		fmt.Sprintf("3+ DICE: %d FREE GAMES", machine.FreeSpinAwards[3]),
		fmt.Sprintf("%d+ CHIPS: LINK FEATURE", machine.LinkTrigger),
		fmt.Sprintf("DICE PAY %d / %d / %d",
			machine.ScatterPays[3]*bet, machine.ScatterPays[4]*bet, machine.ScatterPays[5]*bet),
	}
	if free {
		notes = []string{
			"JOKER: WILD, DOES NOT DOUBLE",
			fmt.Sprintf("GIANT DICE: PAYS %d, %d MORE GAMES", machine.BigScatterPay*bet, machine.BigScatterGames),
			"GIANT CHIP: LINK FEATURE",
		}
	}
	for j, l := range notes {
		ui.DrawText(screen, l, panelX+10, y+14+float64(j)*12, 9, ui.LightGray, ui.AlignStart, ui.AlignStart)
	}
}

// drawStat draws a labelled number under the given reel column.
func drawStat(screen *ebiten.Image, label string, value, column int, c color.Color) {
	x, _ := cellCentre(column, 0)
	ui.DrawText(screen, label, x, 370, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, fmt.Sprint(value), x, 384, 24, c, ui.AlignCenter, ui.AlignStart)
}
