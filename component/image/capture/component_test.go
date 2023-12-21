package capture

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	timage "unstable.build/go-tui/component/image"
	"unstable.build/go-tui/term"
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
	cfg := timage.DefaultConfig()
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

	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
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
