package ui

import (
	"math"
	"math/rand/v2"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

const sampleRate = 44100

// Sounds plays the effects, and holds the ones every game shares. They are
// generated at start-up so the program needs no audio files. A game makes
// any others it wants with Tone.
type Sounds struct {
	ctx *audio.Context
	On  bool

	Click, Coin, Card, Win, BigWin, Bye []byte
}

func NewSounds() *Sounds {
	s := &Sounds{ctx: audio.NewContext(sampleRate), On: true}

	s.Click = Tone(660, 30, 0.15, true)
	s.Coin = append(Tone(1319, 50, 0.2, true), Tone(1976, 160, 0.2, true)...)
	s.Card = Tone(165, 45, 0.3, true)

	for _, f := range []float64{523, 659, 784} {
		s.Win = append(s.Win, Tone(f, 110, 0.25, true)...)
	}
	for _, f := range []float64{523, 659, 784, 1047, 784, 1047, 1319, 1047, 1319, 1568} {
		s.BigWin = append(s.BigWin, Tone(f, 110, 0.25, true)...)
	}

	// The original slot machine said goodbye with a burst of random
	// PC-speaker beeps.
	for range 60 {
		s.Bye = append(s.Bye, Tone(200+rand.Float64()*4000, 20, 0.12, false)...)
	}
	return s
}

func (s *Sounds) Play(pcm []byte) {
	if s.On {
		s.ctx.NewPlayerFromBytes(pcm).Play()
	}
}

// Tone returns a square wave as 16-bit stereo PCM. With fade set the note
// dies away over its length; otherwise it holds its volume.
func Tone(freq float64, ms int, volume float64, fade bool) []byte {
	n := sampleRate * ms / 1000
	pcm := make([]byte, 0, n*4)
	for i := range n {
		v := volume
		if math.Sin(2*math.Pi*freq*float64(i)/sampleRate) < 0 {
			v = -volume
		}
		if fade {
			v *= 1 - float64(i)/float64(n)
		}
		sample := int16(v * math.MaxInt16)
		lo, hi := byte(sample), byte(sample>>8)
		pcm = append(pcm, lo, hi, lo, hi)
	}
	return pcm
}
