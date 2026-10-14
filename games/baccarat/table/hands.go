// Package table holds the Baccarat rules: the two hands and how they are
// drawn to, the bets and what each pays, and the shoe with its burnt cards
// and its cut card. The credits come from the bank. It has no graphics so
// it can be tested on its own.
package table

import "videocasino/cards"

const (
	// Decks is the number of decks in the shoe.
	Decks    = 8
	ShoeSize = Decks * cards.DeckSize

	// CutBehind is how many cards lie behind the cut card: the shoe is
	// dealt down to these, and one hand more.
	CutBehind = 14

	// Natural is the least that two cards must make to be a natural, which
	// ends the hand there and then.
	Natural = 8
)

// Value is what a card counts: an ace 1, a two to a nine its number, and a
// ten or a picture nothing.
func Value(c cards.Card) int {
	switch r := c.Rank(); {
	case r == cards.Ace:
		return 1
	case r >= cards.Ten:
		return 0
	default:
		return int(r)
	}
}

// Total is what a hand counts: the last figure of its cards added up, so
// that a seven and an eight make 5.
func Total(hand []cards.Card) int {
	total := 0
	for _, c := range hand {
		total += Value(c)
	}
	return total % 10
}

// BurnCount is how many cards are burnt after a shuffle when the given card
// is the first out of the shoe: as many as it counts, but ten for a ten or a
// picture.
func BurnCount(c cards.Card) int {
	if v := Value(c); v > 0 {
		return v
	}
	return 10
}

// The rules for a third card. Nobody chooses: they are the same at every
// table. They are given by totals and values alone, so that the odds are
// worked out from the very rules the hands are played by.

// PlayerDraws reports whether the player's hand takes a third card, given
// what its two cards make. It is asked only when neither hand has a
// natural.
func PlayerDraws(total int) bool {
	return total <= 5
}

// BankerDraws reports whether the banker's hand takes a third card, given
// what its two cards make and the value of the player's third card, or -1
// if the player took none. It is asked only when neither hand has a
// natural.
func BankerDraws(total, playerThird int) bool {
	if playerThird < 0 {
		// The player stood, so the banker plays as the player would.
		return total <= 5
	}
	switch total {
	case 0, 1, 2:
		return true
	case 3:
		return playerThird != 8
	case 4:
		return playerThird >= 2 && playerThird <= 7
	case 5:
		return playerThird >= 4 && playerThird <= 7
	case 6:
		return playerThird == 6 || playerThird == 7
	}
	return false
}

// Play deals a hand from draw: a card to the player, one to the banker, and
// the same again, then a third to either as the rules say.
func Play(draw func() cards.Card) (player, banker []cards.Card) {
	p1, b1, p2, b2 := draw(), draw(), draw(), draw()
	player, banker = []cards.Card{p1, p2}, []cards.Card{b1, b2}
	if Total(player) >= Natural || Total(banker) >= Natural {
		return player, banker
	}
	third := -1
	if PlayerDraws(Total(player)) {
		c := draw()
		player = append(player, c)
		third = Value(c)
	}
	if BankerDraws(Total(banker), third) {
		banker = append(banker, draw())
	}
	return player, banker
}

// Outcome is which hand won.
type Outcome int

const (
	PlayerWins Outcome = iota
	BankerWins
	Tied
)

// OutcomeOf returns which of two hands that have been played out won.
func OutcomeOf(player, banker []cards.Card) Outcome {
	switch p, b := Total(player), Total(banker); {
	case p > b:
		return PlayerWins
	case b > p:
		return BankerWins
	}
	return Tied
}

// IsPair reports whether a hand's first two cards are a pair: two of the
// same rank.
func IsPair(hand []cards.Card) bool {
	return len(hand) >= 2 && hand[0].Rank() == hand[1].Rank()
}

// Bet is one of the bets on the table.
type Bet int

// The bets, in the order they lie across the table.
const (
	PlayerPair Bet = iota
	Player
	Tie
	Banker
	BankerPair
	Bets // how many bets there are
)

func (b Bet) String() string {
	return [...]string{"PLAYER PAIR", "PLAYER", "TIE", "BANKER", "BANKER PAIR"}[b]
}

// Pays is what each bet pays for each credit bet, on top of the bet itself,
// which is handed back.
var Pays = [Bets]int{
	PlayerPair: 11,
	Player:     1,
	Tie:        8,
	Banker:     1,
	BankerPair: 11,
}

// BankerSix is what the banker's hand must win with for a bet on it to be
// paid only half. There is no commission on the banker's other wins.
const BankerSix = 6

// Paid returns what each bet is paid on a hand that has been played out,
// with a winning bet itself handed back. A bet on the player or the banker
// is handed back when the hands tie.
func Paid(bets [Bets]int, player, banker []cards.Card) (paid [Bets]int) {
	if IsPair(player) {
		paid[PlayerPair] = bets[PlayerPair] * (Pays[PlayerPair] + 1)
	}
	if IsPair(banker) {
		paid[BankerPair] = bets[BankerPair] * (Pays[BankerPair] + 1)
	}
	switch OutcomeOf(player, banker) {
	case PlayerWins:
		paid[Player] = bets[Player] * (Pays[Player] + 1)
	case BankerWins:
		paid[Banker] = bets[Banker] * (Pays[Banker] + 1)
		if Total(banker) == BankerSix {
			paid[Banker] = bets[Banker] + bets[Banker]*Pays[Banker]/2
		}
	case Tied:
		paid[Tie] = bets[Tie] * (Pays[Tie] + 1)
		paid[Player], paid[Banker] = bets[Player], bets[Banker]
	}
	return paid
}
