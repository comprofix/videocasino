// Package roulette is Roulette: a wheel with a single zero and the table to
// bet on it, played for pretend credits from the bank. Its rules are in the
// wheel package.
package roulette

import (
	"fmt"
	"image/color"
	"math"
	"math/rand/v2"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"videocasino/bank"
	"videocasino/games/roulette/wheel"
	"videocasino/ui"
)

const (
	// The two rows of buttons under the table.
	rowAY   = 448
	rowBY   = 494
	buttonH = 40

	// How a spin goes, in ticks and turns. The wheel turns one way, slowing
	// all the while. The ball is sent round the rim the other way, drops
	// towards the pockets as it slows, bounces and comes to rest in one;
	// from then on it rides round with the wheel until that stops too.
	landTicks  = 300                         // from the spin until the ball is at rest
	coastTicks = 450                         // until the wheel has stopped
	wheelTurn  = 2.5 * 2 * math.Pi           // how far the wheel turns in a spin
	ballTurn   = 6 * 2 * math.Pi             // how far round the wheel the ball runs, at least
	ballPower  = 2.2                         // how sharply the ball slows
	dropFrom   = 0.72                        // how far through its run the ball leaves the rim
	bounces    = 4                           // how often it bounces on the way down
	bounceH    = 7                           // how high the first bounce is
	rimStep    = math.Pi / 3                 // how far the ball runs between ticks of the rim
	pocketTurn = 2 * math.Pi / wheel.Pockets // the angle a pocket takes up
)

// felt is the table: a dark blue, so that the game has a look of its own.
var felt = color.RGBA{0, 24, 80, 255}

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

	// The sounds that are this game's own.
	chipSound, startSound, rimSound, bounceSound, restSound []byte

	m    *wheel.Machine
	mark point // where on the table the marker is

	// Where the pointer was last, so that it moves the marker only when it
	// is itself moved.
	seen         bool
	lastX, lastY int

	// The machine settles a spin all at once. The ball is then shown
	// running round to the number, and the result once it is at rest.
	spinning  bool
	result    wheel.Result
	spun      bool    // the wheel has been spun since the game was opened
	spinStart int     // the tick it was last spun at
	wheelFrom float64 // the angle it was at then
	ballFrom  float64 // how far round the wheel from its pocket the ball set off
	rimTicks  int     // the sounds the ball has made so far on the rim,
	bounced   int     // and on its way down

	hasBall bool // a number has come up, so the ball lies in its pocket
	number  int  // the number the ball is in, or on its way to
	shown   bool // the result of the last spin is on show

	credits int // credits on display; a win is added once the ball is at rest
	win     int
	message string

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
	g.m = wheel.NewRandom(b)
	g.mark = middle(0)
	g.message = "PLACE YOUR BETS"

	g.chipSound = ui.Tone(1480, 22, 0.14, true)
	g.startSound = append(ui.Tone(392, 50, 0.18, true), ui.Tone(587, 80, 0.18, true)...)
	g.rimSound = ui.Tone(2400, 9, 0.05, true)
	g.bounceSound = ui.Tone(1250, 26, 0.16, true)
	g.restSound = ui.Tone(470, 70, 0.22, true)

	var s wheel.Saved
	if b.Load(saveName, &s) {
		g.m.Restore(s)
		g.resume()
	}
	g.credits = b.Credits
	return g
}

// resume puts the ball back in the pocket of the last number to come up
// before the game was closed.
func (g *Game) resume() {
	if len(g.m.History) > 0 {
		g.hasBall, g.number = true, g.m.History[0]
	}
}

// The buttons under the table.
func chipButton(i int) ui.Button {
	return ui.Button{X: 30 + float64(i)*76, Y: rowAY, W: 70, H: buttonH}
}

var (
	clearButton = ui.Button{X: 420, Y: rowAY, W: 80, H: buttonH}
	rebetButton = ui.Button{X: 506, Y: rowAY, W: 84, H: buttonH}
	spinButton  = ui.Button{X: 600, Y: rowAY, W: 170, H: rowBY + buttonH - rowAY} // as tall as two rows
	coinButton  = ui.Button{X: 30, Y: rowBY, W: 240, H: buttonH}
	resetButton = ui.Button{X: 276, Y: rowBY, W: 90, H: buttonH}
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

// held reports whether a key has just been pressed, or has been held down
// long enough to count as pressed again: the marker has a long way to go
// across the table.
func held(k ebiten.Key) bool {
	d := inpututil.KeyPressDuration(k)
	return d == 1 || d > 22 && d%4 == 0
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
		g.updatePlay()
	}
	return false
}

func (g *Game) updatePlay() {
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if g.spinning {
		// The spin key or a click cuts the spin short.
		g.roll(click || ui.Pressed(ebiten.KeySpace, ebiten.KeyEnter, ebiten.KeyNumpadEnter))
		return
	}
	// The bank may have changed while the game was not being played.
	g.credits = g.bank.Credits
	if ui.Pressed(ebiten.KeyEscape) {
		g.scene = sceneMenu
		return
	}

	// The pointer moves the marker to the bet it is over. A click bets
	// there, and a click of the other button takes the bet off again.
	x, y := ebiten.CursorPosition()
	moved := g.seen && (x != g.lastX || y != g.lastY)
	g.seen, g.lastX, g.lastY = true, x, y
	away := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	if p, ok := pointAt(x, y); ok && (moved || click || away) {
		g.mark = p
		switch {
		case click:
			g.place()
		case away:
			g.remove()
		}
	}

	hit := func(b ui.Button) bool { return click && b.Hit(x, y) }
	for i, n := range wheel.ChipChoices {
		if hit(chipButton(i)) || ui.Pressed(chipKeys[i][:]...) {
			g.setChip(n)
		}
	}
	switch {
	case hit(coinButton), ui.Pressed(ebiten.KeyC):
		g.insertCoin()
	case hit(resetButton), ui.Pressed(ebiten.KeyR):
		g.reset()
	case hit(clearButton), ui.Pressed(ebiten.KeyX):
		g.clear()
	case hit(rebetButton), ui.Pressed(ebiten.KeyB):
		g.rebet()
	case hit(spinButton), ui.Pressed(ebiten.KeySpace):
		g.spin()
	case ui.Pressed(ebiten.KeyEnter, ebiten.KeyNumpadEnter):
		g.place()
	case ui.Pressed(ebiten.KeyBackspace, ebiten.KeyDelete):
		g.remove()
	case held(ebiten.KeyArrowLeft):
		g.mark = move(g.mark, -1, 0)
	case held(ebiten.KeyArrowRight):
		g.mark = move(g.mark, 1, 0)
	case held(ebiten.KeyArrowUp):
		g.mark = move(g.mark, 0, -1)
	case held(ebiten.KeyArrowDown):
		g.mark = move(g.mark, 0, 1)
	}
}

// clearWin takes the last result off the table, once the player has moved
// on from it.
func (g *Game) clearWin() {
	g.win = 0
	g.shown = false
}

func (g *Game) setChip(n int) {
	g.m.Chip = n
	g.snd.Play(g.snd.Click)
	g.save()
}

const outOfCredits = "OUT OF CREDITS - INSERT A COIN OR RESET"

// place puts a chip on the bet the marker is on.
func (g *Game) place() {
	spot, ok := spotAt(g.mark)
	if !ok {
		return
	}
	g.clearWin()
	if !g.m.Place(spot) {
		switch {
		case g.bank.Credits == 0:
			g.message = outOfCredits
		case g.m.Bets[spot]+g.m.Chip > wheel.MaxSpot:
			g.message = fmt.Sprintf("NO MORE THAN %d ON ONE BET", wheel.MaxSpot)
		default:
			g.message = "NOT ENOUGH CREDITS FOR THAT CHIP"
		}
		return
	}
	g.message = "PLACE YOUR BETS"
	g.snd.Play(g.chipSound)
	g.save()
}

// remove takes a chip off the bet the marker is on.
func (g *Game) remove() {
	spot, ok := spotAt(g.mark)
	if !ok || !g.m.Remove(spot) {
		return
	}
	g.clearWin()
	g.message = "PLACE YOUR BETS"
	g.snd.Play(g.chipSound)
	g.save()
}

// clear takes every chip off the table.
func (g *Game) clear() {
	g.m.Clear()
	g.clearWin()
	g.message = "PLACE YOUR BETS"
	g.snd.Play(g.snd.Click)
	g.save()
}

// rebet puts the bets of the last spin back on the table.
func (g *Game) rebet() {
	g.clearWin()
	if !g.m.Rebet() {
		g.message = "NOT ENOUGH CREDITS TO BET THE SAME AGAIN"
		if g.m.LastTotal() == 0 {
			g.message = "THERE ARE NO BETS TO MAKE AGAIN YET"
		}
		return
	}
	g.message = "THE SAME BETS AGAIN - PRESS SPIN"
	g.snd.Play(g.chipSound)
	g.save()
}

// reset puts the bank's credits back to the starting amount. The bets on
// the table are left as they are.
func (g *Game) reset() {
	g.bank.Reset()
	g.credits = g.bank.Credits
	g.clearWin()
	g.message = "CREDITS RESET - PLACE YOUR BETS"
	g.snd.Play(g.snd.Click)
	g.save()
}

// insertCoin adds credits to the bank, so play can go on after they run
// out.
func (g *Game) insertCoin() {
	g.bank.InsertCoin()
	g.credits = g.bank.Credits
	g.message = fmt.Sprintf("COIN INSERTED - %d CREDITS ADDED", bank.CoinCredits)
	g.snd.Play(g.snd.Coin)
	g.save()
}

// spin decides the outcome straight away, then sends the ball round the
// wheel towards it. With nothing on the table the bets of the last spin are
// made again first. With too few credits nothing is spun: the bank is every
// game's, so it is for the player to insert a coin or reset it.
func (g *Game) spin() {
	if g.m.Total() == 0 {
		g.m.Rebet()
	}
	res, ok := g.m.Spin()
	if !ok {
		g.clearWin()
		switch {
		case g.bank.Credits == 0:
			g.message = outOfCredits
		case g.m.Total() == 0 && g.m.LastTotal() == 0:
			g.message = "PLACE A BET BEFORE YOU SPIN"
		default:
			g.message = "NOT ENOUGH CREDITS - INSERT A COIN OR BET LESS"
		}
		return
	}
	// The machine has already settled the spin, so it is safe to save now
	// even though the ball has yet to show it.
	g.save()
	g.clearWin()

	// The wheel is spun on from wherever it has got to. The ball sets off
	// from anywhere round the rim, so where it starts says nothing of where
	// it will end.
	g.wheelFrom = math.Mod(g.wheelAngle(), 2*math.Pi)
	g.spinStart, g.spun = g.ticks, true
	g.ballFrom = ballTurn + rand.Float64()*2*math.Pi
	g.rimTicks, g.bounced = 0, 0

	g.result = res
	g.number, g.hasBall = res.Number, true
	g.spinning = true
	g.credits = g.bank.Credits - res.Win
	g.message = "NO MORE BETS"
	g.snd.Play(g.startSound)
}

// wheelAngle returns how far the wheel has turned. It slows evenly from the
// spin until it stops.
func (g *Game) wheelAngle() float64 {
	if !g.spun {
		return g.wheelFrom
	}
	left := 1 - min(float64(g.ticks-g.spinStart)/coastTicks, 1)
	return g.wheelFrom + wheelTurn*(1-left*left)
}

// ball returns where the ball is: its angle round the wheel, clockwise from
// the top, and how far out from the middle. At rest it lies in the pocket of
// its number and turns with the wheel.
func (g *Game) ball() (angle, radius float64) {
	angle = g.wheelAngle() + float64(wheel.Pocket(g.number))*pocketTurn
	if !g.spinning {
		return angle, pocketR
	}
	// It runs against the wheel, fast at first and slowing into its pocket.
	run := min(float64(g.ticks-g.spinStart)/landTicks, 1)
	angle += g.ballFrom * math.Pow(1-run, ballPower)
	if run < dropFrom {
		return angle, trackR
	}
	// It comes down off the rim, bouncing less each time.
	down := (run - dropFrom) / (1 - dropFrom)
	radius = trackR + (pocketR-trackR)*down*down*(3-2*down)
	radius += bounceH * math.Abs(math.Sin(math.Pi*bounces*down)) * (1 - down)
	return angle, min(radius, trackR)
}

// roll lets the ball run on for a tick, making its sounds: a tick each time
// it has gone a little further round the rim, and a clack at each bounce.
// When it is at rest the result is shown.
func (g *Game) roll(skip bool) {
	if skip {
		g.spinStart = g.ticks - landTicks
	}
	run := float64(g.ticks-g.spinStart) / landTicks
	switch {
	case run >= 1:
		g.land()
	case run < dropFrom:
		gone := g.ballFrom * (1 - math.Pow(1-run, ballPower))
		if n := int(gone / rimStep); n > g.rimTicks {
			g.rimTicks = n
			g.snd.Play(g.rimSound)
		}
	default:
		if n := int((run - dropFrom) / (1 - dropFrom) * bounces); n > g.bounced {
			g.bounced = n
			g.snd.Play(g.bounceSound)
		}
	}
}

// land shows the result of the spin, now that the ball is at rest.
func (g *Game) land() {
	g.spinning = false
	g.shown = true
	g.credits = g.bank.Credits
	g.win = g.result.Win
	g.snd.Play(g.restSound)

	n := g.result.Number
	switch {
	case g.win == 0:
		g.message = fmt.Sprintf("%d %s - NO WIN", n, colourName(n))
	case g.win >= 10*g.result.Bet:
		g.message = fmt.Sprintf("%d %s - YOU WIN %d", n, colourName(n), g.win)
		g.snd.Play(g.snd.BigWin)
	default:
		g.message = fmt.Sprintf("%d %s - YOU WIN %d", n, colourName(n), g.win)
		g.snd.Play(g.snd.Win)
	}
}

func colourName(n int) string {
	switch {
	case wheel.IsRed(n):
		return "RED"
	case wheel.IsBlack(n):
		return "BLACK"
	}
	return "GREEN"
}

// numberInk is the colour of a number's pocket, and of its place on the
// table.
func numberInk(n int) color.Color {
	switch {
	case wheel.IsRed(n):
		return ui.Red
	case wheel.IsBlack(n):
		return ui.Black
	}
	return ui.Green
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(felt)
	ui.DrawBorder(screen, ui.Red)

	switch g.scene {
	case sceneMenu:
		g.drawMenu(screen)
	case sceneHelp:
		g.drawHelp(screen)
	case sceneAbout:
		g.drawAbout(screen)
	case scenePlay:
		g.drawPlay(screen)
	}
}

func (g *Game) drawMenu(screen *ebiten.Image) {
	ui.DrawText(screen, "ROULETTE", ui.ScreenW/2, 70, 48, ui.Yellow, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "SINGLE ZERO", ui.ScreenW/2, 128, 20, ui.White, ui.AlignCenter, ui.AlignStart)

	g.menu.Draw(screen)
	ui.DrawText(screen, "ARROWS move    ENTER or NUMBER choose    ESC main menu", ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "MAKE YOUR CHOICE", ui.ScreenW/2, 350, 24, ui.LightCyan, ui.AlignCenter, ui.AlignStart)

	// A run of numbers as they lie round the wheel, for luck.
	const size, count = 40, 11
	for i := range count {
		n := wheel.Order[(i+wheel.Pockets-count/2)%wheel.Pockets]
		x := ui.ScreenW/2 + (float64(i)-count/2.0)*(size+4) + 2
		ui.FillRect(screen, x, 440, size, size, ui.LightGray)
		ui.FillRect(screen, x+2, 442, size-4, size-4, numberInk(n))
		ui.DrawText(screen, fmt.Sprint(n), x+size/2, 440+size/2, 18, ui.White, ui.AlignCenter, ui.AlignCenter)
	}
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	ui.DrawText(screen, "HOW TO PLAY", ui.ScreenW/2, 22, 30, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	p := wheel.Pays
	lines := []string{
		"Choose a chip and put chips on the table, then spin. The ball comes to",
		"rest on one of 37 numbers: 1 to 36 and a single zero.",
		fmt.Sprintf("A chip on a number pays %d to 1. On the line between two numbers it is", p[wheel.Straight]),
		fmt.Sprintf("a split, paying %d; where four meet, a corner, paying %d. At the foot", p[wheel.Split], p[wheel.Corner]),
		fmt.Sprintf("of a row of three it is a street, paying %d, and at the foot of the line", p[wheel.Street]),
		fmt.Sprintf("between two rows a six line, paying %d. The zero splits with 1, 2 and 3.", p[wheel.SixLine]),
		fmt.Sprintf("A column or a dozen pays %d to 1. Red, black, odd, even, 1 to 18 and", p[wheel.Dozen]),
		fmt.Sprintf("19 to 36 pay %d to 1. The zero is none of these: they all lose on it.", p[wheel.Reds]),
		fmt.Sprintf("A bet can hold up to %d. Bets are paid for when the wheel is spun.", wheel.MaxSpot),
		"SPIN with nothing on the table makes the last bets again.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2-275, 64+float64(i)*19, 15, ui.White, ui.AlignStart, ui.AlignStart)
	}

	keys := [][2]string{
		{"SPACE", "spin the wheel"},
		{"ARROWS", "move the marker over the table"},
		{"ENTER", "put a chip where the marker is"},
		{"BACKSPACE", "take a chip off there"},
		{"1 to 5", "choose a chip"},
		{"X", "clear the table"},
		{"B", "make the last bets again"},
		{"C", "insert a coin for more credits"},
		{"R", "reset your credits"},
		{"S", "sound on or off"},
		{"ESC", "back to the menu"},
	}
	for i, k := range keys {
		y := 268 + float64(i)*18
		ui.DrawText(screen, k[0], ui.ScreenW/2-70, y, 14, ui.LightRed, ui.AlignEnd, ui.AlignStart)
		ui.DrawText(screen, k[1], ui.ScreenW/2-45, y, 14, ui.White, ui.AlignStart, ui.AlignStart)
	}
	ui.DrawText(screen, "With the mouse, click to put a chip down and right click to take it off.", ui.ScreenW/2, 478, 13, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "Click near a line or a corner to bet on it, not on the number.", ui.ScreenW/2, 496, 13, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 546, 20, ui.LightCyan, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawAbout(screen *ebiten.Image) {
	ui.DrawText(screen, "ABOUT THIS GAME", ui.ScreenW/2, 60, 36, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	lines := []string{
		"Roulette is written in Go with Ebitengine.",
		"",
		fmt.Sprintf("The wheel has %d pockets: 1 to 36 and a single zero.", wheel.Pockets),
		"Each is as likely to come up as any other.",
		"",
		"Every bet on the table pays back the same:",
		fmt.Sprintf("%.2f%% of what is bet over a long run.", wheel.Return()*100),
		"",
		"It is played with pretend credits only.",
		"No real money goes in or comes out.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2, 150+float64(i)*28, 18, ui.White, ui.AlignCenter, ui.AlignStart)
	}
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 480, 20, ui.LightCyan, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawPlay(screen *ebiten.Image) {
	g.drawWheel(screen)
	g.drawStatus(screen)
	g.drawTable(screen)

	for i, n := range wheel.ChipChoices {
		ui.DrawButton(screen, chipButton(i), n == g.m.Chip, g.spinning, 11, fmt.Sprint(n), "CHIP")
	}
	ui.DrawButton(screen, clearButton, false, g.spinning, 13, "CLEAR")
	ui.DrawButton(screen, rebetButton, false, g.spinning || g.m.LastTotal() == 0, 13, "REBET")

	ui.FillRect(screen, spinButton.X, spinButton.Y, spinButton.W, spinButton.H, ui.Green)
	ui.FrameRect(screen, spinButton.X, spinButton.Y, spinButton.W, spinButton.H, 2, ui.LightGreen)
	ui.DrawText(screen, "SPIN", spinButton.X+spinButton.W/2, spinButton.Y+spinButton.H/2, 30, ui.White, ui.AlignCenter, ui.AlignCenter)

	coinLabel := fmt.Sprintf("INSERT COIN +$%d", bank.CoinCredits/100)
	ui.DrawCoinButton(screen, coinButton, 14, coinLabel)
	ui.DrawResetButton(screen, resetButton, 13)

	// The bet the marker is on, and what is on it.
	if spot, ok := spotAt(g.mark); ok && !g.spinning {
		s := wheel.Spots[spot]
		x := (resetButton.X + resetButton.W + spinButton.X) / 2
		ui.DrawText(screen, s.String(), x, rowBY+4, 12, ui.LightCyan, ui.AlignCenter, ui.AlignStart)
		note := fmt.Sprintf("PAYS %d TO 1", s.Pays())
		if bet := g.m.Bets[spot]; bet > 0 {
			note += fmt.Sprintf("    BET %d", bet)
		}
		ui.DrawText(screen, note, x, rowBY+22, 11, ui.White, ui.AlignCenter, ui.AlignStart)
	}

	sound := "ON"
	if !g.snd.On {
		sound = "OFF"
	}
	hint := fmt.Sprintf("SPACE spin   ARROWS move   ENTER bet   BACKSPACE take off   1-5 chip   X clear   B rebet   C coin   R reset   S sound %s   ESC menu", sound)
	ui.DrawText(screen, hint, ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
}

// drawStatus draws what is beside the wheel: the numbers that have come up,
// the last of them large, the message and the figures.
func (g *Game) drawStatus(screen *ebiten.Image) {
	// While the ball is running the number it will stop on is not shown.
	history := g.m.History
	if g.spinning {
		history = history[1:]
	}
	ui.DrawText(screen, "LAST NUMBERS", 250, 28, 11, ui.LightGray, ui.AlignStart, ui.AlignStart)
	for i, n := range history {
		x := 250 + float64(i)*32
		ui.FillRect(screen, x, 44, 28, 28, ui.LightGray)
		ui.FillRect(screen, x+1, 45, 26, 26, numberInk(n))
		ui.DrawText(screen, fmt.Sprint(n), x+14, 58, 14, ui.White, ui.AlignCenter, ui.AlignCenter)
	}

	const boxX, boxY, boxSize = 250, 88, 100
	ui.FillRect(screen, boxX, boxY, boxSize, boxSize, ui.LightGray)
	ui.FillRect(screen, boxX+3, boxY+3, boxSize-6, boxSize-6, ui.Black)
	if g.shown || !g.spinning && len(history) > 0 {
		n := history[0]
		ui.FillRect(screen, boxX+3, boxY+3, boxSize-6, boxSize-6, numberInk(n))
		ui.DrawText(screen, fmt.Sprint(n), boxX+boxSize/2, boxY+boxSize/2, 56, ui.White, ui.AlignCenter, ui.AlignCenter)

		what := []string{"ZERO"}
		if n != 0 {
			what = []string{colourName(n), "EVEN", "1 TO 18"}
			if n%2 == 1 {
				what[1] = "ODD"
			}
			if n > 18 {
				what[2] = "19 TO 36"
			}
		}
		for i, w := range what {
			ui.DrawText(screen, w, boxX+boxSize+16, boxY+8+float64(i)*30, 18, ui.White, ui.AlignStart, ui.AlignStart)
		}
	}
	ui.DrawText(screen, g.message, 515, 214, 14, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	// Once the wheel is spun the bets are off the table and in play.
	bet := g.m.Total()
	if g.spinning {
		bet = g.result.Bet
	}
	figure := func(i int, label string, v int, c color.Color) {
		y := 80 + float64(i)*44
		ui.DrawText(screen, label, 690, y, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
		ui.DrawText(screen, fmt.Sprint(v), 690, y+13, 24, c, ui.AlignCenter, ui.AlignStart)
	}
	figure(0, "CREDITS", g.credits, ui.LightGreen)
	figure(1, "TOTAL BET", bet, ui.Yellow)
	figure(2, "WIN", g.win, ui.White)
}

// drawTable draws the table, the chips on it and the marker.
func (g *Game) drawTable(screen *ebiten.Image) {
	// room is how much of the right of a box is kept clear of its name, for
	// a chip to lie there.
	cell := func(b ui.Button, fill color.Color, size, room float64, label string) {
		if fill != nil {
			ui.FillRect(screen, b.X, b.Y, b.W, b.H, fill)
		}
		ui.FrameRect(screen, b.X, b.Y, b.W, b.H, 1, ui.LightGray)
		ui.DrawText(screen, label, b.X+(b.W-room)/2, b.Y+b.H/2, size, ui.White, ui.AlignCenter, ui.AlignCenter)
	}
	cell(zeroButton, ui.Green, 20, 0, "0")
	for n := 1; n < wheel.Pockets; n++ {
		cell(numberButton(n), numberInk(n), 17, 0, fmt.Sprint(n))
	}
	for row := range gridRows {
		cell(columnButton(row), nil, 11, 0, "")
	}
	for i, spot := range dozenSpots {
		cell(dozenButton(i), nil, 14, 0, wheel.Spots[spot].String())
	}
	for i, spot := range evenSpots {
		var fill color.Color
		switch wheel.Spots[spot].Kind {
		case wheel.Reds:
			fill = ui.Red
		case wheel.Blacks:
			fill = ui.Black
		}
		cell(evenButton(i), fill, 12, chipRoom, wheel.Spots[spot].String())
	}

	// The bets in play stay on the table while the ball runs. Afterwards
	// the winning ones are left on show until the player moves on.
	bets, flash := g.m.Bets, false
	switch {
	case g.spinning:
		bets = g.m.Last
	case g.shown && g.m.Total() == 0:
		bets, flash = g.m.Last, true
	}

	// The marker lights up the numbers its bet covers.
	marked, onSpot := spotAt(g.mark)
	if onSpot && !g.spinning {
		for _, n := range wheel.Spots[marked].Numbers {
			b := zeroButton
			if n != 0 {
				b = numberButton(n)
			}
			ui.FrameRect(screen, b.X+2, b.Y+2, b.W-4, b.H-4, 2, ui.LightCyan)
		}
	}
	// The winning number flashes.
	if g.shown && g.ticks/15%2 == 0 {
		b := zeroButton
		if n := g.result.Number; n != 0 {
			b = numberButton(n)
		}
		ui.FrameRect(screen, b.X+1, b.Y+1, b.W-2, b.H-2, 4, ui.Yellow)
	}

	// The columns' bets are named where a chip would hide the name least.
	for row := range gridRows {
		b := columnButton(row)
		ui.DrawText(screen, "2 TO 1", b.X+b.W/2, b.Y+b.H/2, 11, ui.White, ui.AlignCenter, ui.AlignCenter)
	}

	for spot, bet := range bets {
		if bet == 0 || flash && !wheel.Spots[spot].Covers(g.result.Number) {
			continue
		}
		x, y := chipAt(spotPoint[spot])
		ui.DrawChip(screen, x, y, bet, wheel.ChipChoices, flash && g.ticks/15%2 == 0)
	}

	if onSpot && !g.spinning {
		if b, ok := outside(g.mark); ok {
			ui.FrameRect(screen, b.X+1, b.Y+1, b.W-2, b.H-2, 2, ui.White)
		} else {
			x, y := chipAt(g.mark)
			ui.FrameCircle(screen, x, y, ui.ChipR+3, 2, ui.White)
		}
	}
}

// The wheel, from the outside in.
const (
	wheelX, wheelY = 125, 128
	rimR           = 100 // the outside of the wheel
	trackR         = 91  // where the ball runs before it drops
	plateR         = 85  // the part that turns
	numberR        = 73  // where the numbers are written
	pocketR        = 53  // where the ball comes to rest
	hubR           = 42
	ballR          = 4.5
)

var (
	wood     = color.RGBA{120, 60, 16, 255}
	darkWood = color.RGBA{58, 28, 8, 255}
)

// polar returns the point at the given angle, clockwise from the top, and
// distance from (cx, cy).
func polar(cx, cy, angle, r float64) (x, y float64) {
	return cx + r*math.Sin(angle), cy - r*math.Cos(angle)
}

// plate is the part of the wheel that turns, drawn once so that it can be
// drawn again turned.
var plate *ebiten.Image

func plateImage() *ebiten.Image {
	if plate != nil {
		return plate
	}
	const c = plateR
	plate = ebiten.NewImage(2*plateR, 2*plateR)

	// The pockets, each a slice of the ring between the hub and the edge.
	const steps = 4
	for i, n := range wheel.Order {
		from := (float64(i) - 0.5) * pocketTurn
		var pts []float64
		for s := range steps + 1 {
			x, y := polar(c, c, from+float64(s)*pocketTurn/steps, plateR-1)
			pts = append(pts, x, y)
		}
		for s := range steps + 1 {
			x, y := polar(c, c, from+pocketTurn-float64(s)*pocketTurn/steps, hubR-1)
			pts = append(pts, x, y)
		}
		ui.FillPoly(plate, numberInk(n), pts...)
	}
	for i := range wheel.Pockets {
		a := (float64(i) - 0.5) * pocketTurn
		x0, y0 := polar(c, c, a, hubR)
		x1, y1 := polar(c, c, a, plateR-1)
		ui.DrawLine(plate, x0, y0, x1, y1, 1.2, ui.LightGray)
	}
	ui.FrameCircle(plate, c, c, plateR-1, 1.5, ui.LightGray)
	ui.FrameCircle(plate, c, c, numberR-11, 1, ui.LightGray)

	// Each number is written facing outwards over its pocket.
	const tw, th = 24, 16
	label := ebiten.NewImage(tw, th)
	for i, n := range wheel.Order {
		label.Clear()
		ui.DrawText(label, fmt.Sprint(n), tw/2, th/2, 11, ui.White, ui.AlignCenter, ui.AlignCenter)
		a := float64(i) * pocketTurn
		x, y := polar(c, c, a, numberR)
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Translate(-tw/2, -th/2)
		op.GeoM.Rotate(a)
		op.GeoM.Translate(x, y)
		plate.DrawImage(label, op)
	}
	label.Deallocate()

	// The hub, with a turret whose arms show the wheel turning.
	ui.FillCircle(plate, c, c, hubR, ui.LightGray)
	ui.FillCircle(plate, c, c, hubR-1.5, wood)
	ui.FillCircle(plate, c, c, hubR-14, darkWood)
	for i := range 4 {
		x, y := polar(c, c, float64(i)*math.Pi/2, hubR-10)
		ui.DrawLine(plate, c, c, x, y, 4, ui.Gold)
		ui.FillCircle(plate, x, y, 4, ui.Gold)
	}
	ui.FillCircle(plate, c, c, 8, ui.Gold)
	ui.FillCircle(plate, c, c, 4, ui.Yellow)
	return plate
}

// drawWheel draws the wheel as it stands, and the ball if it is on it.
func (g *Game) drawWheel(screen *ebiten.Image) {
	ui.FillCircle(screen, wheelX, wheelY, rimR, ui.Gold)
	ui.FillCircle(screen, wheelX, wheelY, rimR-2, wood)
	ui.FillCircle(screen, wheelX, wheelY, trackR+5, darkWood)

	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Translate(-plateR, -plateR)
	op.GeoM.Rotate(g.wheelAngle())
	op.GeoM.Translate(wheelX, wheelY)
	screen.DrawImage(plateImage(), op)

	if g.hasBall {
		angle, r := g.ball()
		x, y := polar(wheelX, wheelY, angle, r)
		ui.FillCircle(screen, x+1, y+1.5, ballR, color.RGBA{0, 0, 0, 120})
		ui.FillCircle(screen, x, y, ballR, ui.White)
		ui.FillCircle(screen, x-1.2, y-1.2, 1.5, ui.LightGray)
	}
}
