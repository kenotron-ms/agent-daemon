//go:build cgo

package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// makeIcon generates a 22×22 monochrome PNG suitable for a macOS template image.
//
// It draws a 45°-rotated loom plain-weave mark: 3 diagonal "/" warp threads
// crossed by 2 diagonal "\" weft threads, using the diagonal coordinate system:
//
//	d1 = x + y  (constant along "/" diagonals, ranges 0..42)
//	d2 = x - y  (constant along "\" diagonals, ranges -21..21)
//
// Warp threads (5px wide) centered at d1 = 11, 21, 31
// Weft threads (5px wide) centered at d2 = -5, +5
//
// Plain weave over-under (6 crossings, alternating):
//
//	W1∩We1 (3,8)   warp over weft
//	W1∩We2 (8,3)   weft over warp
//	W2∩We1 (8,13)  weft over warp
//	W2∩We2 (13,8)  warp over weft
//	W3∩We1 (13,18) warp over weft
//	W3∩We2 (18,13) weft over warp
//
// Over-under is indicated by 1px diagonal notch lines carved into the
// "under" thread at the boundary of each crossing.
func makeIcon() []byte {
	const size = 22
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	black := color.NRGBA{0, 0, 0, 255}
	clear := color.NRGBA{}

	// Pass 1: draw all threads.
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d1, d2 := x+y, x-y
			warp := (d1 >= 9 && d1 <= 13) || (d1 >= 19 && d1 <= 23) || (d1 >= 29 && d1 <= 33)
			weft := (d2 >= -7 && d2 <= -3) || (d2 >= 3 && d2 <= 7)
			if warp || weft {
				img.Set(x, y, black)
			}
		}
	}

	// Pass 2: carve 1px diagonal notch lines at crossing boundaries.
	//
	// "Weft UNDER warp" crossings → notch the weft band at d1 = warpEdge ± 1
	//   W1∩We1: notch at d1=8,14 within d2=[-7,-3]
	//   W2∩We2: notch at d1=18,24 within d2=[3,7]
	//   W3∩We1: notch at d1=28,34 within d2=[-7,-3]
	//
	// "Warp UNDER weft" crossings → notch the warp band at d2 = weftEdge ± 1
	//   W1∩We2: notch at d2=2,8   within d1=[9,13]
	//   W2∩We1: notch at d2=-8,-2 within d1=[19,23]
	//   W3∩We2: notch at d2=2,8   within d1=[29,33]
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d1, d2 := x+y, x-y
			// Weft-under notches
			if (d1 == 8 || d1 == 14) && d2 >= -7 && d2 <= -3 {
				img.Set(x, y, clear)
			}
			if (d1 == 18 || d1 == 24) && d2 >= 3 && d2 <= 7 {
				img.Set(x, y, clear)
			}
			if (d1 == 28 || d1 == 34) && d2 >= -7 && d2 <= -3 {
				img.Set(x, y, clear)
			}
			// Warp-under notches
			if (d2 == 2 || d2 == 8) && d1 >= 9 && d1 <= 13 {
				img.Set(x, y, clear)
			}
			if (d2 == -8 || d2 == -2) && d1 >= 19 && d1 <= 23 {
				img.Set(x, y, clear)
			}
			if (d2 == 2 || d2 == 8) && d1 >= 29 && d1 <= 33 {
				img.Set(x, y, clear)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
