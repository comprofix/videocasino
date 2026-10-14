// Package wheel holds the roulette rules: the wheel with its single zero,
// the bets that can be made on the table and what each pays. The credits
// come from the bank. It has no graphics so it can be tested on its own.
package wheel

import (
	"fmt"
	"slices"
	"strings"
)

// Pockets is how many pockets the wheel has: 1 to 36 and a single zero.
const Pockets = 37

// Order is the numbers round the wheel, clockwise from the zero.
var Order = [Pockets]int{
	0, 32, 15, 19, 4, 21, 2, 25, 17, 34, 6, 27, 13, 36, 11, 30, 8, 23, 10,
	5, 24, 16, 33, 1, 20, 14, 31, 9, 22, 18, 29, 7, 28, 12, 35, 3, 26,
}

// Pocket returns where on the wheel number n is, counting clockwise from
// the zero.
func Pocket(n int) int {
	return slices.Index(Order[:], n)
}

var reds = [...]int{1, 3, 5, 7, 9, 12, 14, 16, 18, 19, 21, 23, 25, 27, 30, 32, 34, 36}

// IsRed reports whether n is a red number. The zero is neither red nor
// black.
func IsRed(n int) bool {
	return slices.Contains(reds[:], n)
}

// IsBlack reports whether n is a black number.
func IsBlack(n int) bool {
	return n != 0 && !IsRed(n)
}

// Kind is a kind of bet.
type Kind int

// The bets on the numbers themselves come first, then those round the
// outside of the table.
const (
	Straight Kind = iota // one number
	Split                // two that lie side by side
	Street               // a row of three
	Corner               // four that meet at a corner
	SixLine              // two rows of three
	Column               // one of the three long columns of twelve
	Dozen                // 1 to 12, 13 to 24 or 25 to 36
	Low                  // 1 to 18
	Evens                // the even numbers
	Reds
	Blacks
	Odds  // the odd numbers
	High  // 19 to 36
	Kinds // how many kinds there are
)

// Pays is what each kind of bet pays for each credit bet, on top of the bet
// itself, which is handed back.
var Pays = [Kinds]int{
	Straight: 35,
	Split:    17,
	Street:   11,
	Corner:   8,
	SixLine:  5,
	Column:   2,
	Dozen:    2,
	Low:      1,
	Evens:    1,
	Reds:     1,
	Blacks:   1,
	Odds:     1,
	High:     1,
}

// Spot is a place on the table where chips can be put: one bet.
type Spot struct {
	Kind    Kind
	Numbers []int // the numbers it wins on, lowest first
}

// Pays returns what the bet pays for each credit bet on it.
func (s Spot) Pays() int {
	return Pays[s.Kind]
}

// Covers reports whether the bet wins when n comes up.
func (s Spot) Covers(n int) bool {
	return slices.Contains(s.Numbers, n)
}

// String names the bet as it is called at the table.
func (s Spot) String() string {
	n := s.Numbers
	list := func() string {
		parts := make([]string, len(n))
		for i, v := range n {
			parts[i] = fmt.Sprint(v)
		}
		return strings.Join(parts, "/")
	}
	switch s.Kind {
	case Straight:
		return fmt.Sprintf("STRAIGHT UP %d", n[0])
	case Split:
		return "SPLIT " + list()
	case Street:
		// The two with the zero in them are not rows of the table.
		if n[0] == 0 {
			return "STREET " + list()
		}
		return fmt.Sprintf("STREET %d-%d", n[0], n[2])
	case Corner:
		return "CORNER " + list()
	case SixLine:
		return fmt.Sprintf("SIX LINE %d-%d", n[0], n[5])
	case Column:
		return fmt.Sprintf("COLUMN %d TO %d", n[0], n[len(n)-1])
	case Dozen:
		return [...]string{"1ST 12", "2ND 12", "3RD 12"}[n[0]/12]
	case Low:
		return "1 TO 18"
	case Evens:
		return "EVEN"
	case Reds:
		return "RED"
	case Blacks:
		return "BLACK"
	case Odds:
		return "ODD"
	}
	return "19 TO 36"
}

// Spots is every bet on the table. The numbers are laid out in twelve rows
// of three, 1-2-3 up to 34-35-36, with the zero across the end beside 1, 2
// and 3. The first 37 spots are the numbers themselves, so a number's
// straight-up bet is the spot of that number.
//
// A saved game refers to a spot by its place here, so the order must not
// change.
var Spots = spots()

func spots() []Spot {
	var all []Spot
	add := func(k Kind, numbers ...int) {
		all = append(all, Spot{k, numbers})
	}
	// pick returns the numbers from 1 to 36 that keep says to.
	pick := func(keep func(n int) bool) []int {
		var numbers []int
		for n := 1; n < Pockets; n++ {
			if keep(n) {
				numbers = append(numbers, n)
			}
		}
		return numbers
	}

	for n := range Pockets {
		add(Straight, n)
	}

	// A number splits with the next one up in its row, and with the one
	// beside it in the next row. The zero splits with each of 1, 2 and 3.
	for n := 1; n < Pockets; n++ {
		if n%3 != 0 {
			add(Split, n, n+1)
		}
		if n+3 < Pockets {
			add(Split, n, n+3)
		}
	}
	for n := 1; n <= 3; n++ {
		add(Split, 0, n)
	}

	for n := 1; n < Pockets; n += 3 {
		add(Street, n, n+1, n+2)
	}
	add(Street, 0, 1, 2)
	add(Street, 0, 2, 3)

	for n := 1; n+4 < Pockets; n++ {
		if n%3 != 0 {
			add(Corner, n, n+1, n+3, n+4)
		}
	}
	add(Corner, 0, 1, 2, 3)

	for n := 1; n+5 < Pockets; n += 3 {
		add(SixLine, n, n+1, n+2, n+3, n+4, n+5)
	}

	for c := 1; c <= 3; c++ {
		add(Column, pick(func(n int) bool { return n%3 == c%3 })...)
	}
	for d := range 3 {
		add(Dozen, pick(func(n int) bool { return (n-1)/12 == d })...)
	}

	// The bets at even money, in the order they lie along the table.
	add(Low, pick(func(n int) bool { return n <= 18 })...)
	add(Evens, pick(func(n int) bool { return n%2 == 0 })...)
	add(Reds, pick(IsRed)...)
	add(Blacks, pick(IsBlack)...)
	add(Odds, pick(func(n int) bool { return n%2 == 1 })...)
	add(High, pick(func(n int) bool { return n > 18 })...)
	return all
}

// Paid returns what the bets pay when n comes up, with the winning bets
// themselves handed back. The bets are in the order of Spots.
func Paid(bets []int, n int) int {
	total := 0
	for i, bet := range bets {
		if bet > 0 && Spots[i].Covers(n) {
			total += bet * (Spots[i].Pays() + 1)
		}
	}
	return total
}
