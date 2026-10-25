package palette

import (
	"image/color"
	"testing"
)

func TestPalette(t *testing.T) {
	if len(All) != 64 {
		t.Fatalf("%d colours, want 64", len(All))
	}
	seen := map[color.RGBA]bool{}
	for i, c := range All {
		if c.A != 255 {
			t.Errorf("colour %d is not opaque", i)
		}
		if seen[c] {
			t.Errorf("colour %d, %v, is there twice", i, c)
		}
		seen[c] = true
	}
	// Spot checks against the published palette.
	for _, c := range []struct {
		got  color.RGBA
		want uint32
	}{{Ink, 0x2e222f}, {Gold, 0xf9c22b}, {Navy, 0x323353}, {White, 0xffffff}, {Blush, 0xfdcbb0}} {
		if want := (color.RGBA{uint8(c.want >> 16), uint8(c.want >> 8), uint8(c.want), 255}); c.got != want {
			t.Errorf("%v, want #%06x", c.got, c.want)
		}
	}
	if All[0] != Ink || All[63] != Blush {
		t.Error("All is not in the palette's order")
	}
}
