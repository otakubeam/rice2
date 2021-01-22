// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package opentype_test

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func ExampleNewFace() {
	const (
		width        = 72
		height       = 36
		startingDotX = 6
		startingDotY = 28
	)

	f, err := opentype.Parse(goitalic.TTF)
	if err != nil {
		log.Fatalf("Parse: %v", err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    32,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		log.Fatalf("NewFace: %v", err)
	}

	dst := image.NewGray(image.Rect(0, 0, width, height))
	d := font.Drawer{
		Dst:  dst,
		Src:  image.White,
		Face: face,
		Dot:  fixed.P(startingDotX, startingDotY),
	}
	fmt.Printf("The dot is at %v\n", d.Dot)
	d.DrawString("jel")
	fmt.Printf("The dot is at %v\n", d.Dot)
	d.Src = image.NewUniform(color.Gray{0x7F})
	d.DrawString("ly")
	fmt.Printf("The dot is at %v\n", d.Dot)

	const asciiArt = ".++8"
	buf := make([]byte, 0, height*(width+1))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := asciiArt[dst.GrayAt(x, y).Y>>6]
			if c != '.' {
				// No-op.
			} else if x == startingDotX-1 {
				c = ']'
			} else if y == startingDotY-1 {
				c = '_'
			}
			buf = append(buf, c)
		}
		buf = append(buf, '\n')
	}
	os.Stdout.Write(buf)

	// Output:
	// The dot is at {6:00 28:00}
	// The dot is at {41:32 28:00}
	// The dot is at {66:48 28:00}
	// .....]..................................................................
	// .....]..................................................................
	// .....]..................................................................
	// .....]..................................+++......+++....................
	// .....]........+++.......................888......+++....................
	// .....].......+88+......................+88+......+++....................
	// .....].......888+......................+88+.....+++.....................
	// .....].......888+......................+88+.....+++.....................
	// .....].................................888......+++.....................
	// .....].................................888......+++.....................
	// .....]....................++..........+88+......+++.....................
	// .....]......+88+.......+888888+.......+88+.....+++....+++..........++...
	// .....]......888......+888888888+......+88+.....+++....++++........+++...
	// .....]......888.....+888+...+888......888......+++.....+++........++....
	// .....].....+888....+888......+88+.....888......+++.....+++.......+++....
	// .....].....+88+....888.......+88+....+88+......+++.....+++......+++.....
	// .....].....+88+...+888.......+88+....+88+.....+++......+++......+++.....
	// .....].....888....888+++++++++88+....+88+.....+++......+++.....+++......
	// .....].....888....88888888888888+....888......+++......++++....++.......
	// .....]....+888...+88888888888888.....888......+++.......+++...+++.......
	// .....]....+88+...+888...............+888......+++.......+++..+++........
	// .....]....+88+...+888...............+88+.....+++........+++..+++........
	// .....]....888....+888...............+88+.....+++........+++.+++.........
	// .....]....888....+888...............888......+++........++++++..........
	// .....]...+888.....888+..............888......+++........++++++..........
	// .....]...+88+.....+8888+....++8.....888+.....++++........++++...........
	// .....]...+88+......+8888888888+.....+8888....+++++.......++++...........
	// _____]___888________+88888888++______+888_____++++_______+++____________
	// .....]...888...........+++.............++................+++............
	// .....]..+88+............................................+++.............
	// .....]..+88+...........................................+++..............
	// .....].+888............................................+++..............
	// ....888888............................................+++...............
	// ....88888............................................++++...............
	// ....+++.................................................................
	// .....]..................................................................
}
