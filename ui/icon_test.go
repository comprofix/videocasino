package ui

import "testing"

func TestIcon(t *testing.T) {
	for _, size := range []int{16, 32, 48, 64, 128, 256} {
		img := Icon(size)
		if b := img.Bounds(); b.Dx() != size || b.Dy() != size {
			t.Errorf("the icon at %d is %v", size, b)
		}
		// The corner above the cabinet is clear, and the middle of the
		// reels is not.
		if a := img.RGBAAt(0, 0).A; a != 0 {
			t.Errorf("the icon at %d has its top left corner filled", size)
		}
		if a := img.RGBAAt(size*27/64, size*31/64).A; a != 255 {
			t.Errorf("the icon at %d is not solid in the middle", size)
		}
	}
}
