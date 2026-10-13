package poker

// MaxCoins is the most coins that can be bet on a hand.
const MaxCoins = 5

// Paytable is what each hand pays. Changing one changes the odds;
// TestReturn reports the resulting return to player.
type Paytable struct {
	Name string

	// Pays is the payout per coin bet.
	Pays [NumCategories]int

	// RoyalMax is the payout per coin for a royal flush when MaxCoins are
	// bet. It is more than Pays gives, which is the reason to bet the
	// maximum.
	RoyalMax int
}

// NineSix is full-pay Jacks or Better, known as 9/6 after what it pays for
// a full house and a flush.
var NineSix = Paytable{
	Name: "Jacks or Better",
	Pays: [NumCategories]int{
		JacksOrBetter: 1,
		TwoPair:       2,
		ThreeOfAKind:  3,
		Straight:      4,
		Flush:         6,
		FullHouse:     9,
		FourOfAKind:   25,
		StraightFlush: 50,
		RoyalFlush:    250,
	},
	RoyalMax: 800,
}

// Pay is the payout in coins for a hand when the given number of coins was
// bet on it.
func (p *Paytable) Pay(c Category, coins int) int {
	if c == RoyalFlush && coins == MaxCoins {
		return p.RoyalMax * coins
	}
	return p.Pays[c] * coins
}
