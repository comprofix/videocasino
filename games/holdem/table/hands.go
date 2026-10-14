// Package table holds the Ultimate Texas Hold'em rules: the bets and what
// they pay, the rounds of a hand, and the odds. Hands are ranked by the
// pokerhand package, and the credits come from the bank. It has no graphics
// so it can be tested on its own.
package table

import (
	"videocasino/cards"
	"videocasino/pokerhand"
)

// The cards of a hand: two each for the player and the dealer, and five on
// the board that both share, of which the flop is the first three.
const (
	HoleSize  = 2
	BoardSize = 5
	FlopSize  = 3
)

// BlindPays is what the blind is paid on a win, in halves of each credit
// bet on top of the bet itself, for a hand of each category: 3 is 3 to 2.
// Below a straight a win hands the blind back and pays nothing.
var BlindPays = [pokerhand.Categories]int{
	pokerhand.Straight:      2,
	pokerhand.Flush:         3,
	pokerhand.FullHouse:     6,
	pokerhand.FourOfAKind:   20,
	pokerhand.StraightFlush: 100,
	pokerhand.RoyalFlush:    1000,
}

// TripsPays is what Trips is paid for each credit bet, on top of the bet
// itself, for the player's best five of seven of each category, whatever
// the dealer holds.
var TripsPays = [pokerhand.Categories]int{
	pokerhand.ThreeOfAKind:  3,
	pokerhand.Straight:      5,
	pokerhand.Flush:         6,
	pokerhand.FullHouse:     8,
	pokerhand.FourOfAKind:   30,
	pokerhand.StraightFlush: 40,
	pokerhand.RoyalFlush:    50,
}

// DealerNeeds is the least the dealer must hold to qualify: a pair.
var DealerNeeds = pokerhand.Least(pokerhand.Pair)

// Qualifies reports whether a dealer's hand with this score is good enough
// for the ante to be played for: a pair or better.
func Qualifies(s pokerhand.Score) bool {
	return s >= DealerNeeds
}

// Outcome is how the ante, the blind and the play bet went.
type Outcome int

const (
	Folded Outcome = iota // the player gave up the ante and the blind
	Lost                  // the dealer had the better hand
	Pushed                // the hands tied
	Won                   // the player had the better hand
)

// Round is how far a hand has gone: which choice the player has to make.
type Round int

const (
	NoHand  Round = iota // no hand is waiting: bets are being made
	Preflop              // the player's cards are dealt: check, or bet 4 or 3 antes
	Flop                 // the flop is turned: check, or bet 2 antes
	River                // the board is all turned: bet 1 ante, or fold
)

// Bets returns the play bets that can be made in the round, in antes,
// largest first.
func (r Round) Bets() []int {
	switch r {
	case Preflop:
		return []int{4, 3}
	case Flop:
		return []int{2}
	case River:
		return []int{1}
	}
	return nil
}

// Shown returns how many of the board's cards are turned in the round.
func (r Round) Shown() int {
	switch r {
	case Flop:
		return FlopSize
	case River:
		return BoardSize
	}
	return 0
}

// MaxPlay is the largest play bet, in antes.
const MaxPlay = 4

// Settle returns how a hand that was played went against the dealer's,
// and what the ante, the blind and the play bet were paid, themselves
// included.
//
// The better hand wins. On a win the play bet pays 1 to 1, the ante 1 to 1
// if the dealer qualifies and is handed back if not, and the blind by
// BlindPays. On a loss the play bet and the blind are lost, and the ante
// too unless the dealer does not qualify. A tie hands all three back.
func Settle(player, dealer pokerhand.Score, ante, blind, play int) (o Outcome, paid int) {
	q := Qualifies(dealer)
	switch {
	case player > dealer:
		paid = 2*play + ante + blind + blind*BlindPays[player.Category()]/2
		if q {
			paid += ante
		}
		return Won, paid
	case player == dealer:
		return Pushed, ante + blind + play
	}
	if !q {
		paid = ante
	}
	return Lost, paid
}

// TripsPaid returns what Trips is paid, itself included, on the player's
// best five of seven: nothing if it is lost.
func TripsPaid(player pokerhand.Score, trips int) int {
	if pays := TripsPays[player.Category()]; pays > 0 {
		return trips * (pays + 1)
	}
	return 0
}

// seven puts the two cards in a hand and the five on the board together.
func seven(hole [HoleSize]cards.Card, board [BoardSize]cards.Card) []cards.Card {
	return append(hole[:], board[:]...)
}
