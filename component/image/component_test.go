package image

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"
	"testing"

	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

//go:embed test/logo.png
var logo []byte

func TestNewComponent(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(logo))
	if err != nil {
		panic(fmt.Errorf("png decode: %v", err))
	}

	o := New(img, DefaultConfig())

	w := term.NewStringWriter(60, 20)

	tests := []testutil.ComponentTestCase{
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

	testutil.TestComponent(t, o, w, tests)
}
