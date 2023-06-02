package image

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"

	"github.com/disintegration/imaging"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	tcolor "unstable.build/go-tui/term/color"
)

// DefaultDensityCharacters are the default characters used by Encode and EncodeScaler
// as density denotation characters. See EncodeScalerCharacters for more details.
const (
	DefaultDensityCharacters   = "   `'_.,-*=+:;cba!?0123456789$W#@"
	AlternateDensityCharacters = "    .:░▒▓█"
)

// DefaultScaler is the default draw.Scaler used by EncodeScaler, EncodeColor and Encode.
func DefaultScaler() draw.Scaler {
	return draw.NearestNeighbor
}

// DefaultConfig returns the default sane configuration for Encode.
func DefaultConfig() Config {
	return Config{
		Scaler:            draw.NearestNeighbor,
		DensityCharacters: DefaultDensityCharacters,
		Color:             false,
		AdjustContrast:    0,
	}
}

// Config is used to configure Encode.
type Config struct {
	// DensityCharacter is used to encode the density of a pixel. It is
	// assumed that it's sorted by density in ascending order.
	DensityCharacters string
	// Whether the final ASCII image should be encoded in color or not.
	Color  bool
	Scaler draw.Scaler

	// AdjustContrast adjusts the contrast of the image.
	// It ranges from -100 (decrease contrst by 100% to
	// 100 (increase contrast by 100%).
	AdjustContrast float64
}

// Encode takes an image.Image and encodes it in ASCII representation
// into the given cell.Buffer. The arguments outputHeight and outputWidth
// are used to determined the desired output height and width in cells.
// The argument config provides the density characters, a scaler and whether
// the final image should be encoded in color.
// assumed that it's sorted by density in ascending order.
func Encode(
	output *cell.Buffer, outputWidth, outputHeight int,
	src image.Image, config Config,
) {
	density := []rune(config.DensityCharacters)
	rect := image.Rect(0, 0, outputWidth, outputHeight)
	dst := image.NewNRGBA(rect)
	config.Scaler.Scale(dst, rect, src, src.Bounds(), draw.Over, nil)

	if config.AdjustContrast != 0 {
		dst = imaging.AdjustContrast(dst, config.AdjustContrast)
	}

	output.Reset()
	for y := 0; y < outputHeight; y++ {
		for x := 0; x < outputWidth; x++ {
			c := dst.At(x, y)
			rgba := c.(color.NRGBA)
			r, g, b := rgba.R, rgba.G, rgba.B
			avg := (float64(r) + float64(g) + float64(b)) / 3.0
			idx := int(math.Floor(mapValue(avg, 0, 255, 0, float64(len(density)-1))))
			character := density[idx]
			var attr term.Attributes
			if config.Color {
				fg := tcolor.RGBToAttribute(r, g, b)
				attr = term.Attributes{Fg: fg}
			}
			output.InsertWithAttr(term.Coordinates{X: x, Y: y}, character, attr)
		}
	}

}

func mapValue(value, inMin, inMax, outMin, outMax float64) float64 {
	return (value-inMin)*(outMax-outMin)/(inMax-inMin) + outMin
}
