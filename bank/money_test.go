package bank

import "testing"

func TestMoney(t *testing.T) {
	for _, c := range []struct {
		credits        int
		dollars, money string
	}{
		{0, "$0.00", "$0"},
		{5, "$0.05", "$0.05"},
		{25, "$0.25", "$0.25"},
		{100, "$1.00", "$1"},
		{750, "$7.50", "$7.50"},
		{2500, "$25.00", "$25"},
		{100_000, "$1,000.00", "$1,000"},
		{123_456_789, "$1,234,567.89", "$1,234,567.89"},
		{-750, "-$7.50", "-$7.50"},
		{-100_000, "-$1,000.00", "-$1,000"},
	} {
		if got := Dollars(c.credits); got != c.dollars {
			t.Errorf("%d credits are written %q, want %q", c.credits, got, c.dollars)
		}
		if got := Money(c.credits); got != c.money {
			t.Errorf("%d credits are written short %q, want %q", c.credits, got, c.money)
		}
	}
}

func TestWorth(t *testing.T) {
	for credits, want := range map[float64]string{
		0: "+$0.00", 123.4: "+$1.23", 123.6: "+$1.24", -50: "-$0.50", -1234.5: "-$12.35",
	} {
		if got := Worth(credits); got != want {
			t.Errorf("%v credits are worth %q, want %q", credits, got, want)
		}
	}
}
