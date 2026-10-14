// Package table holds the Caribbean Stud rules: the bets and what they
// pay, and the odds. Hands are ranked by the pokerhand package, and the
// credits come from the bank. It has no graphics so it can be tested on its
// own.
package table

import (
	"videocasino/cards"
	"videocasino/pokerhand"
)

// HandSize is the number of cards in a hand.
const HandSize = pokerhand.Size

// Hand is a hand of five cards.
type Hand = pokerhand.Hand

// RaisePays is what the raise is paid for each credit bet, on top of the
// bet itself, when the player beats a dealer who qualifies with a hand of
// each category. Even a hand that makes nothing pays 1 to 1 if it wins.
var RaisePays = [pokerhand.Categories]int{
	pokerhand.HighCard:      1,
	pokerhand.Pair:          1,
	pokerhand.TwoPair:       2,
	pokerhand.ThreeOfAKind:  3,
	pokerhand.Straight:      4,
	pokerhand.Flush:         5,
	pokerhand.FullHouse:     7,
	pokerhand.FourOfAKind:   20,
	pokerhand.StraightFlush: 50,
	pokerhand.RoyalFlush:    100,
}

// DealerNeeds is the least the dealer must hold to qualify: an ace and a
// king, with nothing more.
var DealerNeeds = pokerhand.Least(pokerhand.HighCard, cards.Ace, cards.King)

// Qualifies reports whether a dealer's hand with this score is good enough
// to play: an ace and a king or better.
func Qualifies(s pokerhand.Score) bool {
	// Every hand that makes something scores more than any that does not.
	return s >= DealerNeeds
}

// Outcome is how the ante and the raise went.
type Outcome int

const (
	Folded       Outcome = iota // the player gave up the ante
	Lost                        // the dealer qualified and had the better hand
	Pushed                      // the dealer qualified and the hands tied
	Won                         // the dealer qualified and the player's hand was better
	NotQualified                // the dealer did not qualify
)

// RaiseSize is the raise, in antes.
const RaiseSize = 2

// Settle returns how a hand that was raised went against the dealer's, and
// what the ante and the raise together are paid for each credit of the
// ante, themselves included.
//
// If the dealer does not qualify the ante wins and the raise is handed
// back. If the dealer does, the better hand wins both: the ante at 1 to 1
// and the raise at what the player's hand pays. A tie hands both back.
func Settle(player, dealer pokerhand.Score) (o Outcome, paid int) {
	const staked = 1 + RaiseSize
	switch {
	case !Qualifies(dealer):
		return NotQualified, 2 + RaiseSize
	case player > dealer:
		return Won, 2 + RaiseSize*(1+RaisePays[player.Category()])
	case player == dealer:
		return Pushed, staked
	}
	return Lost, 0
}
