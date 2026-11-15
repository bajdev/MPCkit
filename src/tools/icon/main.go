// Command icon draws the MPCkit app icon: a 4x4 pad grid on the app's dark
// background, with pad 1 (bottom left, as on the device) lit in the accent
// colour. It writes a 1024x1024 PNG; `wails3 generate icons` makes the
// Windows and macOS icons from it.
//
//	go run ./tools/icon build/appicon.png
//
// With -dmg it writes the background of the macOS disk image instead: a
// plain light 540x380 image (Finder labels the icons with dark text).
//
//	go run ./tools/icon -dmg build/darwin/dmg-background.png
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const (
	size = 1024
	ss   = 4 // supersampling per axis, for smooth edges
)

// Colours from the app (frontend/src/app.css).
var (
	bg      = rgb(0x15, 0x17, 0x1A)
	edge    = rgb(0x35, 0x3A, 0x43)
	pad     = rgb(0x33, 0x38, 0x42)
	padEdge = rgb(0x5A, 0x61, 0x6E)
	accent  = rgb(0xF5, 0xA5, 0x24)
)

func rgb(r, g, b uint8) color.NRGBA { return color.NRGBA{r, g, b, 0xFF} }

type rect struct{ x0, y0, x1, y1, r float64 }

// inside reports whether (x, y) is inside the rounded rectangle.
func (q rect) inside(x, y float64) bool {
	if x < q.x0 || x >= q.x1 || y < q.y0 || y >= q.y1 {
		return false
	}
	cx := math.Max(q.x0+q.r, math.Min(x, q.x1-q.r))
	cy := math.Max(q.y0+q.r, math.Min(y, q.y1-q.r))
	return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= q.r*q.r
}

func (q rect) inset(d float64) rect {
	return rect{q.x0 + d, q.y0 + d, q.x1 - d, q.y1 - d, math.Max(q.r-d, 0)}
}

type layer struct {
	shape rect
	col   color.NRGBA
}

func main() {
	dmg := flag.Bool("dmg", false, "write the disk image background instead of the icon")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: icon [-dmg] out.png")
		os.Exit(2)
	}
	if *dmg {
		img := image.NewNRGBA(image.Rect(0, 0, 540, 380))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0xF2, 0xF3, 0xF5, 0xFF
		}
		write(flag.Arg(0), img)
		return
	}

	// Body: a rounded square with a margin, as macOS icons have; the
	// same image works for Windows and Linux.
	body := rect{64, 64, 960, 960, 200}
	layers := []layer{{body, edge}, {body.inset(10), bg}}

	// 4x4 pads, 140 px with 32 px gaps, centred in the body.
	const cell, gap = 140.0, 32.0
	start := (size - (4*cell + 3*gap)) / 2
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			x := start + float64(col)*(cell+gap)
			y := start + float64(row)*(cell+gap)
			r := rect{x, y, x + cell, y + cell, 28}
			if row == 3 && col == 0 { // pad 1: bottom left
				layers = append(layers, layer{r, accent})
				continue
			}
			layers = append(layers, layer{r, padEdge}, layer{r.inset(8), pad})
		}
	}

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := float64(px) + (float64(sx)+0.5)/ss
					y := float64(py) + (float64(sy)+0.5)/ss
					// Topmost layer that covers the point wins.
					for i := len(layers) - 1; i >= 0; i-- {
						if layers[i].shape.inside(x, y) {
							c := layers[i].col
							r += float64(c.R)
							g += float64(c.G)
							b += float64(c.B)
							a++
							break
						}
					}
				}
			}
			if a == 0 {
				continue // transparent outside the body
			}
			img.SetNRGBA(px, py, color.NRGBA{
				uint8(r/a + 0.5), uint8(g/a + 0.5), uint8(b/a + 0.5),
				uint8(255*a/(ss*ss) + 0.5),
			})
		}
	}

	write(flag.Arg(0), img)
}

func write(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := png.Encode(f, img); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
