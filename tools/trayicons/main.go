// Command trayicons renders the small cube mark independently of the detailed app icon.
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type point struct{ x, y float64 }
type segment struct{ a, b point }

var edges = []segment{
	{point{12, 2.5}, point{21, 7.5}}, {point{21, 7.5}, point{21, 16.5}},
	{point{21, 16.5}, point{12, 21.5}}, {point{12, 21.5}, point{3, 16.5}},
	{point{3, 16.5}, point{3, 7.5}}, {point{3, 7.5}, point{12, 2.5}},
	{point{3, 7.5}, point{12, 12.5}}, {point{12, 12.5}, point{21, 7.5}},
	{point{12, 12.5}, point{12, 21.5}},
}

func main() {
	for name, ink := range map[string]color.NRGBA{
		"tray-template.png": {0, 0, 0, 255},
		"tray.png":          {21, 156, 122, 255},
		"tray-dark.png":     {100, 211, 182, 255},
	} {
		if err := writeIcon(filepath.Join("internal", "desktop", "assets", name), ink); err != nil {
			panic(err)
		}
	}
}

func writeIcon(path string, ink color.NRGBA) error {
	const size, samples = 44, 4
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			coverage := 0
			for sy := range samples {
				for sx := range samples {
					p := point{(float64(x) + (float64(sx)+0.5)/samples) * 24 / size, (float64(y) + (float64(sy)+0.5)/samples) * 24 / size}
					for _, edge := range edges {
						if distance(p, edge) <= 0.95 {
							coverage++
							break
						}
					}
				}
			}
			c := ink
			c.A = uint8(coverage * 255 / (samples * samples))
			img.SetNRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return err
	}
	return f.Close()
}

func distance(p point, line segment) float64 {
	dx, dy := line.b.x-line.a.x, line.b.y-line.a.y
	t := max(0, min(1, ((p.x-line.a.x)*dx+(p.y-line.a.y)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(p.x-line.a.x-t*dx, p.y-line.a.y-t*dy)
}
