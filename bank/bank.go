// Package bank holds what the games share and what is kept from one session
// to the next: the credits, the settings and the save file. It has no
// graphics so it can be tested on its own.
package bank

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	StartCredits = 10_000 // $100 at 100 credits to the dollar
	CoinCredits  = 10_000 // credits added by inserting a coin: $100
)

// Bank is the one balance every game plays from, with the settings that
// apply to them all. A game takes its bets from it and pays its wins into
// it, and holds no credits of its own.
type Bank struct {
	Credits int
	Sound   bool // the sound effects are on

	// What each game keeps for itself, under the game's name. The bank does
	// not look inside.
	games map[string]json.RawMessage

	path string // where the bank is kept; empty if it is not
}

// New returns a bank with the starting credits. It is not kept anywhere.
func New() *Bank {
	return &Bank{
		Credits: StartCredits,
		Sound:   true,
		games:   map[string]json.RawMessage{},
	}
}

// Take takes a bet out of the credits. It reports false and takes nothing
// if there are too few.
func (b *Bank) Take(n int) bool {
	if n < 0 || b.Credits < n {
		return false
	}
	b.Credits -= n
	return true
}

// Pay adds a win to the credits.
func (b *Bank) Pay(n int) {
	b.Credits += n
}

// InsertCoin adds credits, so play can go on after they run out.
func (b *Bank) InsertCoin() {
	b.Credits += CoinCredits
}

// Reset puts the credits back to the starting amount.
func (b *Bank) Reset() {
	b.Credits = StartCredits
}

// saved is the save file: the bank, then a section for each game.
type saved struct {
	Credits int
	Sound   bool
	Games   map[string]json.RawMessage `json:",omitempty"`
}

// Path is where the bank is kept: a file beside the program itself.
func Path() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "save.json"), nil
}

// Open returns the bank kept at path. If there is none, or it cannot be
// read, the bank starts afresh. Either way it is kept at path from then on.
func Open(path string) *Bank {
	b := New()
	b.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		return b
	}
	// Anything the file leaves out stays as it is in a new bank.
	s := saved{Credits: b.Credits, Sound: b.Sound}
	if err := json.Unmarshal(data, &s); err != nil {
		return b
	}
	// The file may have been damaged or edited, so credits that could not
	// have come from play are put right.
	b.Credits = max(s.Credits, 0)
	b.Sound = s.Sound
	if s.Games != nil {
		b.games = s.Games
	}
	return b
}

// Save writes the bank to its file. It writes to a second file and then
// renames it, so a crash part-way through cannot spoil the save there was.
// A bank that is not kept anywhere has nothing to do.
func (b *Bank) Save() error {
	if b.path == "" {
		return nil
	}
	s := saved{Credits: b.Credits, Sound: b.Sound, Games: b.games}
	data, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		return err
	}
	tmp := b.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, b.path)
}

// Load fills v with what the named game last kept. It reports false if the
// game has kept nothing, or if what it kept cannot be read: the game then
// starts afresh.
func (b *Bank) Load(game string, v any) bool {
	data, ok := b.games[game]
	if !ok {
		return false
	}
	return json.Unmarshal(data, v) == nil
}

// Keep sets what the named game keeps, to be written at the next Save.
func (b *Bank) Keep(game string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b.games[game] = data
	return nil
}
