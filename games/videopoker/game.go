// Package videopoker is Video Poker: Jacks or Better, played for pretend
// credits from the bank. Its rules are in the poker package.
package videopoker

import (
	"fmt"
	"image/color"
	"strings"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/videopoker/poker"
	"videocasino/ui"
)

const (
	// The paytable runs across the top: the name of each hand, then a
	// column for each number of coins.
	payX    = 30
	payY    = 24
	payW    = ui.ScreenW - 2*payX
	payRowH = 18
	payRows = int(poker.NumCategories) - 1 // every hand that pays
	payH    = float64(payRows * payRowH)
	payColW = 100
	payName = payW - poker.MaxCoins*payColW

	// The hand lies across the middle, with room above each card for the
	// tag that shows it is held.
	cardGap = 18
	cardsX  = (ui.ScreenW - poker.HandSize*ui.CardW - (poker.HandSize-1)*cardGap) / 2
	cardsY  = 218
	tagY    = 195
	tagW    = 70
	tagH    = 19

	messageY = 393
	statY    = 418

	// The two rows of buttons at the bottom.
	row1Y   = 466
	row2Y   = 512
	buttonH = 40

	// turnTicks is the time between one card being turned over and the
	// next.
	turnTicks = 7
)

// felt is the table the cards lie on.
var felt = color.RGBA{0, 0, 96, 255}

type scene int

const (
	sceneMenu scene = iota
	sceneHelp
	sceneAbout
	scenePlay
)

// advice is the odds of the game, which say which cards are best held. A
// royal flush pays more for each coin at the most coins, so that bet has
// odds of its own; any smaller bet is one coin's odds over again.
type advice struct {
	one, most *poker.Odds
}

type Game struct {
	scene scene
	ticks int
	snd   *ui.Sounds
	bank  *bank.Bank
	menu  *ui.Menu

	// The sound that is this game's own.
	holdSound []byte

	m        *poker.Machine
	autoHold bool // the best cards are held as soon as a hand is dealt

	// The odds take a moment to work out, so that is done in the
	// background while the program starts. Until they are ready these are
	// nil.
	advice atomic.Pointer[advice]
	rtp    atomic.Pointer[float64]

	// The hand on show is the machine's. Until the first deal there is
	// none, and the cards lie face down.
	shown   bool
	drawn   bool                // the hand on show has been drawn to and paid
	made    poker.Category      // what it makes
	turnAt  [poker.HandSize]int // the tick at which each card is turned face up
	turning bool                // cards are still being turned
	holdDue bool                // the best cards have yet to be held for this hand
	hint    [poker.HandSize]bool
	hinting bool // the hint is on show

	credits int // credits on display; a win is added once the cards are turned
	win     int
	message string

	saveFailed bool
}

// New returns the game, playing from the given bank and picking up whatever
// it last kept there.
func New(b *bank.Bank, snd *ui.Sounds) *Game {
	g := &Game{snd: snd, bank: b, autoHold: true}
	g.holdSound = ui.Tone(880, 30, 0.15, true)
	g.m = poker.NewRandom(b)
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
	g.message = "CHOOSE YOUR BET AND PRESS DEAL"

	var s saved
	if b.Load(saveName, &s) {
		g.m.Restore(s.Saved)
		g.autoHold = s.AutoHold
		g.resume()
	}
	g.credits = b.Credits

	go func() {
		a := &advice{
			one:  poker.NewOdds(g.m.Table, 1),
			most: poker.NewOdds(g.m.Table, poker.MaxCoins),
		}
		g.advice.Store(a)
		rtp := a.most.Return()
		g.rtp.Store(&rtp)
	}()
	return g
}

// resume puts back on screen a hand that was still to be drawn to when the
// game was last closed.
func (g *Game) resume() {
	if g.m.Dealt {
		g.shown = true
		g.made = poker.Evaluate(g.m.Hand)
		g.message = "HAND TO FINISH - " + holdPrompt
	}
}

// best returns the cards that are best held in the hand, and what the hand
// is then worth on average, in credits. It reports false if the odds are
// not ready yet.
func (g *Game) best() (held [poker.HandSize]bool, value float64, ok bool) {
	a := g.advice.Load()
	if a == nil {
		return held, 0, false
	}
	if g.m.Coins == poker.MaxCoins {
		held, value = a.most.Best(g.m.Hand)
	} else {
		held, value = a.one.Best(g.m.Hand)
		value *= float64(g.m.Coins)
	}
	return held, value * float64(g.m.CoinValue), true
}

// cardButton is the place of a card in the hand. Clicking a card holds it.
func cardButton(i int) ui.Button {
	return ui.Button{X: cardsX + float64(i)*(ui.CardW+cardGap), Y: cardsY, W: ui.CardW, H: ui.CardH}
}

// The coin values take up the left of the top row of buttons.
const (
	valueW   = 84
	valueGap = 6
)

func valueButton(i int) ui.Button {
	return ui.Button{X: payX + float64(i)*(valueW+valueGap), Y: row1Y, W: valueW, H: buttonH}
}

var (
	betOneButton = ui.Button{X: 480, Y: row1Y, W: 90, H: buttonH}
	betMaxButton = ui.Button{X: 576, Y: row1Y, W: 90, H: buttonH}
	dealButton   = ui.Button{X: 672, Y: row1Y, W: 98, H: row2Y + buttonH - row1Y} // as tall as both rows

	autoButton  = ui.Button{X: 30, Y: row2Y, W: 144, H: buttonH}
	hintButton  = ui.Button{X: 180, Y: row2Y, W: 144, H: buttonH}
	coinButton  = ui.Button{X: 330, Y: row2Y, W: 240, H: buttonH}
	resetButton = ui.Button{X: 576, Y: row2Y, W: 90, H: buttonH}
)

// The keys that hold each card, along the top of the keyboard and on the
// number pad.
var holdKeys = [poker.HandSize][2]ebiten.Key{
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
		g.updatePlay()
	}
	return false
}

func (g *Game) updatePlay() {
	if g.turning {
		g.turnCards()
		return
	}
	// The bank may have changed while the game was not being played.
	g.credits = g.bank.Credits
	if g.holdDue {
		g.holdBest()
	}
	if ui.Pressed(ebiten.KeyEscape) {
		g.scene = sceneMenu
		return
	}

	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	x, y := ebiten.CursorPosition()
	hit := func(b ui.Button) bool { return click && b.Hit(x, y) }
	deal := ui.Pressed(ebiten.KeySpace, ebiten.KeyEnter, ebiten.KeyNumpadEnter) || hit(dealButton)

	if ui.Pressed(ebiten.KeyC) || hit(coinButton) {
		g.insertCoin()
	}
	if ui.Pressed(ebiten.KeyR) || hit(resetButton) {
		g.reset()
	}
	if ui.Pressed(ebiten.KeyA) || hit(autoButton) {
		g.toggleAuto()
	}

	// Between the deal and the draw the cards can be held, and the bet
	// cannot be changed.
	if g.m.Dealt {
		for i, keys := range holdKeys {
			if ui.Pressed(keys[:]...) || hit(cardButton(i)) {
				g.toggleHold(i)
			}
		}
		switch {
		case ui.Pressed(ebiten.KeyH) || hit(hintButton):
			g.showHint()
		case deal:
			g.draw()
		}
		return
	}

	for i, v := range poker.CoinValues {
		if hit(valueButton(i)) {
			g.setCoinValue(v)
		}
	}
	switch {
	case ui.Pressed(ebiten.KeyArrowUp):
		g.setCoinValue(ui.Step(poker.CoinValues, g.m.CoinValue, 1))
	case ui.Pressed(ebiten.KeyArrowDown):
		g.setCoinValue(ui.Step(poker.CoinValues, g.m.CoinValue, -1))
	case ui.Pressed(ebiten.KeyArrowRight):
		g.setCoins(min(g.m.Coins+1, poker.MaxCoins))
	case ui.Pressed(ebiten.KeyArrowLeft):
		g.setCoins(max(g.m.Coins-1, 1))
	case hit(betOneButton):
		// One more coin each time, and back to one after the most.
		g.setCoins(g.m.Coins%poker.MaxCoins + 1)
	case ui.Pressed(ebiten.KeyM) || hit(betMaxButton):
		g.setCoins(poker.MaxCoins)
		g.deal()
	case deal:
		g.deal()
	}
}

// clearWin takes the last hand's win off the display. The cards stay.
func (g *Game) clearWin() {
	g.win = 0
	g.made = poker.Nothing
	g.message = ""
}

func (g *Game) setCoinValue(v int) {
	g.m.CoinValue = v
	g.clearWin()
	g.snd.Play(g.snd.Click)
	g.save()
}

func (g *Game) setCoins(n int) {
	g.m.Coins = n
	g.clearWin()
	g.snd.Play(g.snd.Click)
	g.save()
}

const (
	outOfCredits = "OUT OF CREDITS - INSERT A COIN OR RESET"
	holdPrompt   = "HOLD THE CARDS TO KEEP, THEN DRAW"
)

// reset puts the bank's credits back to the starting amount. The bet is
// left as it is, and so is a hand that is being played.
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

// toggleAuto turns auto hold on or off. Turned on in the middle of a hand,
// it holds the best cards there and then.
func (g *Game) toggleAuto() {
	g.autoHold = !g.autoHold
	g.holdDue = g.autoHold && g.m.Dealt
	g.snd.Play(g.snd.Click)
	g.save()
}

// holdBest holds the best cards in the hand, in place of any that are
// held. If the odds are not ready it is left for later.
func (g *Game) holdBest() {
	held, _, ok := g.best()
	if !ok {
		return
	}
	g.m.Held = held
	g.holdDue = false
	g.hinting = false
	g.message = g.dealMessage()
	g.save()
}

func (g *Game) toggleHold(i int) {
	g.m.ToggleHold(i)
	g.holdDue = false
	g.snd.Play(g.holdSound)
	g.save()
}

// showHint marks the best cards to hold without holding them.
func (g *Game) showHint() {
	held, value, ok := g.best()
	if !ok {
		g.message = "STILL WORKING OUT THE ODDS - TRY AGAIN IN A MOMENT"
		return
	}
	g.hint, g.hinting = held, true
	g.snd.Play(g.snd.Click)
	switch held {
	case [poker.HandSize]bool{}:
		g.message = "BEST PLAY: HOLD NOTHING AND DRAW FIVE CARDS"
	case [poker.HandSize]bool{true, true, true, true, true}:
		g.message = "BEST PLAY: HOLD ALL FIVE CARDS"
	default:
		g.message = fmt.Sprintf("BEST PLAY: HOLD THE CARDS MARKED - WORTH $%.2f ON AVERAGE", value/100)
	}
}

// dealMessage is what is said once a hand has been dealt.
func (g *Game) dealMessage() string {
	prompt := holdPrompt
	if g.autoHold && !g.holdDue {
		prompt = "BEST CARDS HELD - CHANGE THEM OR DRAW"
		if g.m.Held == [poker.HandSize]bool{} {
			prompt = "NOTHING WORTH HOLDING - HOLD CARDS OR DRAW"
		}
	}
	if made := poker.Evaluate(g.m.Hand); made != poker.Nothing {
		return strings.ToUpper(made.String()) + " - " + prompt
	}
	return prompt
}

// deal takes the bet and deals a hand, turning the cards over one by one.
// With too few credits nothing is dealt: the bank is every game's, so it is
// for the player to insert a coin or reset it.
func (g *Game) deal() {
	if _, ok := g.m.Deal(); !ok {
		g.clearWin()
		g.message = "NOT ENOUGH CREDITS - INSERT A COIN OR BET LESS"
		if g.bank.Credits == 0 {
			g.message = outOfCredits
		}
		return
	}
	g.clearWin()
	g.shown, g.drawn, g.hinting = true, false, false
	g.credits = g.bank.Credits
	// Hold the best cards now if that can be done, so that they are saved
	// with the hand.
	g.holdDue = g.autoHold
	if g.holdDue {
		g.holdBest()
	}
	g.message = ""
	g.save()

	for i := range g.turnAt {
		g.turnAt[i] = g.ticks + turnTicks*(i+1)
	}
	g.turning = true
}

// draw replaces the cards that are not held, turning the new ones over one
// by one. The machine settles the hand straight away, so it is safe to save
// even though the cards have yet to show it.
func (g *Game) draw() {
	r, ok := g.m.Draw()
	if !ok {
		return
	}
	g.save()
	g.clearWin()
	g.drawn, g.hinting, g.holdDue = true, false, false
	g.credits = g.bank.Credits - r.Win
	g.win = 0

	n := 0
	for i := range g.turnAt {
		g.turnAt[i] = 0
		if !g.m.Held[i] {
			n++
			g.turnAt[i] = g.ticks + turnTicks*n
		}
	}
	g.turning = true
}

// faceUp reports whether the card at the given place is showing its face.
func (g *Game) faceUp(i int) bool {
	return g.shown && g.ticks >= g.turnAt[i]
}

// turnCards carries on turning the cards over, and when the last one is up
// says what the hand makes.
func (g *Game) turnCards() {
	last := 0
	for _, at := range g.turnAt {
		if at == g.ticks {
			g.snd.Play(g.snd.Card)
		}
		last = max(last, at)
	}
	if g.ticks < last {
		return
	}
	g.turning = false
	g.made = poker.Evaluate(g.m.Hand)
	if !g.drawn {
		g.message = g.dealMessage()
		return
	}

	g.credits = g.bank.Credits
	g.win = g.m.Table.Pay(g.made, g.m.Coins) * g.m.CoinValue
	switch {
	case g.win > 0:
		g.message = fmt.Sprintf("%s - YOU WIN %s!", strings.ToUpper(g.made.String()), bank.Money(g.win))
	case g.bank.Credits == 0:
		g.message = outOfCredits
	default:
		g.message = "NO WIN"
	}
	switch {
	case g.made >= poker.FullHouse:
		g.snd.Play(g.snd.BigWin)
	case g.win > 0:
		g.snd.Play(g.snd.Win)
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(felt)
	ui.DrawBorder(screen, ui.Blue)

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
	ui.DrawText(screen, "VIDEO POKER", ui.ScreenW/2, 70, 48, ui.Yellow, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, strings.ToUpper(g.m.Table.Name), ui.ScreenW/2, 128, 20, ui.White, ui.AlignCenter, ui.AlignStart)

	g.menu.Draw(screen)
	ui.DrawText(screen, "ARROWS move    ENTER or NUMBER choose    ESC main menu", ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "MAKE YOUR CHOICE", ui.ScreenW/2, 350, 24, ui.LightCyan, ui.AlignCenter, ui.AlignStart)

	// A royal flush, for luck.
	const scale = 0.62
	for i := range poker.HandSize {
		c := cards.New(cards.Ten+cards.Rank(i), cards.Spades)
		x := ui.ScreenW/2 + (float64(i)-2.5)*(ui.CardW*scale+10) + 5
		ui.DrawCard(screen, c, x, 420, scale)
	}
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	ui.DrawText(screen, "HOW TO PLAY", ui.ScreenW/2, 26, 32, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	p := g.m.Table
	lines := []string{
		"Choose a coin value and how many coins to bet, then deal.",
		"You are dealt five cards. Hold the ones you want to keep, then draw:",
		"the rest are replaced, and the hand you end up with is paid.",
		"A pair of jacks or better is the lowest hand that pays.",
		fmt.Sprintf("A royal flush pays %d a coin, but %d a coin when you bet %d coins,",
			p.Pays[poker.RoyalFlush], p.RoyalMax, poker.MaxCoins),
		"so the game pays back the most at that bet.",
		"",
		"With AUTO HOLD on, the best cards are held for you as soon as the",
		"hand is dealt. You can still change them before you draw.",
		"With it off, HINT marks the best cards without holding them.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2-270, 76+float64(i)*20, 15, ui.White, ui.AlignStart, ui.AlignStart)
	}

	keys := [][2]string{
		{"SPACE or ENTER", "deal, then draw"},
		{"1 to 5", "hold a card, or let it go"},
		{"H", "hint: mark the best cards to hold"},
		{"A", "auto hold on or off"},
		{"UP / DOWN", "change the coin value"},
		{"LEFT / RIGHT", "change the number of coins"},
		{"M", "bet the most coins and deal"},
		{"C", "insert a coin for more credits"},
		{"R", "reset your credits"},
		{"S", "sound on or off"},
		{"ESC", "back to the menu"},
	}
	for i, k := range keys {
		y := 296 + float64(i)*19
		ui.DrawText(screen, k[0], ui.ScreenW/2-70, y, 14, ui.LightRed, ui.AlignEnd, ui.AlignStart)
		ui.DrawText(screen, k[1], ui.ScreenW/2-45, y, 14, ui.White, ui.AlignStart, ui.AlignStart)
	}
	ui.DrawText(screen, "The cards and the buttons on screen can be clicked with the mouse.", ui.ScreenW/2, 518, 13, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 546, 20, ui.LightCyan, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawAbout(screen *ebiten.Image) {
	ui.DrawText(screen, "ABOUT THIS GAME", ui.ScreenW/2, 60, 36, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	p := g.m.Table
	returns := "Working out what it pays back..."
	if rtp := g.rtp.Load(); rtp != nil {
		returns = fmt.Sprintf("it pays back %.2f%% of what is bet over a long run.", *rtp*100)
	}
	lines := []string{
		"Video Poker is written in Go with Ebitengine.",
		"",
		fmt.Sprintf("The game is %s with the %d/%d paytable, named for",
			p.Name, p.Pays[poker.FullHouse], p.Pays[poker.Flush]),
		"what it pays for a full house and a flush.",
		"",
		fmt.Sprintf("Played perfectly at %d coins,", poker.MaxCoins),
		returns,
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
	g.drawPaytable(screen)

	for i, c := range g.m.Hand {
		b := cardButton(i)
		if !g.faceUp(i) {
			ui.DrawCardBack(screen, b.X, b.Y, 1)
			continue
		}
		ui.DrawCard(screen, c, b.X, b.Y, 1)
		if g.m.Held[i] {
			x := b.X + (ui.CardW-tagW)/2
			ui.FillRect(screen, x, tagY, tagW, tagH, ui.Yellow)
			ui.DrawText(screen, "HELD", x+tagW/2, tagY+tagH/2, 14, ui.Black, ui.AlignCenter, ui.AlignCenter)
		}
		if g.hinting && g.hint[i] {
			ui.FrameRect(screen, b.X-4, b.Y-4, ui.CardW+8, ui.CardH+8, 4, ui.LightCyan)
		}
	}

	ui.DrawText(screen, g.message, ui.ScreenW/2, messageY, 16, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	drawStat(screen, "CREDITS", bank.Dollars(g.credits), 0, ui.LightGreen)
	drawStat(screen, "COIN VALUE", bank.Money(g.m.CoinValue), 1, ui.White)
	drawStat(screen, "COINS", fmt.Sprint(g.m.Coins), 2, ui.White)
	drawStat(screen, "BET", bank.Money(g.m.Bet()), 3, ui.White)
	drawStat(screen, "WIN", bank.Money(g.win), 4, ui.Yellow)

	// The bet cannot be changed while a hand is being played.
	locked := g.m.Dealt || g.turning
	for i, v := range poker.CoinValues {
		ui.DrawButton(screen, valueButton(i), v == g.m.CoinValue, locked, 12, bank.Money(v), "COIN")
	}
	ui.DrawButton(screen, betOneButton, false, locked, 13, "BET", "ONE")
	ui.DrawButton(screen, betMaxButton, false, locked, 13, "BET", "MAX")

	auto := "OFF"
	if g.autoHold {
		auto = "ON"
	}
	ui.DrawButton(screen, autoButton, g.autoHold, false, 13, "AUTO HOLD", auto)
	ui.DrawButton(screen, hintButton, false, !g.m.Dealt || g.turning, 15, "HINT")

	label := "DEAL"
	if g.m.Dealt {
		label = "DRAW"
	}
	b := dealButton
	ui.FillRect(screen, b.X, b.Y, b.W, b.H, ui.Green)
	ui.FrameRect(screen, b.X, b.Y, b.W, b.H, 2, ui.LightGreen)
	ui.DrawText(screen, label, b.X+b.W/2, b.Y+b.H/2, 26, ui.White, ui.AlignCenter, ui.AlignCenter)

	coin := fmt.Sprintf("INSERT COIN +$%d", bank.CoinCredits/100)
	ui.DrawCoinButton(screen, coinButton, 14, coin)
	ui.DrawResetButton(screen, resetButton, 13)

	sound := "ON"
	if !g.snd.On {
		sound = "OFF"
	}
	hint := fmt.Sprintf("SPACE deal/draw    1-5 hold    H hint    A auto hold    ARROWS bet    M max bet    C coin    R reset    S sound %s    ESC menu", sound)
	ui.DrawText(screen, hint, ui.ScreenW/2, 562, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
}

// drawPaytable draws what each hand pays, in credits at the coin value
// being played, for each number of coins. The column for the coins being
// bet is lit, and so is the row for the hand on show.
func (g *Game) drawPaytable(screen *ebiten.Image) {
	ui.FillRect(screen, payX, payY, payW, payH, ui.Black)
	colX := func(coins int) float64 { return payX + payName + float64(coins-1)*payColW }
	ui.FillRect(screen, colX(g.m.Coins), payY, payColW, payH, ui.Red)

	// A winning hand flashes once it has been paid.
	lit := poker.Nothing
	if g.shown && !g.turning && (!g.drawn || g.ticks/20%2 == 0) {
		lit = g.made
	}

	for row := range payRows {
		made := poker.RoyalFlush - poker.Category(row)
		y := float64(payY + row*payRowH)
		ink := color.Color(ui.Yellow)
		if made == lit {
			ui.FillRect(screen, payX, y, payW, payRowH, ui.White)
			ink = ui.Black
		}
		cy := y + payRowH/2
		ui.DrawText(screen, strings.ToUpper(made.String()), payX+12, cy, 13, ink, ui.AlignStart, ui.AlignCenter)
		for coins := 1; coins <= poker.MaxCoins; coins++ {
			pay := g.m.Table.Pay(made, coins) * g.m.CoinValue
			ui.DrawText(screen, bank.Money(pay), colX(coins)+payColW-12, cy, 13, ink, ui.AlignEnd, ui.AlignCenter)
		}
	}

	for coins := 1; coins <= poker.MaxCoins; coins++ {
		ui.DrawLine(screen, colX(coins), payY, colX(coins), payY+payH, 1, ui.Yellow)
	}
	ui.FrameRect(screen, payX, payY, payW, payH, 2, ui.Yellow)
}

// drawStat draws a labelled number under the given card.
func drawStat(screen *ebiten.Image, label, value string, column int, c color.Color) {
	b := cardButton(column)
	x := b.X + ui.CardW/2
	ui.DrawText(screen, label, x, statY, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, value, x, statY+13, 24, c, ui.AlignCenter, ui.AlignStart)
}
