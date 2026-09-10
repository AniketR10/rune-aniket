// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package capture

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"go.uber.org/goleak"
	"unstable.build/rune/internal/component/asciiart"
)

func TestComponent(t *testing.T) {
	ch := make(chan struct{})

	trackID, streamID := "1", "s:1"
	fps := 1
	interrupter := term.FuncInterrupter(func(context.Context) error {
		ch <- struct{}{}
		return nil
	})
	reader := testVideoReader{}
	cfg := asciiart.DefaultConfig()
	w := term.NewStringWriter(8, 4)

	expected := "1111    \n1111    \n    @@@@\n    @@@@"

	// sut
	c := NewComponent(trackID, streamID, interrupter, fps, reader, cfg)
	c.Resize(8, 4)

	// first one's resize might not have been processed yet
	<-ch
	<-ch
	c.Draw(w)

	require.NoError(t, w.Flush())
	assert.Equal(t, expected, w.String())

	require.NoError(t, c.Close())
	goleak.VerifyNone(t)
}

type testVideoReader struct {
}

func (t testVideoReader) Read() (image.Image, func(), error) {
	width := 80
	height := 40

	upLeft := image.Point{0, 0}
	lowRight := image.Point{width, height}

	img := image.NewRGBA(image.Rectangle{upLeft, lowRight})

	cyan := color.RGBA{100, 200, 200, 0xff}

	for x := range width {
		for y := range height {
			switch {
			case x < width/2 && y < height/2:
				img.Set(x, y, cyan)
			case x >= width/2 && y >= height/2:
				img.Set(x, y, color.White)
			default:
			}
		}
	}
	return img, func() {}, nil
}
