// Package paigow is Pai Gow Poker: seven cards set as a high hand of five
// and a low hand of two, played against the dealer's seven set the house
// way, for pretend credits from the bank. Its rules are in the table
// package.
package paigow

import (
	"fmt"
	"image/color"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/paigow/table"
	"videocasino/shuffle"
	"videocasino/ui"
)

const (
	cardScale = 0.55
	cardW     = ui.CardW * cardScale
	cardH     = ui.CardH * cardScale
	cardStep  = 74 // how far each card of a row sits from the one before: clear of it
	setGap    = 20 // the space between a high hand and its low hand

	// The table takes up the left of the screen: the dealer's seven at the
	// top, the player's under them, and the bet under those. How the game
	// pays is set out down the right, with the figures.
	tableX   = 300 // the middle of the table
	dealerY  = 36
	playerY  = 184
	lowDrop  = 24 // how far a card chosen for the low hand drops
	messageY = 318
	panelX   = 590
	panelW   = 180

	// The two rows of buttons at the bottom.
	rowAY   = 448
	rowBY   = 494
	buttonH = 40

	// dealTicks is the time between one card being dealt, or turned over,
	// and the next.
	dealTicks = 12
)

// The table: charcoal-black felt with a red edge, for the game's roots in
// the Chinese game of dominoes it is named after.
var (
	felt    = color.RGBA{26, 26, 30, 255}
	redEdge = color.RGBA{178, 28, 28, 255}
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

	// While a hand is set the player's cards lie in a row, highest first;
	// order holds their places in the hand in that order. low is the places
	// of those chosen for the low hand, and mark the card the marker is on,
	// as a place in the row.
	order [table.HandSize]int
	low   []int
	mark  int

	// The deck is shuffled before every hand, and that is shown before the
	// cards are dealt.
	shuffling    bool
	shuffleStart int // the tick it began at

	// The player's cards are dealt one by one, and so are the dealer's
	// turned over once the hand is played. The machine has settled the hand
	// by then, and the result is shown once the last card is up.
	showing     bool // cards are being dealt or turned over
	showAt      int  // the tick at which the next is shown
	playerShown int
	dealerShown int

	// result is the hand last finished, and over is set while it is on
	// show: from when it ends until the next is dealt.
	result table.Result
	over   bool

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

// resume puts back on screen a hand that was still to be set when the game
// was last closed.
func (g *Game) resume() {
	if g.m.Dealt {
		g.sortHand()
		g.playerShown = table.HandSize
		g.message = "HAND TO FINISH - " + chooseLow
	}
}

const chooseLow = "CHOOSE TWO CARDS FOR THE LOW HAND, OR PRESS HOUSE WAY"

// sortHand lays the player's cards out highest first, the joker as an ace,
// and clears the low hand.
func (g *Game) sortHand() {
	for i := range g.order {
		g.order[i] = i
	}
	rank := func(c cards.Card) int {
		if c == table.Joker {
			return 100 // ahead of the aces
		}
		return int(c.Rank())*4 + int(c.Suit())
	}
	slices.SortFunc(g.order[:], func(a, b int) int { return rank(g.m.Hand[b]) - rank(g.m.Hand[a]) })
	g.low = g.low[:0]
	g.mark = 0
}

// The bet, and the buttons under the table.
var betBox = ui.Button{X: 215, Y: 340, W: 170, H: 74}

func chipButton(i int) ui.Button {
	return ui.Button{X: 30 + float64(i)*76, Y: rowAY, W: 70, H: buttonH}
}

var (
	clearButton = ui.Button{X: 420, Y: rowAY, W: 80, H: buttonH}
	rebetButton = ui.Button{X: 506, Y: rowAY, W: 84, H: buttonH}
	dealButton  = ui.Button{X: 600, Y: rowAY, W: 170, H: rowBY + buttonH - rowAY} // as tall as two rows
	houseButton = ui.Button{X: 30, Y: rowBY, W: 140, H: buttonH}
	playButton  = ui.Button{X: 176, Y: rowBY, W: 146, H: buttonH}
	coinButton  = ui.Button{X: 334, Y: rowBY, W: 170, H: buttonH}
	resetButton = ui.Button{X: 510, Y: rowBY, W: 80, H: buttonH}
)

// The keys that choose each chip, and while a hand is set each card, along
// the top of the keyboard and on the number pad.
var numberKeys = [][2]ebiten.Key{
	{ebiten.KeyDigit1, ebiten.KeyNumpad1},
	{ebiten.KeyDigit2, ebiten.KeyNumpad2},
	{ebiten.KeyDigit3, ebiten.KeyNumpad3},
	{ebiten.KeyDigit4, ebiten.KeyNumpad4},
	{ebiten.KeyDigit5, ebiten.KeyNumpad5},
	{ebiten.KeyDigit6, ebiten.KeyNumpad6},
	{ebiten.KeyDigit7, ebiten.KeyNumpad7},
}

// rowX returns where the ith of seven cards lies in a row with its middle
// at tableX.
func rowX(i int) float64 {
	return tableX - (cardW+6*cardStep)/2 + float64(i)*cardStep
}

// setX returns where the ith card of a hand set as five and two lies: the
// high hand, then a space, then the low.
func setX(i int) float64 {
	x := tableX - (cardW+6*cardStep+setGap)/2 + float64(i)*cardStep
	if i >= table.HighSize {
		x += setGap
	}
	return x
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

	// With a hand dealt there is only to set it and play it.
	if g.m.Dealt {
		for i := range table.HandSize {
			card := ui.Button{X: rowX(i), Y: playerY, W: cardW, H: cardH + lowDrop}
			if hit(card) || ui.Pressed(numberKeys[i][:]...) {
				g.mark = i
				g.toggle(i)
			}
		}
		switch {
		case ui.Pressed(ebiten.KeyArrowLeft):
			g.mark = max(g.mark-1, 0)
		case ui.Pressed(ebiten.KeyArrowRight):
			g.mark = min(g.mark+1, table.HandSize-1)
		case ui.Pressed(ebiten.KeyEnter, ebiten.KeyNumpadEnter, ebiten.KeyArrowDown, ebiten.KeyArrowUp):
			g.toggle(g.mark)
		case hit(houseButton), ui.Pressed(ebiten.KeyH):
			g.houseWay()
		case hit(playButton), ui.Pressed(ebiten.KeyP):
			g.play()
		}
		return
	}

	// A click on the bet puts a chip down, and a click of the other button
	// takes one off.
	if betBox.Hit(x, y) {
		switch {
		case click:
			g.place()
		case inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight):
			g.remove()
		}
	}
	for i, n := range table.ChipChoices {
		if hit(chipButton(i)) || ui.Pressed(numberKeys[i][:]...) {
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
	g.playerShown, g.dealerShown = 0, 0
}

func (g *Game) setChip(n int) {
	g.m.Chip = n
	g.snd.Play(g.snd.Click)
	g.save()
}

const outOfCredits = "OUT OF CREDITS - INSERT A COIN OR RESET"

// place puts a chip on the bet. Like the rest of the betting, it does
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
		case g.m.Bet+g.m.Chip > table.MaxBet:
			g.message = fmt.Sprintf("NO MORE THAN %s ON A HAND", bank.Money(table.MaxBet))
		default:
			g.message = "NOT ENOUGH CREDITS FOR THAT BET"
		}
		return
	}
	g.message = "PLACE YOUR BET"
	g.snd.Play(g.chipSound)
	g.save()
}

// remove takes a chip off the bet.
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

// rebet puts the bet of the last hand back on the table.
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

// reset puts the bank's credits back to the starting amount. The bet and a
// hand that is waiting are left as they are.
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
	if g.m.Bet == 0 {
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

// deal has the machine deal the player's seven, then shows them one by one.
// With too few credits nothing is dealt: the bank is every game's, so it is
// for the player to insert a coin or reset it.
func (g *Game) deal() {
	if g.m.Dealt {
		return
	}
	if g.m.Bet == 0 {
		g.m.Rebet()
	}
	if !g.m.Deal() {
		g.clearWin()
		switch {
		case g.bank.Credits == 0:
			g.message = outOfCredits
		case g.m.Bet == 0 && g.m.Last == 0:
			g.message = "PLACE A BET BEFORE YOU DEAL"
		default:
			g.message = "NOT ENOUGH CREDITS - INSERT A COIN OR BET LESS"
		}
		return
	}
	g.save()
	g.clearWin()
	g.sortHand()
	g.showing = true
	g.showAt = g.ticks + dealTicks
	g.credits = g.bank.Credits
	g.message = ""
}

// toggle moves the ith card of the row into the low hand, or back out of
// it. The low hand holds two: a third card chosen takes the place of the
// one chosen first.
func (g *Game) toggle(i int) {
	place := g.order[i]
	if at := slices.Index(g.low, place); at >= 0 {
		g.low = slices.Delete(g.low, at, at+1)
	} else {
		if len(g.low) == table.LowSize {
			g.low = g.low[1:]
		}
		g.low = append(g.low, place)
	}
	g.snd.Play(g.snd.Card)
	g.sayHand()
}

// houseWay sets the hand the house way.
func (g *Game) houseWay() {
	low := g.m.Ranker.HouseWay(g.m.Hand)
	g.low = append(g.low[:0], low[0], low[1])
	g.snd.Play(g.snd.Click)
	g.sayHand()
	if g.message != "" {
		g.message = "SET THE HOUSE WAY - " + g.message
	}
}

// setting returns the hand as it is set, and whether it can be played: two
// cards low and no foul.
func (g *Game) setting() (table.Setting, bool) {
	if len(g.low) != table.LowSize {
		return table.Setting{}, false
	}
	return g.m.Ranker.Set(g.m.Hand, [table.LowSize]int{g.low[0], g.low[1]})
}

// sayHand says how the hand stands as it is set.
func (g *Game) sayHand() {
	s, ok := g.setting()
	switch {
	case len(g.low) < table.LowSize:
		g.message = chooseLow
	case !ok:
		g.message = "THE HIGH HAND MUST BEAT THE LOW HAND - CHOOSE AGAIN"
	default:
		g.message = fmt.Sprintf("HIGH %s, LOW %s - PRESS PLAY", s.High, s.LowS)
	}
}

// play plays the hand as it is set, and turns the dealer's cards over one
// by one.
func (g *Game) play() {
	s, ok := g.setting()
	if !ok {
		g.sayHand()
		return
	}
	r, ok := g.m.Play(s.Low)
	if !ok {
		return
	}
	g.save()
	g.result = r
	g.showing = true
	g.showAt = g.ticks + dealTicks
	g.credits = g.bank.Credits - r.Paid
	g.message = ""
	g.snd.Play(g.chipSound)
}

// show carries on by a tick the dealing of the player's cards, or the
// turning over of the dealer's, and says how things stand once the last is
// shown.
func (g *Game) show(skip bool) {
	shown := &g.dealerShown
	if g.m.Dealt {
		shown = &g.playerShown
	}
	if skip {
		*shown = table.HandSize
		g.snd.Play(g.snd.Card)
	} else {
		if g.ticks < g.showAt {
			return
		}
		g.showAt = g.ticks + dealTicks
		if *shown < table.HandSize {
			*shown++
			g.snd.Play(g.snd.Card)
			return
		}
	}

	g.showing = false
	g.credits = g.bank.Credits
	if g.m.Dealt {
		g.message = chooseLow
		return
	}
	g.land()
}

// land shows how the hand went, now that the dealer's cards are up.
func (g *Game) land() {
	r := &g.result
	g.over = true
	g.win = r.Paid
	g.credits = g.bank.Credits
	each := func(won bool) string {
		if won {
			return "WINS"
		}
		return "LOSES"
	}
	hands := fmt.Sprintf("YOUR HIGH HAND %s, YOUR LOW HAND %s", each(r.HighWon), each(r.LowWon))
	switch r.Outcome {
	case table.Won:
		g.message = fmt.Sprintf("BOTH YOUR HANDS WIN - YOU WIN %s, AFTER %s COMMISSION", bank.Money(r.Paid), bank.Money(r.Commission))
		g.snd.Play(g.snd.Win)
	case table.Pushed:
		g.message = hands + " - A PUSH"
	default:
		g.message = "THE DEALER WINS BOTH HANDS"
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(felt)
	ui.DrawBorder(screen, redEdge)

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

// drawCard draws a card face up, the joker as itself.
func drawCard(screen *ebiten.Image, c cards.Card, x, y, scale float64) {
	if c == table.Joker {
		ui.DrawJoker(screen, x, y, scale)
		return
	}
	ui.DrawCard(screen, c, x, y, scale)
}

func (g *Game) drawMenu(screen *ebiten.Image) {
	ui.DrawText(screen, "PAI GOW POKER", ui.ScreenW/2, 70, 48, ui.Yellow, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "SEVEN CARDS, TWO HANDS", ui.ScreenW/2, 128, 20, ui.White, ui.AlignCenter, ui.AlignStart)

	g.menu.Draw(screen)
	ui.DrawText(screen, "ARROWS move    ENTER or NUMBER choose    ESC main menu", ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "MAKE YOUR CHOICE", ui.ScreenW/2, 350, 24, ui.LightCyan, ui.AlignCenter, ui.AlignStart)

	// Five aces, the best hand there is, for luck.
	const scale = 0.55
	for i := range 5 {
		x := ui.ScreenW/2 + (float64(i)-2.5)*(ui.CardW*scale+10) + 5
		c := table.Joker
		if i < 4 {
			c = cards.New(cards.Ace, cards.Suit(3-i))
		}
		drawCard(screen, c, x, 420, scale)
	}
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	ui.DrawText(screen, "HOW TO PLAY", ui.ScreenW/2, 22, 30, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	lines := []string{
		"Put chips on the BET and deal. You and the dealer get seven cards each,",
		"from 52 and a joker. Set yours as a HIGH hand of five and a LOW hand of",
		"two: the high hand must beat the low. The low hand counts only pairs.",
		"Win both hands and you are paid 1 to 1, less 5% commission. Win one and",
		"lose one and the bet comes back. Lose both and it is lost. A hand that",
		"ties goes to the dealer. The joker is an ace, or any card that makes a",
		"straight or a flush. Five aces is the best hand of all.",
		"The dealer sets the house way: no pair, the next two highest go low; one",
		"pair stays high; two pairs are split unless they are small and an ace",
		"can go low; three pairs, the highest goes low; a full house is split.",
		"HOUSE WAY sets your hand the same way. You can change it before you play.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2-285, 60+float64(i)*18, 15, ui.White, ui.AlignStart, ui.AlignStart)
	}

	keys := [][2]string{
		{"SPACE", "shuffle and deal"},
		{"ENTER / BACKSPACE", "put a chip on the bet, or take one off"},
		{"1 to 5", "choose a chip"},
		{"X, B", "clear the table; make the last bet again"},
		{"1 to 7", "with a hand dealt: move that card to the low hand, or back"},
		{"LEFT / RIGHT, ENTER", "move the marker; move that card"},
		{"H", "set the hand the house way"},
		{"P", "play the hand"},
		{"C, R", "insert a coin; reset your credits"},
		{"S", "sound on or off"},
		{"ESC", "back to the menu"},
	}
	for i, k := range keys {
		y := 270 + float64(i)*18
		ui.DrawText(screen, k[0], ui.ScreenW/2-70, y, 14, ui.LightRed, ui.AlignEnd, ui.AlignStart)
		ui.DrawText(screen, k[1], ui.ScreenW/2-45, y, 14, ui.White, ui.AlignStart, ui.AlignStart)
	}
	ui.DrawText(screen, "With the mouse, click a card to move it, and click the bet to put a chip down.", ui.ScreenW/2, 486, 13, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 546, 20, ui.LightCyan, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawAbout(screen *ebiten.Image) {
	ui.DrawText(screen, "ABOUT THIS GAME", ui.ScreenW/2, 50, 36, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	won, pushed, lost := table.Shares()
	lines := []string{
		"Pai Gow Poker is written in Go with Ebitengine.",
		"",
		"Each hand is dealt from one deck and a joker, freshly shuffled.",
		"",
		"Set the house way, a hand pays back about",
		fmt.Sprintf("%.2f%% of what is bet, to within %.2f%%.", table.Return()*100, table.Margin()*100),
		fmt.Sprintf("%.1f%% of hands win, %.1f%% push and %.1f%% lose.", won*100, pushed*100, lost*100),
		"This is the one figure in these games that is estimated,",
		"by playing hands out: it cannot be counted exactly in time.",
		"",
		"It is played with pretend credits only.",
		"No real money goes in or comes out.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2, 110+float64(i)*28, 18, ui.White, ui.AlignCenter, ui.AlignStart)
	}
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 500, 20, ui.LightCyan, ui.AlignCenter, ui.AlignStart)
}

// inOrder puts cards in the order they are read in: those of a rank there
// are most of first, so a pair or three lie together, and the higher first
// among those as many. The joker goes with the aces.
func inOrder(cs []cards.Card) {
	rank := func(c cards.Card) int {
		if c == table.Joker {
			return int(cards.Ace)
		}
		return int(c.Rank())
	}
	count := make(map[int]int)
	for _, c := range cs {
		count[rank(c)]++
	}
	slices.SortStableFunc(cs, func(a, b cards.Card) int {
		if d := count[rank(b)] - count[rank(a)]; d != 0 {
			return d
		}
		return rank(b) - rank(a)
	})
}

// drawSet draws a hand set as five and two, with its labels over each part
// lit if that part won, and as many face up as are shown.
func drawSet(screen *ebiten.Image, hand [table.HandSize]cards.Card, s table.Setting, shown int, y float64, who string, highLit, lowLit, labels bool) {
	high, low := table.Split(hand, s.Low)
	inOrder(high[:])
	inOrder(low[:])
	set := append(high[:], low[:]...)
	for i, c := range set {
		if i < shown {
			drawCard(screen, c, setX(i), y, cardScale)
		} else {
			ui.DrawCardBack(screen, setX(i), y, cardScale)
		}
	}
	if !labels {
		ui.DrawText(screen, who, tableX, y-16, 13, ui.White, ui.AlignCenter, ui.AlignStart)
		return
	}
	ink := func(lit bool) color.Color {
		if lit {
			return ui.Yellow
		}
		return ui.White
	}
	highMid := (setX(0) + setX(table.HighSize-1) + cardW) / 2
	lowMid := (setX(table.HighSize) + setX(table.HandSize-1) + cardW) / 2
	ui.DrawText(screen, who+" HIGH: "+s.High.String(), highMid, y-16, 13, ink(highLit), ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "LOW: "+s.LowS.String(), lowMid, y-16, 13, ink(lowLit), ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawPlay(screen *ebiten.Image) {
	done := g.over && !g.showing
	r := &g.result

	switch {
	case g.shuffling:
		// While the deck is shuffled the table is clear.
		shuffleLength.Draw(screen, g.ticks-g.shuffleStart, tableX, (dealerY+playerY+cardH)/2, cardScale)

	case g.m.Dealt:
		// The dealer's seven lie face down while the hand is set, and the
		// player's in a row, those chosen for the low hand dropped.
		for i := range table.HandSize {
			ui.DrawCardBack(screen, rowX(i), dealerY, cardScale)
		}
		ui.DrawText(screen, "DEALER", tableX, dealerY-16, 13, ui.White, ui.AlignCenter, ui.AlignStart)
		label := "YOUR CARDS"
		if s, ok := g.setting(); ok {
			label = fmt.Sprintf("HIGH: %s    LOW: %s", s.High, s.LowS)
		}
		ui.DrawText(screen, label, tableX, playerY-16, 13, ui.White, ui.AlignCenter, ui.AlignStart)
		setting := !g.showing
		for i, place := range g.order {
			x, y := rowX(i), float64(playerY)
			if i >= g.playerShown {
				ui.DrawCardBack(screen, x, y, cardScale)
				continue
			}
			isLow := slices.Contains(g.low, place)
			if isLow {
				y += lowDrop
			}
			drawCard(screen, g.m.Hand[place], x, y, cardScale)
			if isLow {
				ui.FrameRect(screen, x-3, y-3, cardW+6, cardH+6, 3, ui.LightCyan)
			}
			if setting && i == g.mark {
				ui.FrameRect(screen, x-6, y-6, cardW+12, cardH+12, 2, ui.White)
			}
			if setting {
				ui.DrawText(screen, fmt.Sprint(i+1), x+cardW/2, playerY+cardH+lowDrop+4, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
			}
		}

	case g.result.Bet > 0 && (g.showing || g.over):
		// The hand that was played: both set as five and two.
		drawSet(screen, r.Dealer, r.House, g.dealerShown, dealerY, "DEALER", done && !r.HighWon, done && !r.LowWon, done)
		drawSet(screen, r.Hand, r.Player, table.HandSize, playerY, "YOUR", done && r.HighWon, done && r.LowWon, true)

	default:
		for i := range table.HandSize {
			ui.DrawCardBack(screen, rowX(i), dealerY, cardScale)
			ui.DrawCardBack(screen, rowX(i), playerY, cardScale)
		}
		ui.DrawText(screen, "DEALER", tableX, dealerY-16, 13, ui.White, ui.AlignCenter, ui.AlignStart)
		ui.DrawText(screen, "YOUR CARDS", tableX, playerY-16, 13, ui.White, ui.AlignCenter, ui.AlignStart)
	}

	ui.DrawText(screen, g.message, tableX, messageY, 13, ui.Yellow, ui.AlignCenter, ui.AlignStart)
	g.drawBet(screen)
	g.drawPanel(screen)
	g.drawButtons(screen)
}

// drawBet draws the bet and the chips on it.
func (g *Game) drawBet(screen *ebiten.Image) {
	bet, lit := g.m.Bet, false
	switch {
	case g.m.Dealt:
		bet = g.m.Stake
	case g.showing:
		bet = g.result.Bet
	case g.over && g.m.Bet == 0 && g.result.Paid > 0:
		bet, lit = g.result.Bet, g.result.Outcome == table.Won && g.ticks/15%2 == 0
	}
	b := betBox
	ui.FrameRect(screen, b.X, b.Y, b.W, b.H, 1, ui.LightGray)
	cx := b.X + b.W/2
	ui.DrawText(screen, "BET", cx, b.Y+7, 15, ui.White, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "BOTH HANDS MUST WIN", cx, b.Y+27, 10, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	if lit {
		ui.FrameRect(screen, b.X+1, b.Y+1, b.W-2, b.H-2, 4, ui.Yellow)
	}
	if !g.m.Dealt && !g.showing && !g.shuffling {
		ui.FrameRect(screen, b.X+1, b.Y+1, b.W-2, b.H-2, 2, ui.White)
	}
	if bet > 0 {
		ui.DrawChip(screen, cx, b.Y+b.H-17, bet, table.ChipChoices, lit)
	}
}

// drawPanel sets out how the game pays down the right of the table, with
// the figures under it.
func (g *Game) drawPanel(screen *ebiten.Image) {
	rows := [][2]string{
		{"BOTH HANDS WIN", "1 TO 1"},
		{"", "LESS 5%"},
		{"ONE EACH", "PUSH"},
		{"BOTH LOSE", "LOSE"},
	}
	ui.DrawText(screen, "HOW IT PAYS", panelX, 24, 11, ui.LightGray, ui.AlignStart, ui.AlignStart)
	for i, row := range rows {
		y := 40 + float64(i)*16
		ui.DrawText(screen, row[0], panelX, y, 12, ui.White, ui.AlignStart, ui.AlignStart)
		ui.DrawText(screen, row[1], panelX+panelW, y, 12, ui.White, ui.AlignEnd, ui.AlignStart)
	}
	notes := []string{
		"A TIED HAND GOES",
		"TO THE DEALER",
		"",
		"THE JOKER IS AN ACE, OR",
		"MAKES A STRAIGHT OR FLUSH",
		"",
		"FIVE ACES BEAT",
		"A ROYAL FLUSH",
		"",
		"THE DEALER SETS",
		"THE HOUSE WAY",
	}
	for i, n := range notes {
		ui.DrawText(screen, n, panelX, 118+float64(i)*13, 10, ui.LightGray, ui.AlignStart, ui.AlignStart)
	}

	// What is bet: on the table before the deal, and in play after it.
	bet := g.m.Bet
	switch {
	case g.m.Dealt:
		bet = g.m.Stake
	case g.showing:
		bet = g.result.Bet
	}
	stat := func(i int, label, v string, c color.Color) {
		sy := 274 + float64(i)*50
		ui.DrawText(screen, label, panelX+panelW/2, sy, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
		ui.DrawText(screen, v, panelX+panelW/2, sy+13, 22, c, ui.AlignCenter, ui.AlignStart)
	}
	stat(0, "CREDITS", bank.Dollars(g.credits), ui.LightGreen)
	stat(1, "TOTAL BET", bank.Money(bet), ui.Yellow)
	stat(2, "WIN", bank.Money(g.win), ui.White)
}

func (g *Game) drawButtons(screen *ebiten.Image) {
	// The buttons that bet are locked while a hand is played, and those
	// that set it are locked until one is dealt.
	busy := g.showing || g.shuffling
	betting := !g.m.Dealt && !busy
	setting := g.m.Dealt && !busy
	_, playable := g.setting()
	for i, n := range table.ChipChoices {
		ui.DrawButton(screen, chipButton(i), n == g.m.Chip, !betting, 11, bank.Money(n), "CHIP")
	}
	ui.DrawButton(screen, clearButton, false, !betting, 13, "CLEAR")
	ui.DrawButton(screen, rebetButton, false, !betting || g.m.Last == 0, 13, "REBET")
	ui.DrawButton(screen, houseButton, false, !setting, 13, "HOUSE WAY")
	ui.DrawButton(screen, playButton, setting && playable, !setting || !playable, 14, "PLAY")
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
	keys := fmt.Sprintf("SPACE deal   ENTER bet   BACKSPACE take off   1-5 chip   X clear   B rebet   C coin   R reset   S sound %s   ESC menu", sound)
	if g.m.Dealt {
		keys = fmt.Sprintf("1-7 or ENTER move a card    LEFT/RIGHT marker    H house way    P play    C coin    R reset    S sound %s    ESC menu", sound)
	}
	ui.DrawText(screen, keys, ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
}
