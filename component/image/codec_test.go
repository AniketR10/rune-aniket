// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package image

import (
	"image"
	_ "image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cell"
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
