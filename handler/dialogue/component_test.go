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

package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func testDrawComponent(t *testing.T, cfg ComponentConfig, expectedOut string) {
	s := NewComponent(cfg)

	s.Resize(20, 10)

	w := term.NewStringWriter(21, 11)

	tests := []testutil.ComponentTestCase{
		{Expected: expectedOut},
	}

	testutil.TestComponent(t, s, w, tests)
}

func TestComponentDraw(t *testing.T) {
	for i := 0; i < 2; i++ {
		var desc string
		if i == 0 {
			desc = "resize before adding history"
		} else {
			desc = "resize after adding history"
		}

		t.Run(desc, func(t *testing.T) {
			comp := NewComponent(ComponentConfig{})
			if i == 0 {
				comp.Resize(20, 10)
			}

			comp.AddSendMessage("wasup bro")
			comp.AddReceiveMessage("I am a AI assistant blabla.")
			comp.AddSendMessage("Speak in bro.")
			comp.AddReceiveMessage("Yo, wa'tup.")

			if i == 1 {
				comp.Resize(20, 10)
			}

			w := term.NewStringWriter(21, 11)
			tests := []testutil.ComponentTestCase{
				{
					Action: func() {},
					Expected: `wasup bro            
I am a AI assistant  
blabla.              
Speak in bro.        
Yo, wa'tup.          
                     
                     
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						comp.AddSendMessage("This is rather bore")
						comp.AddSendMessage("This is rather bore")
						comp.AddSendMessage("This is rather bore")
					},
					Expected: `I am a AI assistant  
blabla.              
Speak in bro.        
Yo, wa'tup.          
This is rather bore  
This is rather bore  
This is rather bore  
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						assert.True(t, comp.SeekUp())
						assert.False(t, comp.SeekUp())
					},
					Expected: `wasup bro            
I am a AI assistant  
blabla.              
Speak in bro.        
Yo, wa'tup.          
This is rather bore  
This is rather bore  
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						assert.True(t, comp.SeekDown())
						assert.False(t, comp.SeekDown())
					},
					Expected: `I am a AI assistant  
blabla.              
Speak in bro.        
Yo, wa'tup.          
This is rather bore  
This is rather bore  
This is rather bore  
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						comp.AddReceiveMessageChunk("1234")
						comp.Reset()
						comp.AddReceiveMessageChunk("1234")
					},
					Expected: `1234                 
                     
                     
                     
                     
                     
                     
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						frames, seq := []string{"$"}, []int{0}
						animation := component.NewAnimation(term.NopInterrupter(), frames, seq, 1)
						comp.AddReceiveMessageHint(animation, component.SpanConfig{
							PadHorizontal:    -1,
							ContentAlignment: component.SpanAlignmentLeft,
						})
					},
					Expected: `1234                 
$                    
                     
                     
                     
                     
                     
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						// should reset  receive hint
						comp.AddReceiveMessageChunk("1234")
					},
					Expected: `12341234             
                     
                     
                     
                     
                     
                     
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						frames, seq := []string{"$"}, []int{0}
						animation := component.NewAnimation(term.NopInterrupter(), frames, seq, 1)
						comp.AddReceiveMessageHint(animation, component.SpanConfig{
							PadHorizontal:    -1,
							ContentAlignment: component.SpanAlignmentLeft,
						})
					},
					Expected: `12341234             
$                    
                     
                     
                     
                     
                     
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						comp.AddReceiveMessageChunk("1234")
					},
					Expected: `123412341234         
                     
                     
                     
                     
                     
                     
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						comp.Reset()
					},
					Expected: `                     
                     
                     
                     
                     
                     
                     
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						frames, seq := []string{"$"}, []int{0}
						animation := component.NewAnimation(term.NopInterrupter(), frames, seq, 1)
						comp.AddReceiveMessageHint(animation, component.SpanConfig{
							PadHorizontal:    -1,
							ContentAlignment: component.SpanAlignmentLeft,
						})
					},
					Expected: `$                    
                     
                     
                     
                     
                     
                     
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						comp.AddSendMessage("1234")
					},
					Expected: `1234                 
                     
                     
                     
                     
                     
                     
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
			}
			testutil.TestComponent(t, comp, w, tests)
		})
	}
}

func TestComponentInputPosition(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.Resize(20, 10)

	assert.Equal(t, term.Coordinates{Y: 7, X: 1}, comp.InputPosition())
}

func TestComponentMessagesPosition(t *testing.T) {
	comp := NewComponent(ComponentConfig{MessagesRowConfig: component.SpanConfig{
		PadVertical:      2,
		PadHorizontal:    2,
		ContentAlignment: component.SpanAlignmentCentered,
	}})
	comp.Resize(20, 10)

	assert.Equal(t, term.Coordinates{Y: 1, X: 1}, comp.MessagesPosition())
}

func TestComponentResetWhileStreaming(t *testing.T) {
	comp := NewComponent(ComponentConfig{MessagesRowConfig: component.SpanConfig{
		ContentAlignment: component.SpanAlignmentCentered,
	}})
	comp.Resize(20, 10)
	assert.NotPanics(t, func() {
		comp.AddReceiveMessageChunk("1234")
		comp.Reset()
		comp.AddReceiveMessageChunk("1234")
	})
}
