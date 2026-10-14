package baccarat

// The game is saved after every change, so it can be closed at any moment
// and picked up where it was left: the chip, the bets on the table, how the
// hands of the shoe have gone and the shoe itself. The credits are saved
// with it, as they are the bank's.

// saveName is the name the game's section of the save file goes by.
const saveName = "baccarat"

// save keeps the game as it stands, and the bank with it. A failure is
// reported on screen once, not on every hand.
func (g *Game) save() {
	err := g.bank.Keep(saveName, g.m.Save())
	if err == nil {
		err = g.bank.Save()
	}
	if err != nil && !g.saveFailed {
		g.saveFailed = true
		g.message = "COULD NOT SAVE THE GAME"
	}
}
