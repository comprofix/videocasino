// Package baccarat is Baccarat: the player's hand against the banker's,
// dealt from a shoe of eight decks and bet on for pretend credits from the
// bank. Its rules are in the table package.
package baccarat

import (
	"fmt"
	"image/color"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/baccarat/table"
	"videocasino/palette"
	"videocasino/shuffle"
	"videocasino/ui"
)

const (
	cardScale = 0.75
	cardW     = ui.CardW * cardScale
	cardH     = ui.CardH * cardScale
	cardStep  = 96 // how far each card of a hand sits from the one before: clear of it
	burnStep  = 28 // and each burnt card, which lies face down

	// The two hands lie side by side under the row of the last hands, with
	// the shoe in the corner beyond the banker's.
	historyY = 52
	handsY   = 102
	playerX  = 200 // the middle of the player's hand,
	bankerX  = 540 // and of the banker's
	messageY = 244

	// The bets lie in a row under the hands, and the figures under them.
	betsY = 270
	betsH = 92
	statY = 374

	// The two rows of buttons at the bottom.
	rowAY   = 448
	rowBY   = 494
	buttonH = 40

	// dealTicks is the time between one card being dealt and the next, and
	// burnPause how many cards' worth of time the burnt cards lie on the
	// table before the hand is dealt.
	dealTicks = 16
	burnPause = 3
)

// felt is the table: a deep purple, so that the game has a look of its own.
var felt = color.RGBA{56, 16, 80, 255}

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

	// What the bets pay back from a full shoe takes a moment to work out,
	// so that is done in the background while the program starts. Until it
	// is ready this is nil.
	odds atomic.Pointer[table.Odds]

	// Where the pointer was last, so that it moves the marker only when it
	// is itself moved.
	seen         bool
	lastX, lastY int

	// The machine deals and settles a hand all at once. The cards are then
	// shown one by one: those burnt first, if the shoe is a new one, and
	// then the hand, with the result once the last card is down.
	result    table.Result
	dealt     bool // a hand has been dealt since the game was opened
	revealing bool
	revealAt  int  // the tick at which the next card is shown
	shownStep int  // how many steps of the deal are on show
	shown     bool // the result of the last hand is on show

	// The cut card is dealt onto the table when it comes out of the shoe,
	// and lies there until the shoe is shuffled.
	cutShown bool

	// The shuffle is shown before the first hand from a new shoe is dealt.
	shuffling    bool
	shuffleStart int // the tick it began at
	shuffleFrom  int // the cards that were left in the shoe

	// The sounds that are this game's own.
	chipSound, cutSound, riffleSound []byte

	credits int // credits on display; a win is added once the cards are down
	win     int
	message string
	confirm ui.Confirm // asks before the credits are reset

	saveFailed bool
}

// New returns the game, playing from the given bank and picking up whatever
// it last kept there.
func New(b *bank.Bank, snd *ui.Sounds) *Game {
	g := &Game{snd: snd, bank: b, mark: table.Player}
	g.menu = &ui.Menu{
		Items: []ui.MenuItem{
			{Title: "HOW TO PLAY GAME"},
			{Title: "PLAY GAME"},
			{Title: "ABOUT THIS GAME"},
			{Title: "BACK TO MAIN MENU"},
		},
		Columns: 1, Rows: 4,
		Y: 190, RowH: 36,
		Number: palette.White, Title: palette.Red,
	}
	g.m = table.NewRandom(b)
	g.message = "PLACE YOUR BETS"

	g.chipSound = ui.Tone(1480, 22, 0.14, true)
	g.cutSound = append(ui.Tone(880, 60, 0.2, true), ui.Tone(660, 110, 0.2, true)...)
	g.riffleSound = shuffle.Sound()

	var s table.Saved
	if b.Load(saveName, &s) {
		g.m.Restore(s)
	}
	// A cut card that came out before the game was closed is on the table.
	g.cutShown = g.m.CutOut()
	g.credits = b.Credits

	go func() {
		o := table.NewOdds()
		g.odds.Store(&o)
	}()
	return g
}

// The places of the bets, across the table: the two hands wider than the
// rest.
func betButton(b table.Bet) ui.Button {
	const gap, narrow, wide = 5, 120, 180
	x := [table.Bets]float64{30, 30 + narrow + gap, 30 + narrow + wide + 2*gap, 30 + 2*narrow + wide + 3*gap, 30 + 2*narrow + 2*wide + 4*gap}
	w := [table.Bets]float64{narrow, wide, narrow, wide, narrow}
	return ui.Button{X: x[b], Y: betsY, W: w[b], H: betsH}
}

// The buttons under the table.
func chipButton(i int) ui.Button {
	return ui.Button{X: 30 + float64(i)*76, Y: rowAY, W: 70, H: buttonH}
}

var (
	clearButton = ui.Button{X: 420, Y: rowAY, W: 80, H: buttonH}
	rebetButton = ui.Button{X: 506, Y: rowAY, W: 84, H: buttonH}
	dealButton  = ui.Button{X: 600, Y: rowAY, W: 170, H: rowBY + buttonH - rowAY} // as tall as two rows
	coinButton  = ui.Button{X: 30, Y: rowBY, W: 240, H: buttonH}
	resetButton = ui.Button{X: 276, Y: rowBY, W: 90, H: buttonH}
	shoeBox     = ui.Button{X: 620, Y: 40, W: 150, H: 26}
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
	// The deal key or a click cuts short the shuffle, and the deal.
	skip := click || ui.Pressed(ebiten.KeySpace, ebiten.KeyEnter, ebiten.KeyNumpadEnter)
	if g.shuffling {
		g.shuffleStep(skip)
		return
	}
	if g.revealing {
		g.reveal(skip)
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
	for b := table.Bet(0); b < table.Bets; b++ {
		if !betButton(b).Hit(x, y) || !(moved || click || away) {
			continue
		}
		g.mark = b
		switch {
		case click:
			g.place()
		case away:
			g.remove()
		}
	}

	hit := func(b ui.Button) bool { return click && b.Hit(x, y) }
	for i, n := range table.ChipChoices {
		if hit(chipButton(i)) || ui.Pressed(chipKeys[i][:]...) {
			g.setChip(n)
		}
	}
	switch {
	case hit(coinButton), ui.Pressed(ebiten.KeyC):
		g.insertCoin()
	case hit(resetButton), ui.Pressed(ebiten.KeyR):
		g.askReset()
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
		g.mark = max(g.mark-1, 0)
	case ui.Pressed(ebiten.KeyArrowRight):
		g.mark = min(g.mark+1, table.Bets-1)
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
	g.clearWin()
	if !g.m.Place(g.mark) {
		switch {
		case g.bank.Credits == 0:
			g.message = outOfCredits
		case g.m.Bets[g.mark]+g.m.Chip > table.MaxBet:
			g.message = fmt.Sprintf("NO MORE THAN %s ON ONE BET", bank.Money(table.MaxBet))
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
	if !g.m.Remove(g.mark) {
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

// rebet puts the bets of the last hand back on the table.
func (g *Game) rebet() {
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

// reset puts the bank's credits back to the starting amount. The bets on
// the table and the shoe are left as they are.
func (g *Game) reset() {
	g.bank.Reset()
	g.credits = g.bank.Credits
	g.clearWin()
	g.message = "CREDITS RESET - PLACE YOUR BETS"
	g.snd.Play(g.snd.Click)
	g.save()
}

// insertCoin adds credits to the bank, so play can go on after they run
// out. A coin that would go over the limit is refused.
func (g *Game) insertCoin() {
	if !g.bank.InsertCoin() {
		g.message = "COIN REFUSED - LIMIT IS " + bank.Money(bank.CoinLimit)
		return
	}
	g.credits = g.bank.Credits
	g.message = fmt.Sprintf("COIN INSERTED - %s ADDED", bank.Money(bank.CoinCredits))
	g.snd.Play(g.snd.Coin)
	g.save()
}

// pressDeal deals the next hand. If the shoe's last hand has been dealt it
// is shuffled first, and that is shown before the cards are dealt. With no
// bet that the bank can cover there is no shuffle to show: nothing is dealt.
func (g *Game) pressDeal() {
	bet := g.m.Total()
	if bet == 0 {
		bet = g.m.LastTotal() // the last bets will be made again
	}
	if !g.m.Finished || bet == 0 || bet > g.bank.Credits {
		g.deal()
		return
	}
	g.clearWin()
	g.message = "SHUFFLING THE SHOE"
	g.shuffling = true
	g.shuffleStart = g.ticks
	g.shuffleFrom = g.m.CardsLeft()
}

// shuffleStep carries the shuffle on by a tick, and deals the hand when it
// is over or has been cut short.
func (g *Game) shuffleStep(skip bool) {
	t := g.ticks - g.shuffleStart
	if skip || t >= shuffle.Full.Ticks() {
		g.shuffling = false
		g.deal()
		return
	}
	if shuffle.Full.Falls(t) {
		g.snd.Play(g.riffleSound)
	}
}

// deal has the machine deal and settle a hand straight away, then shows its
// cards one by one. With nothing on the table the bets of the last hand are
// made again first. With too few credits nothing is dealt: the bank is
// every game's, so it is for the player to insert a coin or reset it.
func (g *Game) deal() {
	if g.m.Total() == 0 {
		g.m.Rebet()
	}
	shuffled := g.m.Finished
	res, ok := g.m.Deal()
	if !ok {
		g.clearWin()
		switch {
		case g.bank.Credits == 0:
			g.message = outOfCredits
		case g.m.Total() == 0 && g.m.LastTotal() == 0:
			g.message = "PLACE A BET BEFORE YOU DEAL"
		default:
			g.message = "NOT ENOUGH CREDITS - INSERT A COIN OR BET LESS"
		}
		return
	}
	// The machine has already settled the hand, so it is safe to save now
	// even though the cards have yet to be shown.
	g.save()
	g.clearWin()
	if shuffled {
		g.cutShown = false // shuffled back into the shoe
	}
	g.result = res
	g.dealt = true
	g.revealing = true
	g.shownStep = 0
	g.revealAt = g.ticks + dealTicks
	g.credits = g.bank.Credits - res.Win
	g.message = "NO MORE BETS"
	if res.Burnt > 0 {
		g.message = "A NEW SHOE - BURNING THE FIRST CARDS"
	}
}

// The deal is shown as a run of steps, a card to each. If the shoe is a new
// one the card turned up comes first and the cards burnt for it after, then
// a pause while they lie there. The cards of the hand follow.

// burnSteps is how many steps of the deal are taken up with burning cards.
func (g *Game) burnSteps() int {
	if g.result.Burnt == 0 {
		return 0
	}
	return 1 + g.result.Burnt + burnPause
}

// dealSteps is how many steps the whole deal takes.
func (g *Game) dealSteps() int {
	return g.burnSteps() + len(g.result.Order())
}

// burning reports whether the burnt cards are what is on show, and if so
// how many of them are down, the card turned up among them.
func (g *Game) burning() (down int, ok bool) {
	if !g.revealing || g.shownStep >= g.burnSteps() {
		return 0, false
	}
	return min(g.shownStep, 1+g.result.Burnt), true
}

// handShown is how many cards of the hand are on show.
func (g *Game) handShown() int {
	if !g.revealing {
		return len(g.result.Order())
	}
	return max(g.shownStep-g.burnSteps(), 0)
}

// reveal carries the deal on by a tick, showing the next card when it is
// due, and the result once the last is down. The cut card is dealt onto the
// table in its turn.
func (g *Game) reveal(skip bool) {
	if skip {
		// Everything is shown at once.
		g.shownStep = g.dealSteps()
		if g.result.CutAfter >= 0 {
			g.cutShown = true
		}
		g.snd.Play(g.snd.Card)
		g.land()
		return
	}
	if g.ticks < g.revealAt {
		return
	}
	g.revealAt = g.ticks + dealTicks
	if g.shownStep >= g.dealSteps() {
		g.land()
		return
	}
	g.shownStep++
	burnt := 1 + g.result.Burnt
	switch {
	case g.shownStep <= burnt, g.shownStep > g.burnSteps():
		g.snd.Play(g.snd.Card)
	case g.shownStep == g.burnSteps():
		g.message = "NO MORE BETS"
	}
	if g.result.CutAfter >= 0 && g.handShown() == g.result.CutAfter && !g.cutShown {
		g.cutShown = true
		g.snd.Play(g.cutSound)
	}
}

// land shows the result of the hand, now that its cards are down.
func (g *Game) land() {
	g.revealing = false
	g.shown = true
	g.credits = g.bank.Credits
	g.win = g.result.Win

	r := &g.result
	p, b := table.Total(r.Player), table.Total(r.Banker)
	what := fmt.Sprintf("TIE AT %d", p)
	switch r.Outcome {
	case table.PlayerWins:
		what = fmt.Sprintf("PLAYER WINS %d TO %d", p, b)
	case table.BankerWins:
		what = fmt.Sprintf("BANKER WINS %d TO %d", b, p)
	}
	// What was handed back on a tie is not a win.
	won := 0
	for i, paid := range r.Paid {
		if paid > r.Bets[i] {
			won += paid
		}
	}
	switch {
	case won == 0 && g.win > 0:
		g.message = what + " - YOUR BETS ARE HANDED BACK"
	case won == 0:
		g.message = what + " - NO WIN"
	case g.win >= 5*r.Bet:
		g.message = fmt.Sprintf("%s - YOU WIN %s", what, bank.Money(g.win))
		g.snd.Play(g.snd.BigWin)
	default:
		g.message = fmt.Sprintf("%s - YOU WIN %s", what, bank.Money(g.win))
		g.snd.Play(g.snd.Win)
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(felt)
	ui.DrawBorder(screen, palette.Fog)

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
	ui.DrawText(screen, "BACCARAT", ui.ScreenW/2, 70, 48, palette.Gold, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "NO COMMISSION", ui.ScreenW/2, 128, 20, palette.White, ui.AlignCenter, ui.AlignStart)

	g.menu.Draw(screen)
	ui.DrawText(screen, "ARROWS move    ENTER or NUMBER choose    ESC main menu", ui.ScreenW/2, 550, 11, palette.Fog, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "MAKE YOUR CHOICE", ui.ScreenW/2, 350, 24, palette.Seafoam, ui.AlignCenter, ui.AlignStart)

	// A natural nine, for luck.
	const scale = 0.62
	ui.DrawCard(screen, cards.New(cards.Four, cards.Hearts), ui.ScreenW/2-ui.CardW*scale-5, 420, scale)
	ui.DrawCard(screen, cards.New(cards.Five, cards.Spades), ui.ScreenW/2+5, 420, scale)
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	ui.DrawText(screen, "HOW TO PLAY", ui.ScreenW/2, 22, 30, palette.Gold, ui.AlignCenter, ui.AlignStart)

	p := table.Pays
	lines := []string{
		"Two hands are dealt, the player's and the banker's. You play neither:",
		"you bet on which will win, on a tie, or on either starting with a pair.",
		"An ace counts 1, tens and pictures nothing, and only the last figure of",
		"a hand counts, so a seven and an eight make 5. The hand nearer 9 wins.",
		fmt.Sprintf("Each hand gets two cards. If either makes %d or 9, a natural, that is all.", table.Natural),
		"If not, each may be dealt a third card by fixed rules. Nobody chooses.",
		fmt.Sprintf("PLAYER pays %d to 1. BANKER pays %d to 1 with no commission, but only", p[table.Player], p[table.Banker]),
		fmt.Sprintf("half when the banker wins with a %d. TIE pays %d to 1, and bets on the", table.BankerSix, p[table.Tie]),
		fmt.Sprintf("hands are handed back. A PAIR pays %d to 1 on a hand's first two cards.", p[table.PlayerPair]),
		"After a shuffle a card is turned up and as many cards burnt as it counts.",
		fmt.Sprintf("The %d decks are dealt to a cut card and one hand more, then shuffled.", table.Decks),
		"DEAL with nothing on the table makes the last bets again.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2-275, 60+float64(i)*18, 15, palette.White, ui.AlignStart, ui.AlignStart)
	}

	keys := [][2]string{
		{"SPACE", "deal"},
		{"LEFT / RIGHT", "move the marker from bet to bet"},
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
		y := 288 + float64(i)*18
		ui.DrawText(screen, k[0], ui.ScreenW/2-70, y, 14, palette.Red, ui.AlignEnd, ui.AlignStart)
		ui.DrawText(screen, k[1], ui.ScreenW/2-45, y, 14, palette.White, ui.AlignStart, ui.AlignStart)
	}
	ui.DrawText(screen, "With the mouse, click a bet to put a chip down and right click to take it off.", ui.ScreenW/2, 496, 13, palette.Fog, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 546, 20, palette.Seafoam, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawAbout(screen *ebiten.Image) {
	ui.DrawText(screen, "ABOUT THIS GAME", ui.ScreenW/2, 50, 36, palette.Gold, ui.AlignCenter, ui.AlignStart)

	lines := []string{
		"Baccarat is written in Go with Ebitengine.",
		"",
		fmt.Sprintf("It is dealt from %d decks, with no commission on the banker.", table.Decks),
		"",
		"Working out what it pays back...",
	}
	if o := g.odds.Load(); o != nil {
		lines = append(lines[:4],
			"From a full shoe, of what is bet on it",
			fmt.Sprintf("PLAYER pays back %.2f%% and BANKER %.2f%%,", o.Return(table.Player)*100, o.Return(table.Banker)*100),
			fmt.Sprintf("TIE %.2f%% and either PAIR %.2f%%.", o.Return(table.Tie)*100, o.Return(table.PlayerPair)*100),
			"",
			"They change a little from hand to hand as the shoe runs down.",
		)
	}
	lines = append(lines,
		"",
		"It is played with pretend credits only.",
		"No real money goes in or comes out.",
	)
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2, 124+float64(i)*28, 18, palette.White, ui.AlignCenter, ui.AlignStart)
	}
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 500, 20, palette.Seafoam, ui.AlignCenter, ui.AlignStart)
}

// outcomeInk is the colour a hand's outcome goes by: the player's blue, the
// banker's red and a tie green.
func outcomeInk(o table.Outcome) color.Color {
	switch o {
	case table.PlayerWins:
		return palette.Cobalt
	case table.BankerWins:
		return palette.Crimson
	}
	return palette.Forest
}

func (g *Game) drawPlay(screen *ebiten.Image) {
	g.drawHistory(screen)
	g.drawShoe(screen)

	// While the shoe is being shuffled the table is clear. A new shoe's
	// burnt cards are then shown before the hand is.
	switch down, burning := g.burning(); {
	case g.shuffling:
		shuffle.Full.Draw(screen, g.ticks-g.shuffleStart, (playerX+bankerX)/2, handsY+cardH/2, cardScale)
	case burning:
		g.drawBurn(screen, down)
	default:
		g.drawHands(screen)
	}
	ui.DrawText(screen, g.message, ui.ScreenW/2, messageY, 15, palette.Gold, ui.AlignCenter, ui.AlignStart)

	g.drawBets(screen)

	// Once the hand is dealt the bets are off the table and in play.
	bet := g.m.Total()
	if g.revealing {
		bet = g.result.Bet
	}
	stat := func(i int, label, v string, c color.Color) {
		x := 150 + float64(i)*250
		ui.DrawText(screen, label, x, statY, 11, palette.Fog, ui.AlignCenter, ui.AlignStart)
		ui.DrawText(screen, v, x, statY+13, 22, c, ui.AlignCenter, ui.AlignStart)
	}
	stat(0, "CREDITS", bank.Dollars(g.credits), palette.Leaf)
	stat(1, "TOTAL BET", bank.Money(bet), palette.Gold)
	stat(2, "WIN", bank.Money(g.win), palette.White)

	busy := g.revealing || g.shuffling
	for i, n := range table.ChipChoices {
		ui.DrawButton(screen, chipButton(i), n == g.m.Chip, busy, 11, bank.Money(n), "CHIP")
	}
	ui.DrawButton(screen, clearButton, false, busy, 13, "CLEAR")
	ui.DrawButton(screen, rebetButton, false, busy || g.m.LastTotal() == 0, 13, "REBET")

	ui.FillRect(screen, dealButton.X, dealButton.Y, dealButton.W, dealButton.H, palette.Forest)
	ui.FrameRect(screen, dealButton.X, dealButton.Y, dealButton.W, dealButton.H, 2, palette.Leaf)
	ui.DrawText(screen, "DEAL", dealButton.X+dealButton.W/2, dealButton.Y+dealButton.H/2, 30, palette.White, ui.AlignCenter, ui.AlignCenter)

	coinLabel := fmt.Sprintf("INSERT COIN +$%d", bank.CoinCredits/100)
	ui.DrawCoinButton(screen, coinButton, 14, coinLabel)
	ui.DrawResetButton(screen, resetButton, 13)

	sound := "ON"
	if !g.snd.On {
		sound = "OFF"
	}
	hint := fmt.Sprintf("SPACE deal   LEFT/RIGHT move   ENTER bet   BACKSPACE take off   1-5 chip   X clear   B rebet   C coin   R reset   S sound %s   ESC menu", sound)
	ui.DrawText(screen, hint, ui.ScreenW/2, 550, 11, palette.Fog, ui.AlignCenter, ui.AlignStart)
}

// drawHistory draws how the hands of this shoe have gone, the newest first.
func (g *Game) drawHistory(screen *ebiten.Image) {
	// While the cards are being dealt the hand they make is not shown.
	history := g.m.History
	if g.revealing {
		history = history[1:]
	}
	ui.DrawText(screen, "LAST HANDS", 30, 24, 11, palette.Fog, ui.AlignStart, ui.AlignStart)
	for i, o := range history {
		x := 42 + float64(i)*26
		ui.FillCircle(screen, x, historyY, 11, palette.White)
		ui.FillCircle(screen, x, historyY, 10, outcomeInk(o))
		ui.DrawText(screen, [...]string{"P", "B", "T"}[o], x, historyY, 12, palette.White, ui.AlignCenter, ui.AlignCenter)
	}
}

// drawShoe draws the shoe in the corner of the table, and the cut card
// beside it once it has come out.
func (g *Game) drawShoe(screen *ebiten.Image) {
	// During a shuffle the shoe fills up again.
	full := float64(g.m.CardsLeft()) / table.ShoeSize
	if g.shuffling {
		full = float64(g.shuffleFrom) / table.ShoeSize
		full += (1 - full) * shuffle.Full.Done(g.ticks-g.shuffleStart)
	}

	// The cut card stands that many cards in from the front of what is
	// left. Once it has come out it is gone from the shoe.
	note, ink := "", color.Color(palette.Fog)
	cut := -1.0
	switch {
	case g.shuffling:
		note, ink = "SHUFFLING", palette.Gold
	case g.m.Finished && !g.revealing:
		note, ink = "SHOE DONE: SHUFFLE NEXT", palette.Gold
	case g.cutShown:
		note, ink = "CUT CARD OUT: ONE HAND MORE", palette.Gold
	case !g.m.CutOut():
		cut = float64(g.m.ToCut) / table.ShoeSize
		if g.m.Fresh {
			note = "NEW SHOE"
		}
	}
	ui.DrawShoe(screen, shoeBox, full, cut, palette.Fog, note, ink)
	if g.cutShown && !g.shuffling {
		const scale = 0.5
		ui.DrawCutCard(screen, shoeBox.X+shoeBox.W-ui.CardW*scale, shoeBox.Y+shoeBox.H+22, scale)
	}
}

// drawBurn draws the cards burnt from a new shoe: the one turned up, and
// down of the rest lying face down beside it.
func (g *Game) drawBurn(screen *ebiten.Image, down int) {
	const x = 80
	label := fmt.Sprintf("BURNING %d CARDS", g.result.Burnt)
	if g.result.Burnt == 1 {
		label = "BURNING 1 CARD"
	}
	ui.DrawText(screen, label, x, handsY-18, 13, palette.White, ui.AlignStart, ui.AlignStart)
	if down == 0 {
		return
	}
	ui.DrawCard(screen, g.result.Burn, x, handsY, cardScale)
	for i := range down - 1 {
		ui.DrawCardBack(screen, x+cardW+20+float64(i)*burnStep, handsY, cardScale)
	}
}

// drawHands draws the two hands, as far as they have been dealt. Until the
// first deal the cards lie face down.
func (g *Game) drawHands(screen *ebiten.Image) {
	// The cards are dealt to the hands in turn.
	shown := g.result.Order()[:min(g.handShown(), len(g.result.Order()))]
	var player, banker []cards.Card
	for i, c := range shown {
		// The first four go to each hand in turn. A fifth is the player's
		// third card if the player has one, or else the banker's.
		toPlayer := i%2 == 0
		if i == 4 {
			toPlayer = len(g.result.Player) == 3
		}
		if toPlayer {
			player = append(player, c)
		} else {
			banker = append(banker, c)
		}
	}

	// Once the hand is over each is marked with how it went.
	hand := func(name string, cx float64, dealt []cards.Card, went string) {
		label := name
		if len(dealt) > 0 {
			label = fmt.Sprintf("%s: %d", name, table.Total(dealt))
		}
		ink := color.Color(palette.White)
		if went != "" {
			label += " - " + went
			ink = palette.Gold
		}
		ui.DrawText(screen, label, cx, handsY-18, 13, ink, ui.AlignCenter, ui.AlignStart)

		if !g.dealt {
			for i := range 2 {
				ui.DrawCardBack(screen, cx-(cardW+cardStep)/2+float64(i)*cardStep, handsY, cardScale)
			}
			return
		}
		// A hand keeps its place as it grows: it is set out for the cards
		// it will have.
		n := 2
		if len(dealt) > 2 {
			n = 3
		}
		x := cx - (cardW+float64(n-1)*cardStep)/2
		for i, c := range dealt {
			ui.DrawCard(screen, c, x+float64(i)*cardStep, handsY, cardScale)
		}
	}
	playerWent, bankerWent := "", ""
	if g.dealt && !g.revealing {
		switch g.result.Outcome {
		case table.PlayerWins:
			playerWent = "WINS"
		case table.BankerWins:
			bankerWent = "WINS"
		default:
			playerWent, bankerWent = "TIE", "TIE"
		}
	}
	hand("PLAYER", playerX, player, playerWent)
	hand("BANKER", bankerX, banker, bankerWent)
}

// drawBets draws the bets across the table, the chips on them and the
// marker.
func (g *Game) drawBets(screen *ebiten.Image) {
	// The bets in play stay on the table while the cards are dealt.
	// Afterwards the ones that were paid are left on show until the player
	// moves on.
	bets, flash := g.m.Bets, false
	switch {
	case g.revealing:
		bets = g.result.Bets
	case g.shown && g.m.Total() == 0:
		bets, flash = g.result.Bets, true
	}
	lit := flash && g.ticks/15%2 == 0

	fills := [table.Bets]color.Color{table.Player: palette.Cobalt, table.Tie: palette.Forest, table.Banker: palette.Crimson}
	for b := table.Bet(0); b < table.Bets; b++ {
		box := betButton(b)
		if fills[b] != nil {
			ui.FillRect(screen, box.X, box.Y, box.W, box.H, fills[b])
		}
		ui.FrameRect(screen, box.X, box.Y, box.W, box.H, 1, palette.Fog)

		cx := box.X + box.W/2
		ui.DrawText(screen, b.String(), cx, box.Y+8, 15, palette.White, ui.AlignCenter, ui.AlignStart)
		ui.DrawText(screen, fmt.Sprintf("PAYS %d TO 1", table.Pays[b]), cx, box.Y+29, 11, palette.White, ui.AlignCenter, ui.AlignStart)
		if b == table.Banker {
			ui.DrawText(screen, fmt.Sprintf("HALF ON A %d", table.BankerSix), cx, box.Y+43, 11, palette.Gold, ui.AlignCenter, ui.AlignStart)
		}

		paid := flash && g.result.Paid[b] > 0
		if paid && lit {
			ui.FrameRect(screen, box.X+1, box.Y+1, box.W-2, box.H-2, 4, palette.Gold)
		}
		if b == g.mark && !g.revealing && !g.shuffling {
			ui.FrameRect(screen, box.X+1, box.Y+1, box.W-2, box.H-2, 2, palette.White)
		}
		if bets[b] > 0 && (!flash || paid) {
			ui.DrawChip(screen, cx, box.Y+box.H-18, bets[b], table.ChipChoices, paid && lit)
		}
	}
}
