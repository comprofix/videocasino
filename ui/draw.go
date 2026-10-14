// Package ui holds what the main menu and every game share on screen and in
// sound: the palette, text, shapes, buttons, playing cards and sound effects.
package ui

import (
	"bytes"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gobold"
)

// Everything is drawn to a fixed 800x600 screen and Ebitengine scales it to
// the window.
const (
	ScreenW = 800
	ScreenH = 600
	Border  = 15
)

// The 16-colour palette of the old PC screens.
var (
	Black        = color.RGBA{0, 0, 0, 255}
	Blue         = color.RGBA{0, 0, 170, 255}
	Green        = color.RGBA{0, 170, 0, 255}
	Red          = color.RGBA{170, 0, 0, 255}
	Magenta      = color.RGBA{170, 0, 170, 255}
	Brown        = color.RGBA{170, 85, 0, 255}
	LightGray    = color.RGBA{170, 170, 170, 255}
	DarkGray     = color.RGBA{85, 85, 85, 255}
	LightBlue    = color.RGBA{85, 85, 255, 255}
	LightCyan    = color.RGBA{85, 255, 255, 255}
	LightGreen   = color.RGBA{85, 255, 85, 255}
	LightRed     = color.RGBA{255, 85, 85, 255}
	LightMagenta = color.RGBA{255, 85, 255, 255}
	Yellow       = color.RGBA{255, 255, 85, 255}
	White        = color.RGBA{255, 255, 255, 255}
)

// Gold is the colour of a coin, which the palette has no good match for.
var Gold = color.RGBA{255, 170, 0, 255}

var fontSource *text.GoTextFaceSource

func init() {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF))
	if err != nil {
		log.Fatal(err)
	}
	fontSource = src
}

const (
	AlignStart  = text.AlignStart
	AlignCenter = text.AlignCenter
	AlignEnd    = text.AlignEnd
)

// DrawText draws s with (x, y) placed according to the two alignments:
// h along the line, v down the page.
func DrawText(dst *ebiten.Image, s string, x, y, size float64, c color.Color, h, v text.Align) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	op.PrimaryAlign = h
	op.SecondaryAlign = v
	text.Draw(dst, s, &text.GoTextFace{Source: fontSource, Size: size}, op)
}

// TextWidth returns how wide s is when drawn at the given size.
func TextWidth(s string, size float64) float64 {
	w, _ := text.Measure(s, &text.GoTextFace{Source: fontSource, Size: size}, 0)
	return w
}

func FillRect(dst *ebiten.Image, x, y, w, h float64, c color.Color) {
	vector.FillRect(dst, float32(x), float32(y), float32(w), float32(h), c, false)
}

func FrameRect(dst *ebiten.Image, x, y, w, h, width float64, c color.Color) {
	vector.StrokeRect(dst, float32(x), float32(y), float32(w), float32(h), float32(width), c, false)
}

func FillCircle(dst *ebiten.Image, cx, cy, r float64, c color.Color) {
	vector.FillCircle(dst, float32(cx), float32(cy), float32(r), c, true)
}

// FrameCircle draws the outline of a circle.
func FrameCircle(dst *ebiten.Image, cx, cy, r, width float64, c color.Color) {
	vector.StrokeCircle(dst, float32(cx), float32(cy), float32(r), float32(width), c, true)
}

func DrawLine(dst *ebiten.Image, x0, y0, x1, y1, width float64, c color.Color) {
	vector.StrokeLine(dst, float32(x0), float32(y0), float32(x1), float32(y1), float32(width), c, true)
}

func FillPath(dst *ebiten.Image, p *vector.Path, c color.Color) {
	op := &vector.DrawPathOptions{AntiAlias: true}
	op.ColorScale.ScaleWithColor(c)
	vector.FillPath(dst, p, nil, op)
}

// FillPoly fills the polygon whose corners are given as x, y pairs.
func FillPoly(dst *ebiten.Image, c color.Color, pts ...float64) {
	var p vector.Path
	p.MoveTo(float32(pts[0]), float32(pts[1]))
	for i := 2; i < len(pts); i += 2 {
		p.LineTo(float32(pts[i]), float32(pts[i+1]))
	}
	p.Close()
	FillPath(dst, &p, c)
}

// FillRoundRect fills a rectangle whose corners are rounded off with the
// given radius.
func FillRoundRect(dst *ebiten.Image, x, y, w, h, radius float64, c color.Color) {
	x0, y0, x1, y1, r := float32(x), float32(y), float32(x+w), float32(y+h), float32(radius)
	var p vector.Path
	p.MoveTo(x0+r, y0)
	p.LineTo(x1-r, y0)
	p.ArcTo(x1, y0, x1, y0+r, r)
	p.LineTo(x1, y1-r)
	p.ArcTo(x1, y1, x1-r, y1, r)
	p.LineTo(x0+r, y1)
	p.ArcTo(x0, y1, x0, y1-r, r)
	p.LineTo(x0, y0+r)
	p.ArcTo(x0, y0, x0+r, y0, r)
	p.Close()
	FillPath(dst, &p, c)
}

// DrawBorder draws the frame that runs round the edge of the screen.
func DrawBorder(dst *ebiten.Image, c color.Color) {
	FillRect(dst, 0, 0, ScreenW, Border, c)
	FillRect(dst, 0, ScreenH-Border, ScreenW, Border, c)
	FillRect(dst, 0, 0, Border, ScreenH, c)
	FillRect(dst, ScreenW-Border, 0, Border, ScreenH, c)
}
