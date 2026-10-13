// Package cards holds the playing cards that card games share: the card
// itself, its rank and its suit. It has no graphics so it can be tested on
// its own.
package cards

const (
	DeckSize = 52
	NumRanks = 13
	NumSuits = 4
)

// Card is one of the 52 cards. They are numbered rank by rank, from the two
// of clubs at 0 to the ace of spades at 51.
type Card int

// Rank is a card's rank, numbered as it is on the card: a two is 2 and an
// ace, which is high, is 14.
type Rank int

const (
	Two Rank = iota + 2
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
)

type Suit int

const (
	Clubs Suit = iota
	Diamonds
	Hearts
	Spades
)

func New(r Rank, s Suit) Card {
	return Card(int(r-Two)*NumSuits + int(s))
}

func (c Card) Rank() Rank {
	return Two + Rank(c/NumSuits)
}

func (c Card) Suit() Suit {
	return Suit(c % NumSuits)
}

// String names a card in two letters, such as "Td" for the ten of diamonds.
func (c Card) String() string {
	return string("23456789TJQKA"[c/NumSuits]) + string("cdhs"[c%NumSuits])
}

// Valid reports whether c is a card in the deck.
func (c Card) Valid() bool {
	return c >= 0 && c < DeckSize
}
