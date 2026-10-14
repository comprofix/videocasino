package ui

import (
	"fmt"
	"testing"
)

// newMenu returns a menu of two columns of four with the given number of
// entries, and an entry under them.
func newMenu(items int) *Menu {
	m := &Menu{Foot: "EXIT", Columns: 2, Rows: 4, Y: 150, RowH: 38}
	for i := range items {
		m.Items = append(m.Items, MenuItem{Title: fmt.Sprint("GAME ", i)})
	}
	return m
}

// layout describes each page of a menu: P for the way to the page before,
// N for the way to the next and a digit for how many entries lie between.
func layout(m *Menu) string {
	s := ""
	for i, p := range m.pages() {
		if i > 0 {
			s += " "
		}
		items := 0
		for _, pl := range p {
			if pl.kind == placeItem {
				items++
			}
		}
		if p[0].kind == placePrev {
			s += "P"
		}
		s += fmt.Sprint(items)
		if p[len(p)-1].kind == placeNext {
			s += "N"
		}
	}
	return s
}

func TestMenuPages(t *testing.T) {
	for _, c := range []struct {
		items int
		want  string
	}{
		{3, "3"},
		{8, "8"},
		{9, "7N P2"},
		{14, "7N P7"},
		{15, "7N P6N P2"},
		{20, "7N P6N P7"},
		{21, "7N P6N P6N P2"},
	} {
		m := newMenu(c.items)
		if got := layout(m); got != c.want {
			t.Errorf("%d entries were set out as %q, want %q", c.items, got, c.want)
		}
		// Every entry is on a page, once, and in order.
		next := 0
		for _, p := range m.pages() {
			if len(p) > m.Columns*m.Rows {
				t.Errorf("%d entries: a page has %d places", c.items, len(p))
			}
			for _, pl := range p {
				if pl.kind == placeItem {
					if pl.item != next {
						t.Errorf("%d entries: entry %d came where %d should", c.items, pl.item, next)
					}
					next++
				}
			}
		}
		if next != c.items {
			t.Errorf("%d entries: %d were set out", c.items, next)
		}
	}
}

func TestMenuColumns(t *testing.T) {
	for _, c := range []struct{ items, cols, rows int }{
		{1, 1, 1},
		{3, 1, 3},
		{4, 2, 2},
		{5, 2, 3},
		{8, 2, 4},
		{20, 2, 4},
	} {
		m := newMenu(c.items)
		if cols, rows := m.columns(); cols != c.cols || rows != c.rows {
			t.Errorf("%d entries were set out in %d columns of %d, want %d of %d",
				c.items, cols, rows, c.cols, c.rows)
		}
	}

	// Five entries read down the left column, then down the right, and the
	// entry under them is in the middle of the screen.
	m := newMenu(5)
	left, right := m.rect(0), m.rect(3)
	if m.rect(2).X != left.X || m.rect(2).Y <= m.rect(1).Y {
		t.Error("the third entry is not under the second")
	}
	if right.X <= left.X || right.Y != left.Y || m.rect(4).X != right.X {
		t.Error("the fourth entry does not start the right column")
	}
	foot := m.rect(5)
	if mid := foot.X + foot.W/2; mid < ScreenW/2-0.01 || mid > ScreenW/2+0.01 || foot.Y <= m.rect(2).Y {
		t.Error("the entry under the columns is not in the middle below them")
	}
	if left.X+right.X+right.W != ScreenW {
		t.Error("the columns are not in the middle of the screen")
	}
}

func TestMenuMove(t *testing.T) {
	m := newMenu(3)
	// Down through the three entries to the one under them, and round.
	for _, want := range []int{1, 2, 3, 0} {
		if m.move(1); m.at != want {
			t.Fatalf("moved down to place %d, want %d", m.at, want)
		}
	}
	if m.move(-1); m.at != 3 {
		t.Errorf("moved up from the top to place %d, want the last", m.at)
	}
	if got := m.pick(m.at); got != MenuFoot {
		t.Errorf("choosing the entry under the columns gave %d", got)
	}
	if m.move(-1); m.pick(m.at) != 2 {
		t.Error("choosing the last entry of the list did not give it")
	}
}

func TestMenuTurn(t *testing.T) {
	m := newMenu(15) // 7N P6N P2

	if m.turn(-1) {
		t.Error("turned back from the first page")
	}
	// The last place on the first page is the way to the next, and the
	// player is put on that page's first entry, not on the way back.
	if got := m.pick(7); got != MenuNone || m.page != 1 || m.at != 1 {
		t.Fatalf("NEXT PAGE gave %d and left page %d, place %d", got, m.page, m.at)
	}
	if got := m.pick(m.at); got != 7 {
		t.Errorf("the first entry on the second page is %d, want 7", got)
	}
	if !m.turn(1) || m.page != 2 || m.turn(1) {
		t.Errorf("turning on from the second page left page %d", m.page)
	}
	if got := m.pick(2); got != 14 {
		t.Errorf("the last entry is %d, want 14", got)
	}
	if got := m.pick(0); got != MenuNone || m.page != 1 {
		t.Errorf("PREVIOUS PAGE gave %d and left page %d", got, m.page)
	}

	m.Reset()
	if m.page != 0 || m.at != 0 {
		t.Errorf("reset left page %d, place %d", m.page, m.at)
	}
}

func TestMenuDigit(t *testing.T) {
	m := newMenu(15) // 7N P6N P2
	if got := m.digit(3); got != 2 || m.at != 2 {
		t.Errorf("3 chose %d and left the player on place %d", got, m.at)
	}
	if got := m.digit(0); got != MenuFoot {
		t.Errorf("0 chose %d, want the entry under the columns", got)
	}
	if got := m.digit(9); got != MenuNone {
		t.Errorf("9 chose %d on a page of eight", got)
	}
	// On a later page 1 is the way back and 8 the way on.
	if m.digit(8); m.page != 1 {
		t.Fatalf("8 left page %d", m.page)
	}
	if got := m.digit(2); got != 7 {
		t.Errorf("2 on the second page chose %d, want 7", got)
	}
	if m.digit(8); m.page != 2 {
		t.Errorf("8 on the second page left page %d", m.page)
	}
	if m.digit(1); m.page != 1 {
		t.Errorf("1 on the last page left page %d", m.page)
	}

	// A menu with no entry under it has nothing on 0.
	m = &Menu{Items: []MenuItem{{Title: "A"}, {Title: "B"}}, Columns: 1, Rows: 4}
	if got := m.digit(0); got != MenuNone {
		t.Errorf("0 chose %d on a menu with nothing under it", got)
	}
	if got := m.digit(2); got != 1 {
		t.Errorf("2 chose %d, want 1", got)
	}
}

// A menu whose entries have gone is brought back within what is left.
func TestMenuShrinks(t *testing.T) {
	m := newMenu(15)
	m.turn(1)
	m.turn(1)
	m.Items = m.Items[:2]
	if p := m.shown(); m.page != 0 || len(p) != 3 || m.at >= len(p) {
		t.Errorf("left on page %d, place %d of %d", m.page, m.at, len(p))
	}
}
