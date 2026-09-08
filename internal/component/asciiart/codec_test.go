// Copyright (C) 2017-2026 Unstable Build, LLC
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

package asciiart

import (
	"image"
	_ "image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/rune/internal/cell"
)

func TestResizeMaintainAspectRatio(t *testing.T) {
	suite := []struct {
		description                              string
		srcWidth, srcHeight, dstWidth, dstHeight int
		expectedWidth, expectedHeight            int
	}{
		{"if src and dst are the same, just compensate for cell aspect ratio", 100, 100, 100, 100, 98, 43},
		{"if src and dst are the same, just compensate for cell aspect ratio, width > height", 80, 60, 80, 60, 79, 26},
		{"if src and dst are the same, just compensate for cell aspect ratio, height > width", 60, 80, 60, 80, 60, 35},
		{"down scale, out same ratio, width > height", 80, 60, 40, 30, 39, 13},
		{"down scale, out same ratio, height > width", 60, 80, 30, 40, 29, 17},
		{"up scale, out same ratio, width > height", 40, 30, 80, 60, 79, 26},
		{"up scale, out same ratio, height > width", 30, 40, 60, 80, 60, 35},
		{"down scale, out inverted ratio, width > height", 80, 60, 30, 40, 30, 10},
		{"down scale, out inverted ratio, height > width", 60, 80, 40, 30, 39, 23},
		{"up scale, out inverted ratio, width > height", 40, 30, 60, 80, 58, 19},
		{"up scale, out inverted ratio, height > width", 30, 40, 80, 60, 79, 46},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			actualWidth, actualHeight := ResizeMaintainAspectRatio(test.srcWidth, test.srcHeight,
				test.dstWidth, test.dstHeight)

			assert.Equal(t, test.expectedWidth, actualWidth, "width")
			assert.Equal(t, test.expectedHeight, actualHeight, "height")
		})
	}
}

func TestCodec(t *testing.T) {
	suite := []struct {
		description               string
		outputWidth, outputHeight int
		src                       image.Image
		config                    Config
		expectedOutput            string
	}{
		{"encode a non-colorized image, with default scaler and density characters",
			40, 20, loadImage("testdata/image_1.png"), DefaultConfig(),
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
			40, 20, loadImage("testdata/image_2.png"), DefaultConfig(),
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
			40, 20, loadImage("testdata/image_2.png"), defaultConfigColor(),
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
		{"encode a non-colorized image with transparent background, add contrast with default scaler and density characters",
			40, 20, loadImage("testdata/image_1.png"), defaultConfigContrast(),
			`
                                        
                                        
                                        
                                        
              @@@@@@@@@@@@@@@           
             @@@@     @@@@              
            @@@@       @@@@             
            @@@@@     @@@@@             
              @@@@@@@@@@@@              
             @@@@@@@@@@                 
            @@@@                        
             @@@@@@@@@@@@@              
             @@@@@@@@@@@@@@@            
           @@@@@        @@@@@           
            @@@@@     @@@@@@            
              @@@@@@@@@@@               
                                        
                                        
                                        
                                        `,
		},
		{"encode an image and maintain aspect ratio, inverted aspect ratio",
			30, 30, loadImage("testdata/image_1.png"), configMaintainAspectRatio(),
			`








                              
                              
            $@@@$  #@0        
          @@@.  #@@:          
         @@@     @@@          
          @@@@@@@@@           
         $@@                  
         1@@@@@@@@##          
         @@@      #@@         
         @@@#9+,9@@@#         
                              
                              
                              








                               `,
		},
		{"encode an image and maintain aspect ratio, similar aspect ratio",
			50, 20, loadImage("testdata/image_1.png"), configMaintainAspectRatio(),
			`
                                                
                                                
                                                
                                                
                  W@@@@@@@@@@#@@@@@             
                :@@@@#     @@@@#                
                @@@@@       @@@@#               
                @@@@@.     7@@@@$               
                 .@@@@@@@@@@@@@$                
                 2#@@@@@####,                   
                @@@@W                           
                $@@@@@@@@@@@@###'               
                 ##@@##@@@##@@@@@@b             
               @@@@#          #@@@@             
               W@@@@@9      #@@@@#              
                 ,##@@@@@@@@@@#.                
                                                
                                                
                                                
                                                
                                                   `,
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			output := cell.NewBuffer()
			require.NotZero(t, test.expectedOutput)
			require.True(t, len(test.expectedOutput) > 0)
			// trim first line so its easier to describe tests
			expectedOutput := test.expectedOutput[1:]

			// sut
			Encode(output, test.outputWidth,
				test.outputHeight, test.src, test.config)
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

func defaultConfigColor() Config {
	ret := DefaultConfig()
	ret.Color = true
	return ret
}

func defaultConfigContrast() Config {
	ret := DefaultConfig()
	ret.AdjustContrast = 100
	return ret
}

func configMaintainAspectRatio() Config {
	ret := DefaultConfig()
	ret.MaintainAspectRatio = true
	return ret
}
