package roulette

import (
	"videocasino/games/roulette/wheel"
	"videocasino/ui"
)

// The table the bets are made on. The numbers lie in twelve columns of
// three with 1, 2 and 3 at the left, 3 at the top, and the zero across
// their end. The columns' own bets are at the right, and the dozens and the
// bets at even money run along underneath.
const (
	gridX, gridY = 130, 244 // the top left corner of the numbers
	cellW, cellH = 44, 40   // the size of a number
	gridCols     = 12
	gridRows     = 3
	columnW      = 56 // the width of a column's bet, at the end of its row
	outsideH     = 32 // the height of the dozens, and of the even money bets

	dozensY = gridY + gridRows*cellH
	evensY  = dozensY + outsideH

	// chipRoom is the room kept for a chip at the right of a bet along the
	// bottom, clear of its name.
	chipRoom = 24

	// edge is how near the edge of a number a click must be to count as a
	// bet on the line or the corner there, not on the number.
	edge = 9
)

// point is a place on the table where the marker can be, and so where a bet
// can be made. Across the numbers it counts in half numbers, from the top
// left corner of the 3: a number's middle is at odd x and odd y, the lines
// between numbers are at even ones, and the line under the bottom row is
// y = 6. The zero is at x = -1 and the columns' bets at x = 25. The dozens
// and the even money bets have rows of their own further down, where x
// says only which of them the marker is on.
type point struct{ x, y int }

const (
	zeroX   = -1
	lastX   = 2*gridCols - 1 // the middle of the last column of numbers
	columnX = lastX + 2
	streetY = 2 * gridRows // the line the streets and six lines are bet on
	dozenY  = streetY + 2
	evenY   = dozenY + 2

	dozenSpan = 8 // how far along the numbers a dozen runs, and
	evenSpan  = 4 // an even money bet
)

// middle returns where on the table number n is.
func middle(n int) point {
	if n == 0 {
		return point{zeroX, gridRows}
	}
	return point{2*((n-1)/3) + 1, 2*(2-(n-1)%3) + 1}
}

// The spots of the bets round the outside, in their order along the table,
// and where each of the others is on it.
var (
	dozenSpots []int
	evenSpots  []int
	spotPoint  = make([]point, len(wheel.Spots))
	pointSpot  = map[point]int{}
)

func init() {
	for i, s := range wheel.Spots {
		n := s.Numbers
		var p point
		switch s.Kind {
		case wheel.Straight:
			p = middle(n[0])
		case wheel.Split:
			// On the line between the two.
			a, b := middle(n[0]), middle(n[1])
			p = point{(a.x + b.x) / 2, (a.y + b.y) / 2}
			if n[0] == 0 {
				p = point{0, b.y}
			}
		case wheel.Street:
			// At the foot of its row; with the zero, where the three meet.
			p = point{middle(n[0]).x, streetY}
			if n[0] == 0 {
				p = point{0, (middle(n[1]).y + middle(n[2]).y) / 2}
			}
		case wheel.Corner:
			// Where the four meet; the first four, at the foot of the zero.
			a, b := middle(n[0]), middle(n[3])
			p = point{(a.x + b.x) / 2, (a.y + b.y) / 2}
			if n[0] == 0 {
				p = point{0, streetY}
			}
		case wheel.SixLine:
			// At the foot of the line between its two rows.
			p = point{(middle(n[0]).x + middle(n[3]).x) / 2, streetY}
		case wheel.Column:
			p = point{columnX, middle(n[0]).y}
		case wheel.Dozen:
			p = point{len(dozenSpots)*dozenSpan + dozenSpan/2, dozenY}
			dozenSpots = append(dozenSpots, i)
		default:
			p = point{len(evenSpots)*evenSpan + evenSpan/2, evenY}
			evenSpots = append(evenSpots, i)
		}
		spotPoint[i] = p
		if p.y < dozenY {
			pointSpot[p] = i
		}
	}
}

// box returns which of the bets in a row along the bottom x is over.
func box(x, span int) int {
	return min(max(x, 0), lastX) / span
}

// spotAt returns the spot the marker is on at p.
func spotAt(p point) (int, bool) {
	switch p.y {
	case dozenY:
		return dozenSpots[box(p.x, dozenSpan)], true
	case evenY:
		return evenSpots[box(p.x, evenSpan)], true
	}
	spot, ok := pointSpot[p]
	return spot, ok
}

// move returns where the marker goes from p when it is moved one step
// across or down the table. Where there is nothing that way it stays put.
func move(p point, dx, dy int) point {
	switch {
	case p.y == dozenY:
		switch {
		case dx != 0:
			d := min(max(box(p.x, dozenSpan)+dx, 0), len(dozenSpots)-1)
			return point{d*dozenSpan + dozenSpan/2, dozenY}
		case dy < 0:
			return point{min(max(p.x, 0), lastX), streetY}
		case dy > 0:
			return point{p.x, evenY}
		}

	case p.y == evenY:
		switch {
		case dx != 0:
			e := min(max(box(p.x, evenSpan)+dx, 0), len(evenSpots)-1)
			return point{e*evenSpan + evenSpan/2, evenY}
		case dy < 0:
			return point{p.x, dozenY}
		}

	case p.x == zeroX:
		if dx > 0 {
			return point{0, p.y}
		}

	case p.x == columnX:
		switch {
		case dx < 0:
			return point{lastX, p.y}
		case dy != 0 && p.y+2*dy >= 1 && p.y+2*dy < streetY:
			return point{columnX, p.y + 2*dy}
		case dy > 0:
			return point{lastX, dozenY}
		}

	default:
		switch {
		case dx != 0 && p.x+dx < 0:
			return point{zeroX, gridRows}
		case dx != 0 && p.x+dx <= lastX:
			return point{p.x + dx, p.y}
		case dx != 0 && p.y%2 == 1:
			return point{columnX, p.y} // on from the end of a row
		case dy != 0 && p.y+dy > streetY:
			return point{p.x, dozenY}
		case dy != 0 && p.y+dy >= 1:
			return point{p.x, p.y + dy}
		}
	}
	return p
}

// The places of the bets round the outside.
func columnButton(row int) ui.Button {
	return ui.Button{X: gridX + gridCols*cellW, Y: gridY + float64(row)*cellH, W: columnW, H: cellH}
}

func dozenButton(i int) ui.Button {
	return ui.Button{X: gridX + float64(i)*4*cellW, Y: dozensY, W: 4 * cellW, H: outsideH}
}

func evenButton(i int) ui.Button {
	return ui.Button{X: gridX + float64(i)*2*cellW, Y: evensY, W: 2 * cellW, H: outsideH}
}

var zeroButton = ui.Button{X: gridX - cellW, Y: gridY, W: cellW, H: gridRows * cellH}

// numberButton is the place of a number from 1 to 36.
func numberButton(n int) ui.Button {
	p := middle(n)
	return ui.Button{X: gridX + float64(p.x-1)/2*cellW, Y: gridY + float64(p.y-1)/2*cellH, W: cellW, H: cellH}
}

// outside returns the place of the bet at p, if it is one of those round
// the outside.
func outside(p point) (ui.Button, bool) {
	switch {
	case p.y == dozenY:
		return dozenButton(box(p.x, dozenSpan)), true
	case p.y == evenY:
		return evenButton(box(p.x, evenSpan)), true
	case p.x == columnX:
		return columnButton(p.y / 2), true
	}
	return ui.Button{}, false
}

// chipAt returns where on the screen a chip bet at p lies: on the number,
// line or corner for a bet on the numbers, and in its box for the others.
func chipAt(p point) (x, y float64) {
	if b, ok := outside(p); ok {
		if p.x == columnX {
			return b.X + b.W/2, b.Y + b.H/2
		}
		return b.X + b.W - chipRoom/2 - 2, b.Y + b.H/2 // clear of the name of the bet
	}
	return gridX + float64(p.x)*cellW/2, gridY + float64(p.y)*cellH/2
}

// along returns which number of a row or column a click falls in, counted
// in half numbers, moved on to the line before or after it if the click is
// near that edge.
func along(at, size float64) int {
	n := int(at / size)
	h := 2*n + 1
	switch in := at - float64(n)*size; {
	case in < edge:
		h--
	case in >= size-edge:
		h++
	}
	return h
}

// pointAt returns the place on the table that is at (x, y) on the screen.
func pointAt(x, y int) (point, bool) {
	fx, fy := float64(x), float64(y)
	for row := range gridRows {
		if columnButton(row).Hit(x, y) {
			return point{columnX, 2*row + 1}, true
		}
	}
	// The line under the numbers, where the streets and six lines are bet,
	// takes in a little of what lies on either side of it.
	if fx >= gridX-edge && fx < gridX+gridCols*cellW && fy >= dozensY-edge && fy < dozensY+edge/2 {
		hx := 0
		if fx >= gridX {
			hx = min(along(fx-gridX, cellW), lastX)
		}
		return point{hx, streetY}, true
	}
	for i := range dozenSpots {
		if dozenButton(i).Hit(x, y) {
			return point{i*dozenSpan + dozenSpan/2, dozenY}, true
		}
	}
	for i := range evenSpots {
		if evenButton(i).Hit(x, y) {
			return point{i*evenSpan + evenSpan/2, evenY}, true
		}
	}

	if zeroButton.Hit(x, y) {
		// All of the zero is the zero, but for the edge it shares with 1, 2
		// and 3.
		if fx < gridX-edge {
			return point{zeroX, gridRows}, true
		}
		return point{0, max(along(fy-gridY, cellH), 1)}, true
	}
	if fx < gridX || fx >= gridX+gridCols*cellW || fy < gridY || fy >= dozensY {
		return point{}, false
	}

	// A line or a corner if there is a bet there, or else the number.
	col, row := int((fx-gridX)/cellW), int((fy-gridY)/cellH)
	hx, hy := along(fx-gridX, cellW), along(fy-gridY, cellH)
	for _, p := range []point{{hx, hy}, {2*col + 1, hy}, {hx, 2*row + 1}} {
		if _, ok := pointSpot[p]; ok {
			return p, true
		}
	}
	return point{2*col + 1, 2*row + 1}, true
}
