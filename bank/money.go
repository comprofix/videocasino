package bank

import (
	"fmt"
	"math"
	"strings"
)

// Each credit is a cent. The slot counts in credits, as poker machines do;
// the other games show money as dollars, written by these.

// Dollars writes an amount as dollars and cents, such as "$1,000.00".
func Dollars(credits int) string {
	return sign(credits) + "$" + thousands(abs(credits)/100) + fmt.Sprintf(".%02d", abs(credits)%100)
}

// Money writes an amount as dollars, leaving out the cents when there are
// none: "$25", or "$7.50".
func Money(credits int) string {
	if credits%100 == 0 {
		return sign(credits) + "$" + thousands(abs(credits)/100)
	}
	return Dollars(credits)
}

func abs(n int) int {
	return max(n, -n)
}

func sign(n int) string {
	if n < 0 {
		return "-"
	}
	return ""
}

// thousands writes a whole number with a comma between each three figures.
func thousands(n int) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Worth writes what something is worth on average, in credits that may be
// part of a cent, as dollars and cents with a sign: "+$1.23", or "-$0.50".
func Worth(credits float64) string {
	cents := int(math.Round(credits))
	if cents < 0 {
		return Dollars(cents)
	}
	return "+" + Dollars(cents)
}
