package bank

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCredits(t *testing.T) {
	b := New()
	if b.Credits != StartCredits {
		t.Fatalf("a new bank has %d credits, want %d", b.Credits, StartCredits)
	}

	if !b.Take(250) || b.Credits != StartCredits-250 {
		t.Errorf("after a bet of 250 there are %d credits, want %d", b.Credits, StartCredits-250)
	}
	b.Pay(1000)
	if b.Credits != StartCredits+750 {
		t.Errorf("after a win of 1000 there are %d credits, want %d", b.Credits, StartCredits+750)
	}

	// A bet the credits do not cover is refused and takes nothing.
	had := b.Credits
	if b.Take(had+1) || b.Credits != had {
		t.Errorf("a bet of %d was taken from %d credits, leaving %d", had+1, had, b.Credits)
	}
	if b.Take(-5) || b.Credits != had {
		t.Errorf("a bet of less than nothing was taken, leaving %d credits", b.Credits)
	}

	// The last credit can be bet.
	if !b.Take(had) || b.Credits != 0 {
		t.Errorf("betting everything left %d credits, want 0", b.Credits)
	}

	b.InsertCoin()
	if b.Credits != CoinCredits {
		t.Errorf("a coin gave %d credits, want %d", b.Credits, CoinCredits)
	}
	b.InsertCoin()
	b.Reset()
	if b.Credits != StartCredits {
		t.Errorf("a reset left %d credits, want %d", b.Credits, StartCredits)
	}
}

// hand stands in for what a game keeps.
type hand struct {
	Bet   int
	Cards [3]int
}

func TestSaveAndOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")

	// With no file yet the bank starts afresh.
	b := Open(path)
	if b.Credits != StartCredits || !b.Sound {
		t.Errorf("a bank with no file has %d credits and sound %v", b.Credits, b.Sound)
	}
	var none hand
	if b.Load("poker", &none) {
		t.Error("loaded a game before one was kept")
	}

	b.Take(1234)
	b.Sound = false
	want := hand{Bet: 25, Cards: [3]int{7, 8, 9}}
	if err := b.Keep("poker", want); err != nil {
		t.Fatal(err)
	}
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}

	b = Open(path)
	if b.Credits != StartCredits-1234 || b.Sound {
		t.Errorf("opened with %d credits and sound %v, want %d and false", b.Credits, b.Sound, StartCredits-1234)
	}
	var got hand
	if !b.Load("poker", &got) || got != want {
		t.Errorf("loaded %+v, want %+v", got, want)
	}

	// A second game is kept beside the first, and saving again replaces the
	// file and leaves nothing else behind.
	if err := b.Keep("slot", hand{Bet: 50}); err != nil {
		t.Fatal(err)
	}
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}
	b = Open(path)
	if !b.Load("poker", &got) || got != want {
		t.Errorf("loaded %+v after a second save, want %+v", got, want)
	}
	if !b.Load("slot", &got) || got.Bet != 50 {
		t.Errorf("loaded %+v for the second game, want a bet of 50", got)
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("%d files in the save folder, want 1", len(files))
	}
}

func TestOpenDamaged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// A damaged file is ignored rather than stopping the program.
	write("{not json")
	if b := Open(path); b.Credits != StartCredits || !b.Sound {
		t.Errorf("a damaged save gave %d credits and sound %v", b.Credits, b.Sound)
	}

	// Credits cannot be less than nothing.
	write(`{"Credits": -50, "Sound": true}`)
	if b := Open(path); b.Credits != 0 {
		t.Errorf("a save with -50 credits gave %d, want 0", b.Credits)
	}

	// What the file leaves out stays as it is in a new bank.
	write(`{}`)
	if b := Open(path); b.Credits != StartCredits || !b.Sound {
		t.Errorf("an empty save gave %d credits and sound %v", b.Credits, b.Sound)
	}

	// A game's section that cannot be read is that game's loss alone.
	write(`{"Credits": 300, "Sound": false, "Games": {"poker": "not a hand"}}`)
	b := Open(path)
	if b.Credits != 300 {
		t.Errorf("opened with %d credits, want 300", b.Credits)
	}
	var h hand
	if b.Load("poker", &h) {
		t.Error("loaded a game section that is not a game")
	}
}

func TestSaveNowhere(t *testing.T) {
	// A bank that is not kept anywhere still works.
	b := New()
	b.InsertCoin()
	if err := b.Save(); err != nil {
		t.Errorf("saving a bank with no file failed: %v", err)
	}
}
