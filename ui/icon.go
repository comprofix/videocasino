package ui

import (
	"image"
	"image/color"

	"videocasino/palette"
)

// The window's icon is an old one-armed bandit: a red cabinet trimmed in
// gold, three reels showing sevens, a payout tray and a lever with a red
// ball. It is drawn from shapes, as the rest of the art is, on a grid of
// 64 by 64 and scaled to each size.

// WindowIconSizes are the sizes the window's icon is given in; the system
// picks the best one for each place it is shown. The icon on the Windows
// exe adds larger ones, drawn by cmd/winicon.
var WindowIconSizes = []int{16, 32, 48, 64}

// iconDetail is the smallest size the sevens, the tray and the trim are
// drawn at. Below it they would only be a blur.
const iconDetail = 32

var (
	iconRed     = palette.Crimson
	iconDarkRed = palette.Maroon
	iconGold    = palette.Amber
	iconSilver  = palette.Mist
	iconBall    = palette.Red
	iconShine   = palette.Salmon
	iconWindow  = palette.Ink
	iconReel    = palette.White
	iconSeven   = palette.Crimson
)

// iconShape is one shape of the icon: whether a point of the grid is in it,
// and its colour. Later shapes are drawn over earlier ones.
type iconShape struct {
	in func(x, y float64) bool
	c  color.RGBA
}

// WindowIcons draws the icon at each of the window's sizes.
func WindowIcons() []image.Image {
	var icons []image.Image
	for _, size := range WindowIconSizes {
		icons = append(icons, Icon(size))
	}
	return icons
}

// Icon draws the icon at one size. Each pixel is sampled 8 by 8 times
// and the samples averaged, which smooths the edges.
func Icon(size int) *image.RGBA {
	shapes := iconShapes(size >= iconDetail)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	const n = 8
	step := 64 / float64(size*n)
	for py := range size {
		for px := range size {
			var r, g, b, a float64
			for sy := range n {
				for sx := range n {
					x := (float64(px*n+sx) + 0.5) * step
					y := (float64(py*n+sy) + 0.5) * step
					c, ok := iconAt(shapes, x, y)
					if !ok {
						continue
					}
					r += float64(c.R)
					g += float64(c.G)
					b += float64(c.B)
					a += 255
				}
			}
			if a == 0 {
				continue
			}
			// The colour is premultiplied by how much of the pixel is
			// covered, as image.RGBA holds it.
			img.SetRGBA(px, py, color.RGBA{
				R: uint8(r / (n * n)), G: uint8(g / (n * n)), B: uint8(b / (n * n)),
				A: uint8(a / (n * n)),
			})
		}
	}
	return img
}

// iconAt is the colour of the top shape at a point, if any is there.
func iconAt(shapes []iconShape, x, y float64) (color.RGBA, bool) {
	for i := len(shapes) - 1; i >= 0; i-- {
		if shapes[i].in(x, y) {
			return shapes[i].c, true
		}
	}
	return color.RGBA{}, false
}

// iconShapes lists the shapes of the icon from the back to the front. The
// sevens, the tray and the trim are left out when detail is false.
func iconShapes(detail bool) []iconShape {
	s := []iconShape{
		// The lever: the arm up the right side, joined to the cabinet.
		{iconRect(51, 12, 56, 40), iconSilver},
		{iconRect(46, 35, 56, 40), iconSilver},
		{iconCircle(53.5, 10, 6), iconBall},
		// The base and the cabinet, with a rounded top.
		{iconRect(4, 54, 50, 60), iconDarkRed},
	}
	if detail {
		s = append(s,
			iconShape{iconRoundRect(6, 8, 48, 56, 10), iconGold},
			iconShape{iconRoundRect(8, 10, 46, 54, 8), iconRed},
		)
	} else {
		s = append(s, iconShape{iconRoundRect(6, 8, 48, 56, 10), iconRed})
	}
	// The window, and a reel in each third of it.
	s = append(s, iconShape{iconRect(10, 20, 44, 42), iconWindow})
	for i := range 3 {
		x := 11.5 + float64(i)*10.5
		s = append(s, iconShape{iconRect(x, 21.5, x+10, 40.5), iconReel})
		if detail {
			s = append(s,
				iconShape{iconRect(x+2, 24, x+8.5, 27.5), iconSeven},
				iconShape{iconPoly(x+8.5, 27.5, x+5.5, 38.5, x+2.5, 38.5, x+5, 27.5), iconSeven},
			)
		}
	}
	// The payout tray, and a shine on the ball.
	if detail {
		s = append(s, iconShape{iconRect(16, 46, 38, 50), iconWindow})
		s = append(s, iconShape{iconCircle(51.5, 8, 2), iconShine})
	}
	return s
}

func iconRect(x0, y0, x1, y1 float64) func(x, y float64) bool {
	return func(x, y float64) bool {
		return x >= x0 && x < x1 && y >= y0 && y < y1
	}
}

func iconCircle(cx, cy, r float64) func(x, y float64) bool {
	return func(x, y float64) bool {
		return (x-cx)*(x-cx)+(y-cy)*(y-cy) < r*r
	}
}

// iconRoundRect is a rectangle with its top corners rounded to radius r.
func iconRoundRect(x0, y0, x1, y1, r float64) func(x, y float64) bool {
	left, right := iconCircle(x0+r, y0+r, r), iconCircle(x1-r, y0+r, r)
	return func(x, y float64) bool {
		switch {
		case x < x0 || x >= x1 || y < y0 || y >= y1:
			return false
		case y >= y0+r:
			return true
		case x < x0+r:
			return left(x, y)
		case x >= x1-r:
			return right(x, y)
		}
		return true
	}
}

// iconPoly is a convex shape with its corners given in turn, clockwise on
// screen.
func iconPoly(pts ...float64) func(x, y float64) bool {
	return func(x, y float64) bool {
		n := len(pts) / 2
		for i := range n {
			ax, ay := pts[2*i], pts[2*i+1]
			bx, by := pts[2*((i+1)%n)], pts[2*((i+1)%n)+1]
			if (bx-ax)*(y-ay)-(by-ay)*(x-ax) < 0 {
				return false
			}
		}
		return true
	}
}
