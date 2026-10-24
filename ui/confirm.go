package ui

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The box a question is asked in, centred on the screen.
const (
	confirmW = 440
	confirmH = 210
	confirmX = (ScreenW - confirmW) / 2
	confirmY = (ScreenH - confirmH) / 2
)

var (
	confirmShade = color.RGBA{0, 0, 0, 170} // over the screen behind the box
	confirmFill  = color.RGBA{96, 0, 0, 255}

	confirmYes = Button{X: ScreenW/2 - 130, Y: confirmY + 118, W: 110, H: 40}
	confirmNo  = Button{X: ScreenW/2 + 20, Y: confirmY + 118, W: 110, H: 40}
)

// Confirm asks the player to make sure before something that can not be
// undone, in a box over the screen with YES and NO. It is the same wherever
// it is shown. The bar starts on NO, so a stray ENTER or click does nothing:
// Y answers yes and N or ESC no, the left and right arrows move the bar and
// ENTER chooses where it is, and a click on either button chooses it.
//
// While it is open the game behind it should do nothing but draw itself.
type Confirm struct {
	Open bool

	question, note string
	yes            bool // the bar is on YES

	// Where the pointer was last, so that it moves the bar only when it is
	// itself moved.
	seen         bool
	lastX, lastY int
}

// Ask opens the box with a question and a note under it.
func (c *Confirm) Ask(question, note string) {
	*c = Confirm{Open: true, question: question, note: note}
}

// Update takes the player's answer, if one has been given. It reports true
// when the answer is yes; the box is closed on either answer.
func (c *Confirm) Update() (yes bool) {
	switch {
	case Pressed(ebiten.KeyY):
		return c.answer(true)
	case Pressed(ebiten.KeyN, ebiten.KeyEscape):
		return c.answer(false)
	case Pressed(ebiten.KeyArrowLeft):
		c.yes = true
	case Pressed(ebiten.KeyArrowRight):
		c.yes = false
	case Pressed(ebiten.KeyEnter, ebiten.KeyNumpadEnter):
		return c.answer(c.yes)
	}

	x, y := ebiten.CursorPosition()
	moved := c.seen && (x != c.lastX || y != c.lastY)
	c.seen, c.lastX, c.lastY = true, x, y
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	for _, b := range []struct {
		button Button
		yes    bool
	}{{confirmYes, true}, {confirmNo, false}} {
		if !b.button.Hit(x, y) {
			continue
		}
		if click {
			return c.answer(b.yes)
		}
		if moved {
			c.yes = b.yes
		}
	}
	return false
}

func (c *Confirm) answer(yes bool) bool {
	c.Open = false
	return yes
}

// Draw shades the screen and draws the box over it, if it is open.
func (c *Confirm) Draw(dst *ebiten.Image) {
	if !c.Open {
		return
	}
	FillRect(dst, 0, 0, ScreenW, ScreenH, confirmShade)
	FillRect(dst, confirmX, confirmY, confirmW, confirmH, confirmFill)
	FrameRect(dst, confirmX, confirmY, confirmW, confirmH, 3, Gold)

	DrawText(dst, c.question, ScreenW/2, confirmY+46, 24, Yellow, AlignCenter, AlignCenter)
	DrawText(dst, c.note, ScreenW/2, confirmY+84, 16, LightGray, AlignCenter, AlignCenter)
	DrawButton(dst, confirmYes, c.yes, false, 20, "YES")
	DrawButton(dst, confirmNo, !c.yes, false, 20, "NO")
	DrawText(dst, "Y yes    N or ESC no    LEFT/RIGHT move    ENTER choose", ScreenW/2, confirmY+186, 11, LightGray, AlignCenter, AlignCenter)
}
