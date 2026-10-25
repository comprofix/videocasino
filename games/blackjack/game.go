// Package blackjack is Blackjack with the Perfect Pairs side bet, played
// for pretend credits from the bank. Its rules are in the table package.
package blackjack

import (
	"fmt"
	"image/color"
	"strings"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"videocasino/bank"
	"videocasino/cards"
	"videocasino/games/blackjack/table"
	"videocasino/palette"
	"videocasino/shuffle"
	"videocasino/ui"
)

const (
	// The cards are drawn smaller than in a hand of five, and overlapped,
	// as a hand can run to many of them.
	cardScale = 0.75
	cardW     = ui.CardW * cardScale
	cardH     = ui.CardH * cardScale
	cardStep  = 40 // how far each card sits from the one before: enough to show its rank

	// The dealer's hand stops short of the shoe, which sits at the end of
	// the table.
	dealerW = 230

	// The table takes up the left of the screen: the dealer's hand above
	// the player's. What the game pays is listed down the right.
	tableX  = 30
	tableW  = 540
	dealerY = 42
	playerY = 204
	panelX  = 590
	panelW  = 180

	messageY = 340
	statY    = 364

	// Three rows of buttons at the bottom: what to do with the hand, the
	// bet, and the Perfect Pairs bet.
	rowAY   = 428
	rowBY   = 466
	rowCY   = 504
	buttonH = 34
	buttonW = 104
	gap     = 6

	// revealTicks is the time between one of the dealer's cards being
	// dealt and the next.
	revealTicks = 18
)

// felt is the table the cards lie on.
var felt = palette.Pine

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

	// What the game pays back from a fresh shoe takes a moment to work out,
	// so that is done in the background while the program starts. Until it
	// is ready this is nil.
	odds atomic.Pointer[table.Odds]

	// The machine settles a hand all at once. The dealer's cards are then
	// shown one by one, and the result once the last is down.
	revealing   bool
	dealerShown int // how many of the dealer's cards are on show
	revealAt    int // the tick at which the next is shown

	hint    table.Action
	hinting bool // the hint is on show

	// The cut card is dealt onto the table when it comes out of the shoe,
	// and lies there until the shoe is shuffled. If it came out among the
	// dealer's cards it is shown in its turn: when cutAt of them are down.
	cutShown bool
	cutAt    int

	// The shuffle is shown before the first hand from a new shoe is dealt.
	shuffling    bool
	shuffleStart int // the tick it began at
	shuffleFrom  int // the cards that were left in the shoe

	// The sounds that are this game's own.
	cutSound, riffleSound []byte

	credits int // credits on display; a win is added once the dealer has played
	win     int
	message string
	confirm ui.Confirm // asks before the credits are reset

	saveFailed bool
}

// New returns the game, playing from the given bank and picking up whatever
// it last kept there.
func New(b *bank.Bank, snd *ui.Sounds) *Game {
	g := &Game{snd: snd, bank: b}
	g.m = table.NewRandom(b)
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
	g.message = "CHOOSE YOUR BET AND PRESS DEAL"

	g.cutSound = append(ui.Tone(880, 60, 0.2, true), ui.Tone(660, 110, 0.2, true)...)
	g.riffleSound = shuffle.Sound()

	var s table.Saved
	if b.Load(saveName, &s) {
		g.m.Restore(s)
		g.resume()
	}
	// A cut card that came out before the game was closed is on the table.
	g.cutShown = g.m.CutOut()
	g.credits = b.Credits

	go func() {
		g.odds.Store(table.NewOdds())
	}()
	return g
}

// resume puts back on screen a hand that was still to be played when the
// game was last closed.
func (g *Game) resume() {
	if g.m.Dealt {
		g.dealerShown = len(g.m.Dealer)
		g.win = g.m.SideWin
		g.message = "HAND TO FINISH - " + g.prompt()
	}
}

// The buttons for the hand, in the order of the keys that work them.
var actions = [...]table.Action{table.ActHit, table.ActStand, table.ActDouble, table.ActSplit}

func rowButton(i int, y float64) ui.Button {
	return ui.Button{X: tableX + float64(i)*(buttonW+gap), Y: y, W: buttonW, H: buttonH}
}

// actionButton is the button for one of the actions, and hintButton the one
// that ends their row.
func actionButton(i int) ui.Button { return rowButton(i, rowAY) }
func betButton(i int) ui.Button    { return rowButton(i, rowBY) }
func sideButton(i int) ui.Button   { return rowButton(i, rowCY) }

var (
	hintButton  = rowButton(len(actions), rowAY)
	dealButton  = ui.Button{X: 580, Y: rowAY, W: 190, H: rowBY + buttonH - rowAY} // as tall as two rows
	coinButton  = ui.Button{X: 580, Y: rowCY, W: 120, H: buttonH}
	resetButton = ui.Button{X: 706, Y: rowCY, W: 64, H: buttonH}
)

// The keys for each action, along the top of the keyboard and on the
// number pad.
var actionKeys = [len(actions)][2]ebiten.Key{
	{ebiten.KeyDigit1, ebiten.KeyNumpad1},
	{ebiten.KeyDigit2, ebiten.KeyNumpad2},
	{ebiten.KeyDigit3, ebiten.KeyNumpad3},
	{ebiten.KeyDigit4, ebiten.KeyNumpad4},
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
	if g.revealing {
		g.reveal()
		return
	}
	if g.shuffling {
		// The deal key or a click cuts the shuffle short.
		skip := ui.Pressed(ebiten.KeySpace, ebiten.KeyEnter, ebiten.KeyNumpadEnter) ||
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
		g.shuffleStep(skip)
		return
	}
	// The bank may have changed while the game was not being played.
	g.credits = g.bank.Credits
	if ui.Pressed(ebiten.KeyEscape) {
		g.scene = sceneMenu
		return
	}

	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	x, y := ebiten.CursorPosition()
	hit := func(b ui.Button) bool { return click && b.Hit(x, y) }

	if ui.Pressed(ebiten.KeyC) || hit(coinButton) {
		g.insertCoin()
	}
	if ui.Pressed(ebiten.KeyR) || hit(resetButton) {
		g.askReset()
	}

	// While a hand is being played it is all that can be changed: the bets
	// stay as they were dealt.
	if g.m.Dealt {
		for i, a := range actions {
			if ui.Pressed(actionKeys[i][:]...) || hit(actionButton(i)) {
				g.act(a)
				return
			}
		}
		if ui.Pressed(ebiten.KeyH) || hit(hintButton) {
			g.showHint()
		}
		return
	}

	for i, n := range table.BetChoices {
		if hit(betButton(i)) {
			g.setBet(n)
		}
	}
	for i, n := range table.SideChoices {
		if hit(sideButton(i)) {
			g.setSideBet(n)
		}
	}
	switch {
	case ui.Pressed(ebiten.KeyArrowUp):
		g.setBet(ui.Step(table.BetChoices, g.m.Bet, 1))
	case ui.Pressed(ebiten.KeyArrowDown):
		g.setBet(ui.Step(table.BetChoices, g.m.Bet, -1))
	case ui.Pressed(ebiten.KeyArrowRight):
		g.setSideBet(ui.Step(table.SideChoices, g.m.SideBet, 1))
	case ui.Pressed(ebiten.KeyArrowLeft):
		g.setSideBet(ui.Step(table.SideChoices, g.m.SideBet, -1))
	case ui.Pressed(ebiten.KeySpace, ebiten.KeyEnter, ebiten.KeyNumpadEnter) || hit(dealButton):
		g.pressDeal()
	}
}

// pressDeal deals the next hand. If the cut card has come out the shoe is
// shuffled first, and that is shown before the cards are dealt. With too
// few credits for the hand there is no shuffle to show: nothing is dealt.
func (g *Game) pressDeal() {
	if !g.m.CutOut() || g.bank.Credits < g.m.Bet+g.m.SideBet {
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

// showCut deals the cut card onto the table.
func (g *Game) showCut() {
	g.cutShown = true
	g.snd.Play(g.cutSound)
}

// clearWin takes the last hand's win off the display. The cards stay.
func (g *Game) clearWin() {
	g.win = 0
	g.message = ""
}

func (g *Game) setBet(n int) {
	g.m.Bet = n
	g.clearWin()
	g.snd.Play(g.snd.Click)
	g.save()
}

func (g *Game) setSideBet(n int) {
	g.m.SideBet = n
	g.clearWin()
	g.snd.Play(g.snd.Click)
	g.save()
}

const outOfCredits = "OUT OF CREDITS - INSERT A COIN OR RESET"

// askReset asks the player to make sure before the credits are reset.
func (g *Game) askReset() {
	g.confirm.Ask("RESET CREDITS TO "+bank.Money(bank.StartCredits)+"?", "YOU HAVE "+bank.Dollars(g.bank.Credits))
}

// reset puts the bank's credits back to the starting amount. The bets are
// left as they are, and so is a hand that is being played.
func (g *Game) reset() {
	g.bank.Reset()
	g.credits = g.bank.Credits
	g.message = "CREDITS RESET"
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

// deal takes the bets and deals a hand. Perfect Pairs is settled there and
// then. With too few credits nothing is dealt: the bank is every game's,
// so it is for the player to insert a coin or reset it.
func (g *Game) deal() {
	if !g.m.Deal() {
		g.clearWin()
		g.message = "NOT ENOUGH CREDITS - INSERT A COIN OR BET LESS"
		if g.bank.Credits == 0 {
			g.message = outOfCredits
		}
		return
	}
	g.hinting = false
	g.win = g.m.SideWin
	g.dealerShown = 1
	if !g.m.CutOut() {
		g.cutShown = false // shuffled back into the shoe
	}
	g.snd.Play(g.snd.Card)
	if g.m.SideWin > 0 {
		g.snd.Play(g.snd.Win)
	}
	g.save()
	g.after()
}

// pairNote says how Perfect Pairs went for the hand on the table, if
// anything was bet on it.
func (g *Game) pairNote() string {
	switch {
	case g.m.Side == 0:
		return ""
	case g.m.Pair == table.NoPair:
		return "NO PAIR - "
	}
	return fmt.Sprintf("%s PAYS %s! - ", strings.ToUpper(g.m.Pair.String()), bank.Money(g.m.SideWin))
}

// prompt says what can be done with the hand being played.
func (g *Game) prompt() string {
	can := "HIT OR STAND"
	switch double, split := g.m.CanDouble(), g.m.CanSplit(); {
	case double && split:
		can = "HIT, STAND, DOUBLE OR SPLIT"
	case double:
		can = "HIT, STAND OR DOUBLE"
	}
	if len(g.m.Hands) == 2 {
		return fmt.Sprintf("HAND %d: %s", g.m.Active+1, can)
	}
	return can
}

// act does something with the hand being played.
func (g *Game) act(a table.Action) {
	ok := false
	switch a {
	case table.ActHit:
		ok = g.m.Hit()
	case table.ActStand:
		ok = g.m.Stand()
	case table.ActDouble:
		ok = g.m.Double()
	case table.ActSplit:
		ok = g.m.Split()
	}
	if !ok {
		// Only a double or a split can be refused while a hand is in play.
		g.message = fmt.Sprintf("CANNOT %s THIS HAND", strings.ToUpper(a.String()))
		return
	}
	g.hinting = false
	if a == table.ActStand {
		g.snd.Play(g.snd.Click)
	} else {
		g.snd.Play(g.snd.Card)
	}
	// The machine settles the hand straight away, so it is safe to save
	// even though the dealer's cards have yet to be shown.
	g.save()
	g.after()
}

// after catches up with the machine once a card has been dealt: there is
// either more of the hand to play, or the dealer's cards to show.
func (g *Game) after() {
	cutDue := g.m.CutOut() && !g.cutShown
	if g.m.Dealt {
		// The cut card came out with the card just dealt.
		if cutDue {
			g.showCut()
		}
		g.credits = g.bank.Credits
		g.message = g.pairNote() + g.prompt()
		return
	}
	g.credits = g.bank.Credits - g.m.Last.Paid
	g.message = ""
	g.revealing = true
	g.revealAt = g.ticks + revealTicks
	if cutDue {
		// The cards dealt since the cut card came out are the last of the
		// dealer's, so it is shown before them: at once if it came out
		// before any that are still to be shown.
		after := -g.m.ToCut
		g.cutAt = max(len(g.m.Dealer)-after, g.dealerShown)
	}
}

// reveal carries on showing the dealer's cards, and when the last one is
// down says how the hand went.
func (g *Game) reveal() {
	if g.ticks < g.revealAt {
		return
	}
	// The cut card takes its turn among the dealer's cards.
	if g.m.CutOut() && !g.cutShown && g.dealerShown >= g.cutAt {
		g.showCut()
		g.revealAt = g.ticks + revealTicks
		return
	}
	if g.dealerShown < len(g.m.Dealer) {
		g.dealerShown++
		g.revealAt = g.ticks + revealTicks
		g.snd.Play(g.snd.Card)
		return
	}
	g.revealing = false
	g.credits = g.bank.Credits

	r := g.m.Last
	net := r.Paid - r.Staked
	g.win = g.m.SideWin + max(net, 0)
	g.message = g.result()
	switch {
	case r.Outcomes[0] == table.WonBlackjack:
		g.snd.Play(g.snd.BigWin)
	case net > 0:
		g.snd.Play(g.snd.Win)
	}
}

// result says how the hand last played went.
func (g *Game) result() string {
	r := g.m.Last
	net := r.Paid - r.Staked
	money := "NOTHING WON OR LOST"
	switch {
	case net > 0:
		money = fmt.Sprintf("YOU WIN %s!", bank.Money(net))
	case net < 0:
		money = fmt.Sprintf("YOU LOSE %s", bank.Money(-net))
	}

	switch {
	case r.Outcomes[0] == table.WonBlackjack:
		return "BLACKJACK! " + money
	case r.DealerBlackjack && r.Outcomes[0] == table.Pushed:
		return "BOTH HAVE BLACKJACK - PUSH"
	case r.DealerBlackjack:
		return "DEALER HAS BLACKJACK - " + money
	}

	if len(r.Outcomes) == 1 {
		switch {
		case r.Outcomes[0] == table.Won:
			return money
		case r.Outcomes[0] == table.Pushed:
			return "PUSH - YOUR BET IS RETURNED"
		case g.m.Hands[0].Bust():
			return "BUST - " + money
		}
		return "DEALER WINS - " + money
	}

	var parts []string
	for i, o := range r.Outcomes {
		how := "LOSES"
		switch o {
		case table.Won:
			how = "WINS"
		case table.Pushed:
			how = "PUSHES"
		}
		parts = append(parts, fmt.Sprintf("HAND %d %s", i+1, how))
	}
	return strings.Join(parts, ", ") + " - " + money
}

// showHint marks the best thing to do with the hand, without doing it. It
// is worked out from the cards left in the shoe.
func (g *Game) showHint() {
	a, ok := table.BestPlay(g.m)
	if !ok {
		return
	}
	g.hint, g.hinting = a.Best, true
	g.snd.Play(g.snd.Click)
	g.message = fmt.Sprintf("BEST PLAY: %s - WORTH %s ON AVERAGE",
		strings.ToUpper(a.Best.String()), bank.Worth(a.Value()*float64(g.m.Stake)))
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(felt)
	ui.DrawBorder(screen, palette.Rust)

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
	ui.DrawText(screen, "BLACKJACK", ui.ScreenW/2, 70, 48, palette.Gold, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "WITH PERFECT PAIRS", ui.ScreenW/2, 128, 20, palette.White, ui.AlignCenter, ui.AlignStart)

	g.menu.Draw(screen)
	ui.DrawText(screen, "ARROWS move    ENTER or NUMBER choose    ESC main menu", ui.ScreenW/2, 550, 11, palette.Fog, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "MAKE YOUR CHOICE", ui.ScreenW/2, 350, 24, palette.Seafoam, ui.AlignCenter, ui.AlignStart)

	// A blackjack, for luck.
	const scale = 0.62
	ui.DrawCard(screen, cards.New(cards.Ace, cards.Spades), ui.ScreenW/2-ui.CardW*scale-5, 420, scale)
	ui.DrawCard(screen, cards.New(cards.Jack, cards.Spades), ui.ScreenW/2+5, 420, scale)
}

func (g *Game) drawHelp(screen *ebiten.Image) {
	ui.DrawText(screen, "HOW TO PLAY", ui.ScreenW/2, 22, 30, palette.Gold, ui.AlignCenter, ui.AlignStart)

	pays := table.PairPays
	lines := []string{
		"Choose a bet, and a Perfect Pairs bet if you want one, then deal.",
		"Get closer to 21 than the dealer without going past it. Pictures count",
		fmt.Sprintf("10 and an ace 1 or 11. The dealer draws to %d and stands on any %d.",
			table.DealerStands, table.DealerStands),
		"A blackjack, an ace and a ten in your first two cards, pays 3 to 2.",
		fmt.Sprintf("DOUBLE doubles the bet for one more card, on a hard %d, %d or %d only.",
			table.DoubleFrom, table.DoubleFrom+1, table.DoubleTo),
		"SPLIT makes two hands of two cards that count the same, once. Split",
		"aces get one card each, and 21 after a split is not a blackjack.",
		"If the dealer makes a blackjack you lose only the bet you first made.",
		fmt.Sprintf("Perfect Pairs is paid on your first two cards: %d for the same card", pays[table.PerfectPair]),
		fmt.Sprintf("twice, %d for the same rank and colour, %d for the same rank. A king",
			pays[table.ColouredPair], pays[table.MixedPair]),
		"and a queen can be split, but they are not a pair.",
		fmt.Sprintf("The %d decks are dealt down to a cut card, then shuffled. HINT works", table.Decks),
		"from the cards left in the shoe: exactly for hit, stand and double, and",
		"for a split without the cards the other hand has still to draw.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2-275, 60+float64(i)*18, 15, palette.White, ui.AlignStart, ui.AlignStart)
	}

	keys := [][2]string{
		{"SPACE or ENTER", "deal"},
		{"1, 2, 3, 4", "hit, stand, double, split"},
		{"H", "hint: mark the best play"},
		{"UP / DOWN", "change the bet"},
		{"LEFT / RIGHT", "change the Perfect Pairs bet"},
		{"C", "insert a coin for more credits"},
		{"R", "reset your credits"},
		{"S", "sound on or off"},
		{"ESC", "back to the menu"},
	}
	for i, k := range keys {
		y := 322 + float64(i)*18
		ui.DrawText(screen, k[0], ui.ScreenW/2-70, y, 14, palette.Red, ui.AlignEnd, ui.AlignStart)
		ui.DrawText(screen, k[1], ui.ScreenW/2-45, y, 14, palette.White, ui.AlignStart, ui.AlignStart)
	}
	ui.DrawText(screen, "The buttons on screen can be clicked with the mouse.", ui.ScreenW/2, 494, 13, palette.Fog, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 546, 20, palette.Seafoam, ui.AlignCenter, ui.AlignStart)
}

func (g *Game) drawAbout(screen *ebiten.Image) {
	ui.DrawText(screen, "ABOUT THIS GAME", ui.ScreenW/2, 50, 36, palette.Gold, ui.AlignCenter, ui.AlignStart)

	returns := "Working out what it pays back..."
	if o := g.odds.Load(); o != nil {
		returns = fmt.Sprintf("without splitting, it pays back %.2f%% of what is bet.", o.Return()*100)
	}
	lines := []string{
		"Blackjack is written in Go with Ebitengine.",
		"",
		fmt.Sprintf("It is dealt from %d decks down to a cut card, then shuffled.", table.Decks),
		"",
		"From a freshly shuffled shoe, played perfectly",
		returns,
		fmt.Sprintf("Perfect Pairs then pays back %.2f%% of what is bet on it.", table.PairReturn()*100),
		"",
		"Both change from hand to hand as the shoe runs down.",
		"",
		"It is played with pretend credits only.",
		"No real money goes in or comes out.",
	}
	for i, l := range lines {
		ui.DrawText(screen, l, ui.ScreenW/2, 130+float64(i)*28, 18, palette.White, ui.AlignCenter, ui.AlignStart)
	}
	ui.DrawText(screen, "PRESS ENTER TO RETURN", ui.ScreenW/2, 500, 20, palette.Seafoam, ui.AlignCenter, ui.AlignStart)
}

// drawCards draws a hand overlapped and centred on cx, squeezed up if it
// would be wider than width. It is followed by the given number of cards
// lying face down.
func drawCards(screen *ebiten.Image, hand []cards.Card, backs int, cx, y, width float64) {
	n := len(hand) + backs
	step := float64(cardStep)
	if n > 1 {
		step = min(step, (width-cardW)/float64(n-1))
	}
	x := cx - (cardW+step*float64(n-1))/2
	for _, c := range hand {
		ui.DrawCard(screen, c, x, y, cardScale)
		x += step
	}
	for range backs {
		ui.DrawCardBack(screen, x, y, cardScale)
		x += step
	}
}

// totalText writes what a hand adds up to.
func totalText(hand []cards.Card) string {
	total, soft := table.Total(hand)
	switch {
	case table.IsBlackjack(hand):
		return "21"
	case total > 21:
		return fmt.Sprintf("%d BUST", total)
	case soft:
		return fmt.Sprintf("SOFT %d", total)
	}
	return fmt.Sprint(total)
}

func (g *Game) drawPlay(screen *ebiten.Image) {
	g.drawPanel(screen)
	const centre = tableX + tableW/2

	// While the shoe is being shuffled the table is clear.
	if g.shuffling {
		shuffle.Full.Draw(screen, g.ticks-g.shuffleStart, tableX+tableW/2, (dealerY+playerY+cardH)/2, cardScale)
	} else {
		g.drawHands(screen)
	}
	g.drawShoe(screen)

	ui.DrawText(screen, g.message, centre, messageY, 15, palette.Gold, ui.AlignCenter, ui.AlignStart)

	// The bets on show are the ones the hand was dealt for while it is
	// being played, and the ones chosen for the next otherwise.
	bet, side := g.m.Bet, g.m.SideBet
	if g.m.Dealt || g.revealing {
		bet, side = g.m.Stake, g.m.Side
	}
	drawStat(screen, "CREDITS", bank.Dollars(g.credits), 0, palette.Leaf)
	drawStat(screen, "BET", bank.Money(bet), 1, palette.White)
	drawStat(screen, "PERFECT PAIRS BET", bank.Money(side), 2, palette.White)
	drawStat(screen, "WIN", bank.Money(g.win), 3, palette.Gold)

	// The hand's buttons work only while there is a hand to play, and the
	// bets only between hands.
	playing := g.m.Dealt && !g.revealing && !g.shuffling
	for i, a := range actions {
		locked := !playing
		switch a {
		case table.ActDouble:
			locked = !playing || !g.m.CanDouble()
		case table.ActSplit:
			locked = !playing || !g.m.CanSplit()
		}
		lit := playing && g.hinting && g.hint == a
		ui.DrawButton(screen, actionButton(i), lit, locked, 13, fmt.Sprintf("%d %s", i+1, strings.ToUpper(a.String())))
	}
	ui.DrawButton(screen, hintButton, false, !playing, 13, "HINT")

	locked := g.m.Dealt || g.revealing || g.shuffling
	for i, n := range table.BetChoices {
		ui.DrawButton(screen, betButton(i), n == g.m.Bet, locked, 13, "BET "+bank.Money(n))
	}
	for i, n := range table.SideChoices {
		label := "PAIRS " + bank.Money(n)
		if n == 0 {
			label = "PAIRS OFF"
		}
		ui.DrawButton(screen, sideButton(i), n == g.m.SideBet, locked, 13, label)
	}

	b := dealButton
	fill, edge, ink := color.Color(palette.Forest), color.Color(palette.Leaf), color.Color(palette.White)
	if locked {
		fill, edge, ink = palette.Ink, palette.Dusk, palette.Dusk
	}
	ui.FillRect(screen, b.X, b.Y, b.W, b.H, fill)
	ui.FrameRect(screen, b.X, b.Y, b.W, b.H, 2, edge)
	ui.DrawText(screen, "DEAL", b.X+b.W/2, b.Y+b.H/2, 28, ink, ui.AlignCenter, ui.AlignCenter)

	ui.DrawCoinButton(screen, coinButton, 11, fmt.Sprintf("COIN +$%d", bank.CoinCredits/100))
	ui.DrawResetButton(screen, resetButton, 11)

	sound := "ON"
	if !g.snd.On {
		sound = "OFF"
	}
	hint := fmt.Sprintf("SPACE deal    1-4 play    H hint    UP/DOWN bet    LEFT/RIGHT pairs bet    C coin    R reset    S sound %s    ESC menu", sound)
	ui.DrawText(screen, hint, ui.ScreenW/2, 552, 11, palette.Fog, ui.AlignCenter, ui.AlignStart)
}

// drawPanel lists what the game pays down the right of the table.
func (g *Game) drawPanel(screen *ebiten.Image) {
	const top, height = dealerY - 17, playerY + cardH - dealerY + 17
	ui.FillRect(screen, panelX, top, panelW, height, palette.Ink)
	ui.FrameRect(screen, panelX, top, panelW, height, 2, palette.Gold)

	y := float64(top + 8)
	title := func(s string) {
		ui.DrawText(screen, s, panelX+panelW/2, y, 14, palette.Gold, ui.AlignCenter, ui.AlignStart)
		y += 22
	}
	row := func(name, pays string, lit bool) {
		ink := color.Color(palette.White)
		if lit {
			ui.FillRect(screen, panelX+3, y-2, panelW-6, 17, palette.White)
			ink = palette.Ink
		}
		ui.DrawText(screen, name, panelX+10, y, 12, ink, ui.AlignStart, ui.AlignStart)
		ui.DrawText(screen, pays, panelX+panelW-10, y, 12, ink, ui.AlignEnd, ui.AlignStart)
		y += 18
	}

	// The row for what the hand on the table made is lit.
	settled := !g.m.Dealt && !g.revealing && len(g.m.Last.Outcomes) > 0
	made := func(o table.Outcome) bool { return settled && g.m.Last.Outcomes[0] == o }

	title("BLACKJACK")
	row("BLACKJACK", "3 TO 2", made(table.WonBlackjack))
	row("WIN", "1 TO 1", false)
	row("PUSH", "BET BACK", false)
	y += 8
	ui.DrawText(screen, fmt.Sprintf("DEALER STANDS ON %d", table.DealerStands), panelX+10, y, 11, palette.Fog, ui.AlignStart, ui.AlignStart)
	y += 15
	ui.DrawText(screen, fmt.Sprintf("DOUBLE ON HARD %d TO %d", table.DoubleFrom, table.DoubleTo), panelX+10, y, 11, palette.Fog, ui.AlignStart, ui.AlignStart)
	y += 15
	ui.DrawText(screen, fmt.Sprintf("%d DECKS, SHUFFLED", table.Decks), panelX+10, y, 11, palette.Fog, ui.AlignStart, ui.AlignStart)
	y += 15
	ui.DrawText(screen, "AT THE CUT CARD", panelX+10, y, 11, palette.Fog, ui.AlignStart, ui.AlignStart)
	y += 24

	// Perfect Pairs pays for the hand on the table from the deal onwards.
	paired := func(k table.PairKind) bool {
		return len(g.m.Hands) > 0 && g.m.Side > 0 && g.m.Pair == k
	}
	title("PERFECT PAIRS")
	for k := table.PerfectPair; k >= table.MixedPair; k-- {
		name := strings.ToUpper(strings.TrimSuffix(k.String(), " Pair"))
		row(name, fmt.Sprintf("%d TO 1", table.PairPays[k]), paired(k))
	}
	y += 8
	for _, l := range []string{"PAID ON YOUR FIRST", "TWO CARDS, AT THE DEAL"} {
		ui.DrawText(screen, l, panelX+10, y, 11, palette.Fog, ui.AlignStart, ui.AlignStart)
		y += 15
	}
}

// drawStat draws a labelled figure in one of four columns under the table.
func drawStat(screen *ebiten.Image, label, value string, column int, c color.Color) {
	x := tableX + (float64(column)+0.5)*tableW/4
	ui.DrawText(screen, label, x, statY, 11, palette.Fog, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, value, x, statY+13, 24, c, ui.AlignCenter, ui.AlignStart)
}

// drawShoe draws the shoe at the end of the table: a box of cards that
// empties from the front as they are dealt, with the cut card standing in
// it where it was put. It shows how far through the shoe is, not a count.
func (g *Game) drawShoe(screen *ebiten.Image) {
	const w, h = 150, 26
	box := ui.Button{X: tableX + tableW - w, Y: dealerY + 6, W: w, H: h}

	// During a shuffle the shoe fills up again.
	full := float64(g.m.CardsLeft()) / table.ShoeSize
	if g.shuffling {
		full += (1 - full) * shuffle.Full.Done(g.ticks-g.shuffleStart)
	}

	// The cut card stands that many cards in from the front of what is
	// left. Once it has come out it is gone from the shoe.
	note, ink := "", color.Color(palette.Fog)
	cut := -1.0
	switch {
	case g.shuffling:
		note, ink = "SHUFFLING", palette.Gold
	case g.m.CutOut():
		note, ink = "CUT CARD OUT: SHUFFLE NEXT", palette.Gold
	default:
		cut = float64(g.m.ToCut) / table.ShoeSize
		if g.m.Shuffled && len(g.m.Hands) > 0 {
			note = "NEW SHOE"
		}
	}
	ui.DrawShoe(screen, box, full, cut, palette.Rust, note, ink)

	// The cut card itself, once it has come out: a plain yellow card dealt
	// onto the table beside the shoe, where it lies until the shuffle.
	if g.cutShown && !g.shuffling {
		ui.DrawCutCard(screen, box.X+box.W/2-ui.CardW*cutScale/2, box.Y+box.H+22, cutScale)
	}
}

// drawHands draws the dealer's hand and the player's.
func (g *Game) drawHands(screen *ebiten.Image) {
	const centre = tableX + tableW/2

	// The dealer's hand. Until the first deal the cards lie face down.
	//
	// While only one of the dealer's cards is on show a second lies face
	// down beside it, as at a table where the dealer is dealt two. It is
	// there for the look: no card has been drawn for it, so there is
	// nothing hidden to be learnt. The card that takes its place is drawn
	// when the dealer plays.
	dealer := g.m.Dealer[:min(g.dealerShown, len(g.m.Dealer))]
	label := "DEALER"
	switch len(dealer) {
	case 0:
		drawCards(screen, nil, 2, centre, dealerY, dealerW)
	case 1:
		label = "DEALER: " + totalText(dealer)
		drawCards(screen, dealer, 1, centre, dealerY, dealerW)
	default:
		label = "DEALER: " + totalText(dealer)
		drawCards(screen, dealer, 0, centre, dealerY, dealerW)
	}
	ui.DrawText(screen, label, centre, dealerY-17, 13, palette.White, ui.AlignCenter, ui.AlignStart)

	// The player's hand, or the two it was split into.
	if len(g.m.Hands) == 0 {
		drawCards(screen, nil, 2, centre, playerY, tableW-60)
		ui.DrawText(screen, "YOUR HAND", centre, playerY-17, 13, palette.White, ui.AlignCenter, ui.AlignStart)
	}
	for i := range g.m.Hands {
		h := &g.m.Hands[i]
		cx, width, name := float64(centre), float64(tableW-60), "YOUR HAND"
		if len(g.m.Hands) == 2 {
			cx = tableX + tableW/4 + float64(i)*tableW/2
			width = tableW/2 - 30
			name = fmt.Sprintf("HAND %d", i+1)
		}
		ink := color.Color(palette.White)
		if g.m.Dealt && i == g.m.Active {
			ink = palette.Gold
		}
		label := fmt.Sprintf("%s: %s    BET %s", name, totalText(h.Cards), bank.Money(h.Bet))
		ui.DrawText(screen, label, cx, playerY-17, 13, ink, ui.AlignCenter, ui.AlignStart)
		drawCards(screen, h.Cards, 0, cx, playerY, width)

		// Once the dealer has played, each hand is marked with how it went.
		if !g.m.Dealt && !g.revealing && i < len(g.m.Last.Outcomes) {
			tag, fill := "LOST", color.Color(palette.Crimson)
			switch g.m.Last.Outcomes[i] {
			case table.Won:
				tag, fill = "WON", palette.Forest
			case table.WonBlackjack:
				tag, fill = "BLACKJACK", palette.Forest
			case table.Pushed:
				tag, fill = "PUSH", palette.Cobalt
			}
			const tagW, tagH = 96, 20
			ui.FillRect(screen, cx-tagW/2, playerY+cardH-tagH-6, tagW, tagH, fill)
			ui.DrawText(screen, tag, cx, playerY+cardH-tagH/2-6, 13, palette.White, ui.AlignCenter, ui.AlignCenter)
		}
	}
}

// cutScale is the size of the cut card: a little smaller than the cards in
// the hands, as it is not one of them.
const cutScale = 0.5
