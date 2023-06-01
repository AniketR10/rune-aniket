package image

import (
	"image"
	_ "image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/draw"
	"unstable.build/go-tui/cell"
)

func TestCodec(t *testing.T) {
	suite := []struct {
		description               string
		outputWidth, outputHeight int
		src                       image.Image
		scaler                    draw.Scaler
		characters                string
		colorize                  bool
		expectedOutput            string
	}{
		{"encode a non-colorized image, with default scaler and density characters",
			40, 20, loadImage("testdata/image_1.png"), DefaultScaler(), DefaultDensityCharacters, false,
			`
                                        
                                        
                                        
                                        
              #@@@@@@@@@#@@@W           
             @@@@+   c@@@#              
            #@@@      ,@@@#             
            8@@@#     #@@@$             
             .@@@@@@@@@@@#              
             2#@@#@#@#$                 
            #@@@                        
            =@@@@@@@@@@@##'             
             ##@##@@#@#@@@@@            
           6@@@3        8@@@7           
           '@@@@#=    ?@@@@W            
             ,#@@@@@@@@@#-              
                                        
                                        
                                        
                                        `,
		},
		{"encode a non-colorized image with transparent background, with default scaler and density characters",
			40, 20, loadImage("testdata/image_2.png"), DefaultScaler(), DefaultDensityCharacters, false,
			`
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@#abbbb@@@bbbcc6@@Wccccc@@@@@@@@@@
@@@@@@@4bbbbbb@@cccccc##ccc;;;2@@@@@@@@@
@@@@@@@#bbbbb#@bccccc5##c;;;;:@@@@@@@@@@
@@@@@@@@@@@@@#cc!@@@@@!;c#@@@@@@@@@@@@@@
@@@@@@@@#ccccc;@#1;;;;;@@#c;;@@@@@@@@@@@
@@@@@@@Wccccc;@@;;;;;;#@c;::::#@@@@@@@@@
@@@@@@@#cccccc@#;;;;;;##::::::#@@@@@@@@@
@@@@@@@@#:;;#@#;;;;;$##:::::a@@@@@@@@@@@
@@@@@@@@@@@##;;5@@@@#;:=@@@#@@@@@@@@@@@@
@@@@@@@#c;;;;;#@;:::::8@#::::+@@@@@@@@@@
@@@@@@@0;;;;::@@::::::##:+++++?@@@@@@@@@
@@@@@@@#;;:;:#@@:::::a@@#+++++@@@@@@@@@@
@@@@@@@@@####@@@@###@@@@@@#@#@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
#$$$W$$@9$$9@##8@##@6W#9@@@66@@99$9@##@@
@##@::+##@@++;W+cW#:@##=@@=@@=+?*+*==W@@
@##@########@###@##@#########@####@#@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@`,
		},
		{"does not panic on a colorized image", // no easy way to test colors here
			40, 20, loadImage("testdata/image_2.png"), DefaultScaler(), DefaultDensityCharacters, true,
			`
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
@@@@@@@#abbbb@@@bbbcc6@@Wccccc@@@@@@@@@@
@@@@@@@4bbbbbb@@cccccc##ccc;;;2@@@@@@@@@
@@@@@@@#bbbbb#@bccccc5##c;;;;:@@@@@@@@@@
@@@@@@@@@@@@@#cc!@@@@@!;c#@@@@@@@@@@@@@@
@@@@@@@@#ccccc;@#1;;;;;@@#c;;@@@@@@@@@@@
@@@@@@@Wccccc;@@;;;;;;#@c;::::#@@@@@@@@@
@@@@@@@#cccccc@#;;;;;;##::::::#@@@@@@@@@
@@@@@@@@#:;;#@#;;;;;$##:::::a@@@@@@@@@@@
@@@@@@@@@@@##;;5@@@@#;:=@@@#@@@@@@@@@@@@
@@@@@@@#c;;;;;#@;:::::8@#::::+@@@@@@@@@@
@@@@@@@0;;;;::@@::::::##:+++++?@@@@@@@@@
@@@@@@@#;;:;:#@@:::::a@@#+++++@@@@@@@@@@
@@@@@@@@@####@@@@###@@@@@@#@#@@@@@@@@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
#$$$W$$@9$$9@##8@##@6W#9@@@66@@99$9@##@@
@##@::+##@@++;W+cW#:@##=@@=@@=+?*+*==W@@
@##@########@###@##@#########@####@#@@@@
@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@`,
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			output := cell.NewBuffer()
			require.NotZero(t, test.expectedOutput)
			require.True(t, len(test.expectedOutput) > 0)
			// trim first line so its easier to describe tests
			expectedOutput := test.expectedOutput[1:]

			config := Config{
				Scaler:            test.scaler,
				Color:             test.colorize,
				DensityCharacters: test.characters,
			}

			// sut
			Encode(output, test.outputWidth, test.outputHeight, test.src, config)
			assert.Equal(t, expectedOutput, output.String())
		})
	}
}

func loadImage(filename string) image.Image {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}

	img, _, err := image.Decode(f)
	if err != nil {
		panic(err)
	}

	return img
}
