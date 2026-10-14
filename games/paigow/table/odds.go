package table

import "math"

// What the game pays back can not be counted exactly in reasonable time:
// every hand of seven against every hand of seven is about 10^15 pairs,
// each set the house way. So it is the one figure in the program that is
// estimated, by playing a great many hands out with the player setting
// them the house way too. TestEstimate plays them, only when asked:
//
//	go test ./games/paigow/table -run TestEstimate -long -timeout 0
//
// and -update writes what it counted into estimate.go. The hands are drawn
// from fixed seeds, so the same count comes out every time.

// Return returns the estimated fraction of what is bet that is paid back,
// for a player who sets every hand the house way.
func Return() float64 {
	n := estimateWon + estimatePushed + estimateLost
	if n == 0 {
		return 0
	}
	won := float64(estimateWon) * (1 - Commission/100.0)
	return 1 + (won-float64(estimateLost))/float64(n)
}

// Margin returns how far the true return could lie from the estimate: two
// standard errors either way, which it is inside 95 times in 100.
func Margin() float64 {
	n := float64(estimateWon + estimatePushed + estimateLost)
	if n == 0 {
		return 0
	}
	win := 1 - Commission/100.0
	mean := Return() - 1
	square := (float64(estimateWon)*win*win + float64(estimateLost)) / n
	return 2 * math.Sqrt((square-mean*mean)/n)
}

// Shares returns how often a hand is won, pushed and lost.
func Shares() (won, pushed, lost float64) {
	n := float64(estimateWon + estimatePushed + estimateLost)
	if n == 0 {
		return 0, 0, 0
	}
	return float64(estimateWon) / n, float64(estimatePushed) / n, float64(estimateLost) / n
}
