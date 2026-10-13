package table

import "videocasino/cards"

// The long-run odds of the game, worked out from the rules rather than by
// playing. Blackjack's return depends on how well it is played, so the
// figures here are for the best play: for every hand that can be held
// against every card the dealer can show, each choice is weighed up over
// every card that could follow and every way the dealer's hand could end.
//
// What is left in the shoe is known exactly: a full shoe less every card
// dealt since the shuffle. The suits make no difference
// to the main game, so the cards are counted by what they are worth.
//
// For a hand that has not been split this is exact. Splitting cannot be
// weighed up exactly in any reasonable time: the two hands share a shoe and
// a dealer, so the best play of one depends on every way the other could
// go. A split hand is weighed up from the cards on the table, as if the
// other hand were to draw no more. That is exact for the second of the two
// hands once the first is finished, and close for the rest.

// shoe is the cards left to be dealt, counted by value: left[1] is the
// aces and left[10] the tens and pictures.
type shoe struct {
	left [11]int
	n    int
}

// freshShoe returns a full shoe.
func freshShoe() shoe {
	var s shoe
	for c := range cards.Card(cards.DeckSize) {
		s.left[Value(c)] += Decks
	}
	s.n = ShoeSize
	return s
}

func (s *shoe) take(v int) {
	s.left[v]--
	s.n--
}

func (s *shoe) put(v int) {
	s.left[v]++
	s.n++
}

// ends is how often the dealer's hand ends each way: on each total from
// DealerStands to 21, bust, or with a blackjack.
type ends [7]float64

const (
	endBust      = 5
	endBlackjack = 6
)

// dealerEnds works out how the dealer's hand ends, from the card that
// shows and the cards left in the shoe, by following every card the dealer
// could draw.
func dealerEnds(up int, s *shoe) (e ends) {
	var draw func(hard int, ace bool, n int, p float64)
	draw = func(hard int, ace bool, n int, p float64) {
		total := hard
		if ace && hard+10 <= 21 {
			total = hard + 10
		}
		if n >= 2 && total >= DealerStands {
			switch {
			case total > 21:
				e[endBust] += p
			case n == 2 && total == 21:
				e[endBlackjack] += p
			default:
				e[total-DealerStands] += p
			}
			return
		}
		for v := 1; v <= 10; v++ {
			k := s.left[v]
			if k == 0 {
				continue
			}
			q := p * float64(k) / float64(s.n)
			s.take(v)
			draw(hard+v, ace || v == 1, n+1, q)
			s.put(v)
		}
	}
	draw(up, up == 1, 1, 1)
	return e
}

// blackjackChance is how often the dealer's next card makes a blackjack.
func blackjackChance(up int, s *shoe) float64 {
	switch up {
	case 1:
		return float64(s.left[10]) / float64(s.n)
	case 10:
		return float64(s.left[1]) / float64(s.n)
	}
	return 0
}

// stand is what a hand standing on total wins on average, in bets, when
// units bets are staked on it: one, or two if it was doubled. A dealer's
// blackjack costs it lose bets however many were staked.
func (e *ends) stand(total int, units, lose float64) float64 {
	win := e[endBust]
	for d := DealerStands; d <= 21; d++ {
		switch {
		case d < total:
			win += e[d-DealerStands]
		case d > total:
			win -= e[d-DealerStands]
		}
	}
	return units*win - lose*e[endBlackjack]
}

// holding is a player's hand, as far as its worth goes: how many cards of
// each value it has. key packs those counts into one number, five bits to
// a value.
type holding struct {
	key  uint64
	hard int // the total with every ace counted as 1
	ace  bool
	n    int
}

func (h holding) add(v int) holding {
	h.key += 1 << (5 * uint(v-1))
	h.hard += v
	h.ace = h.ace || v == 1
	h.n++
	return h
}

func (h holding) total() int {
	if h.ace && h.hard+10 <= 21 {
		return h.hard + 10
	}
	return h.hard
}

// solver works out the best play of one hand against one dealer's card,
// starting from a known shoe and a known holding. Each holding the hand can grow into fixes
// what is left in the shoe, so what is worked out for a holding can be
// kept and used again.
type solver struct {
	up int

	// lose is what a dealer's blackjack costs the hand, in bets. The
	// player loses the one bet first made, so that is 1 for a hand that
	// has not been split and half of it for each of two split hands.
	lose float64

	ends map[uint64]ends    // how the dealer ends against each holding
	hits map[uint64]float64 // what each holding is worth if it is hit
}

func newSolver(up int, lose float64) *solver {
	return &solver{up: up, lose: lose, ends: map[uint64]ends{}, hits: map[uint64]float64{}}
}

// stand is what the holding wins on average if it stands, in bets.
func (s *solver) stand(h holding, sh *shoe, units float64) float64 {
	e, ok := s.ends[h.key]
	if !ok {
		e = dealerEnds(s.up, sh)
		s.ends[h.key] = e
	}
	return e.stand(h.total(), units, s.lose)
}

// bust is what a hand that has gone bust loses on average, in bets, with
// units staked on it: all of them, unless the dealer then makes a
// blackjack. For one bet on a hand that was not split that is the bet
// either way.
func (s *solver) bust(sh *shoe, units float64) float64 {
	p := blackjackChance(s.up, sh)
	return units*(1-p) + s.lose*p
}

// hit is what the holding wins on average if it takes a card and is played
// as well as it can be from there. A hand that reaches 21 stands.
func (s *solver) hit(h holding, sh *shoe) float64 {
	if v, ok := s.hits[h.key]; ok {
		return v
	}
	total := 0.0
	for v := 1; v <= 10; v++ {
		k := sh.left[v]
		if k == 0 {
			continue
		}
		p := float64(k) / float64(sh.n)
		next := h.add(v)
		sh.take(v)
		var worth float64
		switch {
		case next.hard > 21:
			worth = -s.bust(sh, 1)
		case next.total() == 21:
			worth = s.stand(next, sh, 1)
		default:
			worth = max(s.stand(next, sh, 1), s.hit(next, sh))
		}
		sh.put(v)
		total += p * worth
	}
	s.hits[h.key] = total
	return total
}

// double is what the holding wins on average if it is doubled: two bets
// ride on one more card.
func (s *solver) double(h holding, sh *shoe) float64 {
	total := 0.0
	for v := 1; v <= 10; v++ {
		k := sh.left[v]
		if k == 0 {
			continue
		}
		p := float64(k) / float64(sh.n)
		next := h.add(v)
		sh.take(v)
		if next.hard > 21 {
			total -= p * s.bust(sh, 2)
		} else {
			total += p * s.stand(next, sh, 2)
		}
		sh.put(v)
	}
	return total
}

// Action is something the player can do with a hand.
type Action int

const (
	ActStand Action = iota
	ActHit
	ActDouble
	ActSplit
)

func (a Action) String() string {
	return [...]string{"Stand", "Hit", "Double", "Split"}[a]
}

// Advice is what each choice is worth for a hand, in bets won on average,
// and which is best of those that can be made. A hand can be doubled only
// on a hard 9, 10 or 11 in its first two cards, and split only if they
// count the same.
type Advice struct {
	Stand, Hit, Double, Split float64
	CanDouble, CanSplit       bool
	Best                      Action
}

// Value is what the hand is worth played the best way.
func (a Advice) Value() float64 {
	return [...]float64{a.Stand, a.Hit, a.Double, a.Split}[a.Best]
}

// choose picks the best of the choices that can be made.
func (a *Advice) choose() {
	a.Best = ActStand
	if a.Hit > a.Value() {
		a.Best = ActHit
	}
	if a.CanDouble && a.Double > a.Value() {
		a.Best = ActDouble
	}
	if a.CanSplit && a.Split > a.Value() {
		a.Best = ActSplit
	}
}

// advise weighs up a holding of at least two cards that can still take one.
func (s *solver) advise(h holding, sh *shoe) Advice {
	a := Advice{Stand: s.stand(h, sh, 1), Hit: s.hit(h, sh)}
	// A holding with an ace in its first two cards is soft.
	if h.n == 2 && !h.ace && h.hard >= DoubleFrom && h.hard <= DoubleTo {
		a.CanDouble = true
		a.Double = s.double(h, sh)
	}
	a.choose()
	return a
}

// Odds is the odds of the game from a freshly shuffled shoe, for a hand
// that is not split. Working them out takes a moment.
type Odds struct {
	solvers [11]*solver // against each card the dealer can show
	ret     float64
}

// NewOdds works out the odds from a freshly shuffled shoe: the best play
// for every hand against every dealer's card, and what the game pays back
// played that way.
func NewOdds() *Odds {
	o := &Odds{}
	for up := 1; up <= 10; up++ {
		o.solvers[up] = newSolver(up, 1)
	}

	// Every way the first two cards and the dealer's card can be dealt.
	sh := freshShoe()
	won := 0.0
	for a := 1; a <= 10; a++ {
		pa := float64(sh.left[a]) / float64(sh.n)
		sh.take(a)
		for b := 1; b <= 10; b++ {
			pb := float64(sh.left[b]) / float64(sh.n)
			sh.take(b)
			for up := 1; up <= 10; up++ {
				pu := float64(sh.left[up]) / float64(sh.n)
				sh.take(up)
				h := holding{}.add(a).add(b)
				worth := 0.0
				if h.total() == 21 {
					// A blackjack pays 3 to 2 unless the dealer has one too.
					worth = 1.5 * (1 - blackjackChance(up, &sh))
				} else {
					worth = o.solvers[up].advise(h, &sh).Value()
				}
				won += pa * pb * pu * worth
				sh.put(up)
			}
			sh.put(b)
		}
		sh.put(a)
	}
	o.ret = 1 + won
	return o
}

// Return is the fraction of what is first bet that is paid back, on a hand
// from a freshly shuffled shoe, to a player who makes the best play and
// never splits. As the shoe runs down, what a hand pays back moves with
// the cards left in it.
func (o *Odds) Return() float64 {
	return o.ret
}

// Advise weighs up a hand from a freshly shuffled shoe that has not been
// split, against the dealer's card, without looking at whether it could be
// split. The hand must have at least two cards and be short of 21.
func (o *Odds) Advise(up cards.Card, hand []cards.Card) Advice {
	sh := freshShoe()
	sh.take(Value(up))
	var h holding
	for _, c := range hand {
		h = h.add(Value(c))
		sh.take(Value(c))
	}
	return o.solvers[Value(up)].advise(h, &sh)
}

// shoe counts what is left in the machine's shoe by value. The cards on
// the table have already come out of it.
func (m *Machine) shoe() shoe {
	var s shoe
	for c, k := range m.Shoe {
		s.left[Value(cards.Card(c))] += k
		s.n += k
	}
	return s
}

// splitWorth is what splitting two cards of the given value is worth
// against the dealer's card, in bets, with the given cards left in the
// shoe: each hand weighed up by itself over the card it is dealt next, as
// if the other were to draw no more.
func splitWorth(v, up int, sh shoe) float64 {
	s := newSolver(up, 0.5)
	h := holding{}.add(v)

	total := 0.0
	for x := 1; x <= 10; x++ {
		k := sh.left[x]
		if k == 0 {
			continue
		}
		p := float64(k) / float64(sh.n)
		next := h.add(x)
		sh.take(x)
		var worth float64
		if v == 1 || next.total() == 21 {
			// Split aces get the one card, and 21 stands.
			worth = s.stand(next, &sh, 1)
		} else {
			worth = s.advise(next, &sh).Value()
		}
		sh.put(x)
		total += p * worth
	}
	return 2 * total
}

// BestPlay weighs up the hand being played on a machine from the cards
// actually left in its shoe, counting only the choices the machine allows:
// a double or a split the bank cannot cover is left out. It reports false
// if no hand is being played. It takes a moment, as it starts afresh from
// the shoe as it stands.
func BestPlay(m *Machine) (Advice, bool) {
	h := m.hand()
	if h == nil {
		return Advice{}, false
	}
	up := Value(m.Dealer[0])
	sh := m.shoe()
	var hold holding
	for _, c := range h.Cards {
		hold = hold.add(Value(c))
	}

	// A dealer's blackjack costs the one bet first made: all of it on a
	// hand that has not been split, and half on each of two that have.
	lose := 1.0
	if len(m.Hands) == 2 {
		lose = 0.5
	}
	a := newSolver(up, lose).advise(hold, &sh)
	a.CanDouble = m.CanDouble()
	if m.CanSplit() {
		a.CanSplit = true
		a.Split = splitWorth(Value(h.Cards[0]), up, sh)
	}
	a.choose()
	return a, true
}
