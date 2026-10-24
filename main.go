// Video Casino: several casino games in one program, played for pretend
// credits from one balance.
package main

import (
	"fmt"
	"image/color"
	"log"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"videocasino/bank"
	"videocasino/games/baccarat"
	"videocasino/games/blackjack"
	"videocasino/games/caribbean"
	"videocasino/games/holdem"
	"videocasino/games/paigow"
	"videocasino/games/roulette"
	"videocasino/games/slot"
	"videocasino/games/threecard"
	"videocasino/games/videopoker"
	"videocasino/ui"
)

// game is one of the games on the menu. It has the whole screen while it is
// being played, and draws its own background and border.
type game interface {
	// Update plays one tick. It reports true when the player has asked to
	// go back to the main menu.
	Update() (leave bool)
	Draw(screen *ebiten.Image)
}

// entry is a line on the menu. Its game is nil until the game has been
// moved in, and the line is then shown dimmed.
type entry struct {
	title string
	game  game
}

type scene int

const (
	sceneMenu scene = iota
	sceneGame
	sceneBye
)

// The menu's own colour: a deep red, under a gold border.
var carpet = color.RGBA{96, 0, 0, 255}

const (
	rowsY = 150 // the first line of the menu
	rowH  = 38

	creditsY = 388
	messageY = 444
	buttonY  = 476
	buttonH  = 40
)

var (
	coinButton  = ui.Button{X: 232, Y: buttonY, W: 240, H: buttonH}
	resetButton = ui.Button{X: 478, Y: buttonY, W: 90, H: buttonH}
)

type App struct {
	scene scene
	snd   *ui.Sounds
	bank  *bank.Bank

	entries []entry
	menu    *ui.Menu
	playing game // the game being played, while there is one

	message    string
	saveFailed bool
	confirm    ui.Confirm // asks before the credits are reset

	byeTicks int
}

func newApp() *App {
	a := &App{snd: ui.NewSounds()}

	// Pick up the saved bank, if there is one. A program with nowhere to
	// save still plays; it just starts afresh each time.
	a.bank = bank.New()
	if path, err := bank.Path(); err == nil {
		a.bank = bank.Open(path)
	}
	a.snd.On = a.bank.Sound

	// The games are listed in alphabetical order, whatever order they are
	// given in here.
	a.entries = []entry{
		{title: "VIDEO POKER", game: videopoker.New(a.bank, a.snd)},
		{title: "BLACKJACK", game: blackjack.New(a.bank, a.snd)},
		{title: "BACCARAT", game: baccarat.New(a.bank, a.snd)},
		{title: "SLOT MACHINE", game: slot.New(a.bank, a.snd)},
		{title: "ROULETTE", game: roulette.New(a.bank, a.snd)},
		{title: "THREE CARD POKER", game: threecard.New(a.bank, a.snd)},
		{title: "CARIBBEAN STUD", game: caribbean.New(a.bank, a.snd)},
		{title: "ULTIMATE HOLD'EM", game: holdem.New(a.bank, a.snd)},
		{title: "PAI GOW POKER", game: paigow.New(a.bank, a.snd)},
	}
	slices.SortFunc(a.entries, func(x, y entry) int { return strings.Compare(x.title, y.title) })

	// Two columns of four, with the way out under them.
	a.menu = &ui.Menu{
		Foot:    "EXIT",
		Columns: 2, Rows: 4,
		Y: rowsY, RowH: rowH,
		Number: ui.White, Title: ui.White,
	}
	for _, e := range a.entries {
		a.menu.Items = append(a.menu.Items, ui.MenuItem{Title: e.title, Dim: e.game == nil})
	}
	return a
}

// save keeps the bank as it stands. A failure is reported on screen once,
// not on every change.
func (a *App) save() {
	if err := a.bank.Save(); err != nil && !a.saveFailed {
		a.saveFailed = true
		a.message = "COULD NOT SAVE THE GAME"
	}
}

func (a *App) Update() error {
	if ui.Pressed(ebiten.KeyS) {
		a.bank.Sound = !a.bank.Sound
		a.snd.On = a.bank.Sound
		a.save()
	}

	switch a.scene {
	case sceneMenu:
		a.updateMenu()

	case sceneGame:
		if a.playing.Update() {
			a.scene = sceneMenu
			a.message = ""
		}

	case sceneBye:
		a.byeTicks++
		if a.byeTicks > 100 {
			return ebiten.Termination
		}
	}
	return nil
}

func (a *App) updateMenu() {
	// While the player is asked about a reset, nothing else moves.
	if a.confirm.Open {
		if a.confirm.Update() {
			a.reset()
		}
		return
	}

	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	x, y := ebiten.CursorPosition()
	hit := func(b ui.Button) bool { return click && b.Hit(x, y) }

	if ui.Pressed(ebiten.KeyC) || hit(coinButton) {
		a.insertCoin()
	}
	if ui.Pressed(ebiten.KeyR) || hit(resetButton) {
		a.askReset()
		return
	}

	switch chosen := a.menu.Update(); chosen {
	case ui.MenuNone:
	case ui.MenuFoot, ui.MenuBack:
		a.scene = sceneBye
		a.snd.Play(a.snd.Bye)
	default:
		a.choose(chosen)
	}
}

// choose starts the game on the given line of the menu.
func (a *App) choose(i int) {
	e := a.entries[i]
	if e.game == nil {
		a.message = e.title + " IS NOT HERE YET"
		return
	}
	a.snd.Play(a.snd.Click)
	a.playing = e.game
	a.scene = sceneGame
}

// askReset asks the player to make sure before the credits are reset.
func (a *App) askReset() {
	a.confirm.Ask("RESET CREDITS TO "+bank.Money(bank.StartCredits)+"?", "YOU HAVE "+bank.Dollars(a.bank.Credits))
}

// reset puts the credits back to the starting amount.
func (a *App) reset() {
	a.bank.Reset()
	a.message = "CREDITS RESET"
	a.snd.Play(a.snd.Click)
	a.save()
}

// insertCoin adds credits, so play can go on after they run out.
// A coin that would go over the limit is refused.
func (a *App) insertCoin() {
	if !a.bank.InsertCoin() {
		a.message = "COIN REFUSED - LIMIT IS " + bank.Money(bank.CoinLimit)
		return
	}
	a.message = fmt.Sprintf("COIN INSERTED - %s ADDED", bank.Money(bank.CoinCredits))
	a.snd.Play(a.snd.Coin)
	a.save()
}

func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ui.ScreenW, ui.ScreenH
}

func (a *App) Draw(screen *ebiten.Image) {
	if a.scene == sceneGame {
		a.playing.Draw(screen)
		return
	}

	screen.Fill(carpet)
	ui.DrawBorder(screen, ui.Gold)
	if a.scene == sceneBye {
		ui.DrawText(screen, "Thanks for playing", ui.ScreenW/2, ui.ScreenH/2, 44, ui.Yellow, ui.AlignCenter, ui.AlignCenter)
		return
	}
	a.drawMenu(screen)
}

func (a *App) drawMenu(screen *ebiten.Image) {
	ui.DrawText(screen, "VIDEO CASINO", ui.ScreenW/2, 70, 48, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	a.menu.Draw(screen)
	ui.DrawText(screen, "MAKE YOUR CHOICE", ui.ScreenW/2, a.menu.Bottom()+12, 24, ui.LightCyan, ui.AlignCenter, ui.AlignStart)

	ui.DrawText(screen, "CREDITS", ui.ScreenW/2, creditsY, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, bank.Dollars(a.bank.Credits), ui.ScreenW/2, creditsY+13, 32, ui.LightGreen, ui.AlignCenter, ui.AlignStart)
	ui.DrawText(screen, a.message, ui.ScreenW/2, messageY, 16, ui.Yellow, ui.AlignCenter, ui.AlignStart)

	coin := fmt.Sprintf("INSERT COIN +$%d", bank.CoinCredits/100)
	ui.DrawCoinButton(screen, coinButton, 14, coin)
	ui.DrawResetButton(screen, resetButton, 13)

	sound := "ON"
	if !a.snd.On {
		sound = "OFF"
	}
	hint := fmt.Sprintf("ARROWS move    ENTER or NUMBER choose    C coin    R reset    S sound %s    ESC exit", sound)
	ui.DrawText(screen, hint, ui.ScreenW/2, 550, 11, ui.LightGray, ui.AlignCenter, ui.AlignStart)

	a.confirm.Draw(screen)
}

func main() {
	// Use a bigger window where the monitor has room for it.
	scale := 1.0
	if _, h := ebiten.Monitor().Size(); h >= 1000 {
		scale = 1.5
	}
	ebiten.SetWindowSize(int(ui.ScreenW*scale), int(ui.ScreenH*scale))
	ebiten.SetWindowTitle("Video Casino")
	ebiten.SetWindowIcon(ui.WindowIcons())
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(newApp()); err != nil {
		log.Fatal(err)
	}
}
