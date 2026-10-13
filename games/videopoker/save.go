package videopoker

import "videocasino/games/videopoker/poker"

// The game is saved after every change, so it can be closed at any moment
// and picked up where it was left: the bet and any hand that has been dealt
// but not yet drawn to. The credits are saved with it, as they are the
// bank's.

// saveName is the name the game's section of the save file goes by.
const saveName = "videopoker"

// saved is what is kept: the game itself, and how the player likes to play
// it.
type saved struct {
	poker.Saved
	AutoHold bool
}

// save keeps the game as it stands, and the bank with it. A failure is
// reported on screen once, not on every hand.
func (g *Game) save() {
	err := g.bank.Keep(saveName, saved{Saved: g.m.Save(), AutoHold: g.autoHold})
	if err == nil {
		err = g.bank.Save()
	}
	if err != nil && !g.saveFailed {
		g.saveFailed = true
		g.message = "COULD NOT SAVE THE GAME"
	}
}
