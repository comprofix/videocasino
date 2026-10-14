package ui

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// What a menu's Update reports when no entry of its list was chosen.
const (
	MenuNone = -1 - iota // nothing was chosen
	MenuFoot             // the entry under the columns was chosen
	MenuBack             // ESC was pressed on the first page
)

// The size of a line on a menu.
const (
	menuSize   = 24  // the text
	menuBarW   = 340 // the bar that marks the line the player is on
	menuBarH   = 32
	menuGap    = 16 // between two columns
	menuInset  = 30 // from the edge of the bar to the number
	menuNumber = 40 // from the number to the title
)

// MenuItem is an entry on a menu. A dimmed one is shown greyed out, but can
// still be chosen.
type MenuItem struct {
	Title string
	Dim   bool
}

// Menu is a list of entries to choose from, the same wherever one is shown.
// One entry is marked as the one the player is on. The up and down arrows
// move the mark and ENTER chooses the entry it is on; the number beside an
// entry chooses it at once, and so does a click.
//
// The entries are set out in columns, read down the left one and then down
// the next. When there are more than fit they go onto further pages: the
// last place on a page is then NEXT PAGE, and the first place on any page
// after the first is PREVIOUS PAGE. The left and right arrows turn the page
// too, and ESC turns back.
type Menu struct {
	Items []MenuItem
	// Foot is an entry of its own, centred under the columns on every page
	// and chosen with 0. A menu without one leaves it empty.
	Foot string

	// A page holds Columns x Rows places, which must come to three or more.
	Columns, Rows int
	Y, RowH       float64 // the top of the first line, and the step to the next

	Number, Title color.Color // the colours of an entry's number and title

	page, at int // the page on show, and the place on it the player is on

	// Where the pointer was last, so that it moves the mark only when it is
	// itself moved.
	seen         bool
	lastX, lastY int
}

type placeKind int

const (
	placeItem placeKind = iota
	placePrev
	placeNext
	placeFoot
)

// place is one of the lines on a page: an entry of the list, or a way to
// another page.
type place struct {
	kind placeKind
	item int
}

// The keys for each number, along the top of the keyboard and on the number
// pad.
var digitKeys = [10][2]ebiten.Key{
	{ebiten.KeyDigit0, ebiten.KeyNumpad0},
	{ebiten.KeyDigit1, ebiten.KeyNumpad1},
	{ebiten.KeyDigit2, ebiten.KeyNumpad2},
	{ebiten.KeyDigit3, ebiten.KeyNumpad3},
	{ebiten.KeyDigit4, ebiten.KeyNumpad4},
	{ebiten.KeyDigit5, ebiten.KeyNumpad5},
	{ebiten.KeyDigit6, ebiten.KeyNumpad6},
	{ebiten.KeyDigit7, ebiten.KeyNumpad7},
	{ebiten.KeyDigit8, ebiten.KeyNumpad8},
	{ebiten.KeyDigit9, ebiten.KeyNumpad9},
}

// pages sets the entries out page by page.
func (m *Menu) pages() [][]place {
	per := m.Columns * m.Rows
	var pages [][]place
	for next := 0; ; {
		var p []place
		if len(pages) > 0 {
			p = append(p, place{kind: placePrev})
		}
		// If the rest do not fit, the last place is the way to the next page.
		room := per - len(p)
		more := len(m.Items)-next > room
		if more {
			room--
		}
		for ; room > 0 && next < len(m.Items); room, next = room-1, next+1 {
			p = append(p, place{placeItem, next})
		}
		if more {
			p = append(p, place{kind: placeNext})
		}
		pages = append(pages, p)
		if !more {
			return pages
		}
	}
}

// shown returns the places on the page on show, with the foot last if the
// menu has one. It first brings the page and the mark back within the menu,
// in case its entries have changed.
func (m *Menu) shown() []place {
	pages := m.pages()
	m.page = min(max(m.page, 0), len(pages)-1)
	p := pages[m.page]
	if m.Foot != "" {
		p = append(p, place{kind: placeFoot})
	}
	m.at = min(max(m.at, 0), max(len(p)-1, 0))
	return p
}

// columns returns how many columns the menu is set out in, and how many
// lines the longest of them has on the first page. A column is not started
// for a single entry: a menu too short to put two in each has one fewer.
func (m *Menu) columns() (cols, rows int) {
	n := len(m.pages()[0])
	cols = max(1, min(m.Columns, n/2))
	return cols, (n + cols - 1) / cols
}

// rect returns where on the screen place i of the page on show is.
func (m *Menu) rect(i int) Button {
	p := m.shown()
	cols, rows := m.columns()
	if p[i].kind == placeFoot {
		w := 2*menuInset + menuNumber + TextWidth(m.Foot, menuSize)
		return Button{X: ScreenW/2 - w/2, Y: m.Y + float64(rows)*m.RowH + 3, W: w, H: menuBarH}
	}
	// The page is split evenly between the columns.
	n := len(p)
	if m.Foot != "" {
		n--
	}
	perCol := (n + cols - 1) / cols
	col, row := i/perCol, i%perCol
	x := ScreenW/2 - float64(cols*menuBarW+(cols-1)*menuGap)/2
	return Button{
		X: x + float64(col)*(menuBarW+menuGap),
		Y: m.Y + float64(row)*m.RowH - 3,
		W: menuBarW, H: menuBarH,
	}
}

// Bottom returns how far down the screen the menu reaches. It is the same on
// every page.
func (m *Menu) Bottom() float64 {
	_, rows := m.columns()
	if m.Foot != "" {
		rows++
	}
	return m.Y + float64(rows-1)*m.RowH + menuBarH
}

// Reset puts the menu back as it first was: on its first page, with the
// player on the first entry.
func (m *Menu) Reset() {
	m.page, m.at = 0, 0
}

// move moves the mark up or down the page, going round at either end.
func (m *Menu) move(by int) {
	n := len(m.shown())
	if n > 0 {
		m.at = ((m.at+by)%n + n) % n
	}
}

// turn turns to the page before or after, and puts the player on its first
// entry. It reports false if there is no page that way.
func (m *Menu) turn(by int) bool {
	m.shown()
	to := m.page + by
	if to < 0 || to >= len(m.pages()) {
		return false
	}
	m.page, m.at = to, 0
	for i, p := range m.shown() {
		if p.kind == placeItem {
			m.at = i
			break
		}
	}
	return true
}

// pick chooses place i of the page on show. It returns the entry chosen, or
// MenuFoot, or MenuNone if the place was a way to another page.
func (m *Menu) pick(i int) int {
	p := m.shown()
	if i < 0 || i >= len(p) {
		return MenuNone
	}
	m.at = i
	switch p[i].kind {
	case placeItem:
		return p[i].item
	case placeFoot:
		return MenuFoot
	case placePrev:
		m.turn(-1)
	case placeNext:
		m.turn(1)
	}
	return MenuNone
}

// digit chooses the place with the given number beside it.
func (m *Menu) digit(n int) int {
	p := m.shown()
	if m.Foot != "" {
		if n == 0 {
			return m.pick(len(p) - 1)
		}
		p = p[:len(p)-1]
	}
	if n < 1 || n > len(p) {
		return MenuNone
	}
	return m.pick(n - 1)
}

// Update plays one tick of the menu. It returns the entry of Items that was
// chosen, or MenuFoot, MenuBack or MenuNone.
func (m *Menu) Update() int {
	switch {
	case Pressed(ebiten.KeyArrowUp):
		m.move(-1)
	case Pressed(ebiten.KeyArrowDown):
		m.move(1)
	case Pressed(ebiten.KeyArrowLeft):
		m.turn(-1)
	case Pressed(ebiten.KeyArrowRight):
		m.turn(1)
	case Pressed(ebiten.KeyEnter, ebiten.KeyNumpadEnter):
		return m.pick(m.at)
	case Pressed(ebiten.KeyEscape):
		if !m.turn(-1) {
			return MenuBack
		}
	}
	for n, keys := range digitKeys {
		if Pressed(keys[:]...) {
			return m.digit(n)
		}
	}

	// The pointer moves the mark to the entry it is over, and a click
	// chooses that entry.
	x, y := ebiten.CursorPosition()
	moved := m.seen && (x != m.lastX || y != m.lastY)
	m.seen, m.lastX, m.lastY = true, x, y
	click := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if moved || click {
		for i := range m.shown() {
			if !m.rect(i).Hit(x, y) {
				continue
			}
			if click {
				return m.pick(i)
			}
			m.at = i
		}
	}
	return MenuNone
}

// Draw draws the page on show, with a bar behind the entry the player is on.
func (m *Menu) Draw(dst *ebiten.Image) {
	for i, p := range m.shown() {
		number, title, text := m.Number, m.Title, ""
		key := i + 1
		switch p.kind {
		case placeItem:
			text = m.Items[p.item].Title
			if m.Items[p.item].Dim {
				number, title = DarkGray, DarkGray
			}
		case placePrev:
			text = "PREVIOUS PAGE"
		case placeNext:
			text = "NEXT PAGE"
		case placeFoot:
			text, key = m.Foot, 0
		}

		b := m.rect(i)
		if i == m.at {
			FillRect(dst, b.X, b.Y, b.W, b.H, Yellow)
			if number != DarkGray {
				number, title = Black, Black
			}
		}
		DrawText(dst, fmt.Sprintf("%d.", key), b.X+menuInset, b.Y+3, menuSize, number, AlignStart, AlignStart)
		DrawText(dst, text, b.X+menuInset+menuNumber, b.Y+3, menuSize, title, AlignStart, AlignStart)
	}
}
