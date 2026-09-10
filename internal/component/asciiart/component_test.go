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

package asciiart

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/component/comptest"
	"github.com/unstablebuild/rune-go-sdk/term"
)

//go:embed test/logo.png
var logo []byte

func TestNewComponent(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(logo))
	if err != nil {
		panic(fmt.Errorf("png decode: %v", err))
	}

	o := NewComponent(img, DefaultConfig())

	w := term.NewStringWriter(60, 20)

	tests := []comptest.TestCase{
		{
			func() { o.Resize(30, 15) }, `
              6                                             
             22                                             
         -a   !00 11                                        
       +     0 .! 00ac                                      
       +0a= ??    !!bc                                      
       +?bb  !!;  a!cc                                      
       +!cc  .+;; c;;                                       
       =!;c  -b:;                                           
       =a:c  -c+: c++                                       
       =b;;  .;++ a;=+                                      
       =c:;  .==   ;:=.                                     
        ;:::      ::==                                      
         ++::::::::=*                                       
           ==******                                         
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            `,
		}, {
			func() { o.Resize(60, 20) }, `
                                                            
                           22                               
                           11112    4                       
                   aaac      ;0000  2111!                   
                          01   !!?  1000aaa                 
               1111a     ???        ????bbb                 
               0000aa    ?!!!!      ?!!!ccc                 
               ????bbb     !!!!ca   aaaaccc                 
               ?!!?bcb     ++ca;;;  aa;;;;                  
               !!!!ccb     ++aa;;;  ;;;                     
               !aaac;c     bbcb:::  :+:=                    
               aaaa;;c     cccc++:  cc+++++                 
               bbbb;;;     ;;;c+++  -;;;:+++                
               cccc:;;     ;;===+     ;;;====               
               ;;cc;;;     ====       ;;:====               
                ;;;:;:::            ::::*===                
                 ::+++::::::::::::::::****+                 
                    +++=====+::+*=******                    
                         +=*******+                         
                                                            `,
		}, {
			func() { o.Resize(10, 5) }, `
    2                                                       
  +=? !                                                     
  =c-;                                                      
  =;. ;.                                                    
    **                                                      
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            
                                                            `,
		},
	}

	comptest.TestComponent(t, o, w, tests)
}
