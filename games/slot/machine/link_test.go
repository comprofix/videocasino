package machine

import (
	"math"
	"testing"

	"videocasino/bank"
)

// playLink plays a link feature to its end and returns the last respin.
func playLink(t *testing.T, m *Machine) Respin {
	t.Helper()
	for {
		r, ok := m.Respin()
		if !ok {
			t.Fatal("Respin refused with a link feature in progress")
		}
		if r.Done {
			return r
		}
	}
}

// startLink puts a machine into a link feature holding the first n cells.
func startLink(m *Machine, n, bet int) {
	l := &Feature{Bet: bet, Respins: LinkRespins}
	for i := range n {
		l.Held[i/Rows][i%Rows] = true
		l.Chips[i/Rows][i%Rows] = Chip{Bets: 1}
	}
	m.Link = l
}

// linkEnd works out how link features end. Check it against features
// played from each starting position.
func TestLinkEnd(t *testing.T) {
	const features = 300_000
	m := New(bank.New(), 5, 6)
	for _, start := range []int{6, 9, 12} {
		for _, bet := range []int{25, MaxBet} {
			chips, full := 0, 0
			for range features {
				startLink(m, start, bet)
				end := playLink(t, m)
				chips += end.Link.Count()
				if end.GrandWin > 0 {
					full++
				}
			}
			wantChips, wantFull := linkEnd(start, LinkRespins, grandChance(bet), map[[2]int][2]float64{})
			gotChips, gotFull := float64(chips)/features, float64(full)/features
			if math.Abs(gotChips-wantChips) > 0.03 {
				t.Errorf("from %d chips at bet %d: ended with %.3f on average, want %.3f", start, bet, gotChips, wantChips)
			}
			// Allow for how few full grids a run of this length sees.
			if slack := 5*math.Sqrt(wantFull/features) + 1e-5; math.Abs(gotFull-wantFull) > slack {
				t.Errorf("from %d chips at bet %d: filled the grid %.6f of the time, want %.6f", start, bet, gotFull, wantFull)
			}
		}
	}
}

func TestLinkPays(t *testing.T) {
	m := New(bank.New(), 1, 2)
	m.Bank.Credits = 0
	const bet = 50

	// With one respin left, play until a feature ends without a new chip,
	// then check what it paid.
	for {
		startLink(m, 14, bet)
		m.Link.Respins = 1
		m.Link.Chips[0][0] = Chip{Jackpot: Mini}
		m.Link.Chips[0][1] = Chip{Jackpot: Minor}
		m.Link.Chips[0][2] = Chip{Jackpot: Major}
		m.Link.Chips[1][0] = Chip{Bets: 10}
		major := m.Major()
		credits := m.Bank.Credits

		end, _ := m.Respin()
		if !end.Done || end.GrandWin > 0 {
			continue // the last cell filled; try again
		}
		want := MiniBets*bet + MinorBets*bet + major + 10*bet + 10*bet
		if end.Win != want || end.MajorWin != major || m.Bank.Credits != credits+want {
			t.Errorf("feature paid %d (major %d), want %d (major %d)", end.Win, end.MajorWin, want, major)
		}
		if m.Major() != MajorStart {
			t.Errorf("major pot is %d after being won, want %d", m.Major(), MajorStart)
		}
		break
	}

	// A full grid pays the grand pot as well, and the pot starts again.
	startLink(m, Cells, bet)
	m.grandPot += 12345 * potUnit
	grand := m.Grand()
	end, _ := m.Respin()
	if !end.Done || end.GrandWin != grand || end.Win != Cells*bet+grand {
		t.Errorf("full grid paid %d (grand %d), want %d (grand %d)", end.Win, end.GrandWin, Cells*bet+grand, grand)
	}
	if m.Grand() != GrandStart {
		t.Errorf("grand pot is %d after being won, want %d", m.Grand(), GrandStart)
	}
	if _, ok := m.Respin(); ok {
		t.Error("Respin succeeded with no feature in progress")
	}
}

// A landed chip resets the respins; a respin with none uses one up.
func TestLinkRespins(t *testing.T) {
	m := New(bank.New(), 7, 8)
	startLink(m, 6, 25)
	for m.Link != nil {
		before := m.Link.Respins
		r, _ := m.Respin()
		landed := 0
		for _, col := range r.New {
			for _, n := range col {
				if n {
					landed++
				}
			}
		}
		want := before - 1
		if landed > 0 {
			want = LinkRespins
		}
		if r.Link.Respins != want {
			t.Fatalf("%d chips landed with %d respins left: now %d, want %d", landed, before, r.Link.Respins, want)
		}
	}
}

// The major pot pays the same at any bet, so a major chip should be dealt
// in proportion to the bet.
func TestMajorOdds(t *testing.T) {
	const chips = 20_000_000
	m := New(bank.New(), 9, 10)
	majors := 0
	for range chips {
		if m.dealChip(MaxBet, true, chipBets).Jackpot == Major {
			majors++
		}
	}
	if got := float64(majors) / chips; math.Abs(got-majorOdds) > majorOdds/10 {
		t.Errorf("a chip is a major %v of the time at the largest bet, want about %v", got, majorOdds)
	}
	for range chips / 20 {
		if m.dealChip(MaxBet, false, chipBets).Jackpot == Major {
			t.Fatal("dealt a second major chip to a feature")
		}
	}
}

// The two pots must stay within reach at every bet that can be played: at
// least once in a hundred million spins.
func TestRarestPrizes(t *testing.T) {
	const rarest = 1e-8
	for _, lines := range LineChoices {
		for _, perLine := range BetChoices {
			bet := lines * perLine
			r := ReturnAt(bet)
			grand := r.Grand * float64(bet) / GrandStart
			major := r.Major * float64(bet) / MajorStart
			if grand < rarest || major < rarest {
				t.Errorf("at a bet of %d the grand is won 1 spin in %.0f and the major 1 in %.0f", bet, 1/grand, 1/major)
			}
		}
	}
	small, large := ReturnAt(1), ReturnAt(MaxBet)
	t.Logf("grand 1 spin in %.0f at a bet of 1 and 1 in %.0f at %d",
		GrandStart/small.Grand, GrandStart/(large.Grand*float64(MaxBet)), MaxBet)
}
