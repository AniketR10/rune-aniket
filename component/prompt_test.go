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

package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component/comptest"
	"unstable.build/go-tui/term"
)

func testDrawPrompt(t *testing.T, cfg PromptConfig, expectedOut string) {
	s := NewPrompt(cfg)

	s.Resize(20, 10)

	w := term.NewStringWriter(21, 11)

	tests := []comptest.TestCase{
		{Expected: expectedOut},
	}

	comptest.TestComponent(t, s, w, tests)
}

func TestDrawPrompt(t *testing.T) {
	t.Run("no frame", func(t *testing.T) {
		testDrawPrompt(t, PromptConfig{
			Message: "Do you?",
			Options: []string{"Yay", "Nay"},
		}, `                     
                     
      Do you?        
                     
                     
   Yay       Nay     
                     
                     
                     
                     
                     `,
		)
	})

	t.Run("with frame", func(t *testing.T) {
		testDrawPrompt(t, PromptConfig{
			Message: "Do you?",
			Options: []string{"Yay", "Nay"},
			Frame:   FrameCharSetDefault(),
		}, `                     
                     
      Do you?        
                     
                     
 ┌─────┐   ┌─────┐   
 │ Yay │   │ Nay │   
 └─────┘   └─────┘   
                     
                     
                     `,
		)
	})

	t.Run("with frame overflow options", func(t *testing.T) {
		testDrawPrompt(t, PromptConfig{
			Message: "Why soooooo serious?",
			Options: []string{"Yay", "Nay", "Say", "Wey"},
			Frame:   FrameCharSetDefault(),
		}, `                     
                     
    Why soooooo      
    serious?         
                     
                     
┌───┐┌───┐┌───┐┌───┐ 
│ Y ││ N ││ S ││ W │ 
│ a ││ a ││ a ││ e │ 
└───┘└───┘└───┘└───┘ 
                     `,
		)
	})

	t.Run("with frame overflow options", func(t *testing.T) {
		testDrawPrompt(t, PromptConfig{
			Message: "Why soooooo serious?",
			Options: []string{"Yay", "Nay", "Say", "Wey", "They", "May"},
		}, `                     
                     
    Why soooooo      
    serious?         
                     
                     
 Y  N  S  W  T  M    
 a  a  a  e  h  a    
 y  y  y  y  e  y    
             y       
                     `,
		)
	})
}

func TestPromptSetOptionAttr(t *testing.T) {
	t.Run("does not panic with frame", func(t *testing.T) {
		p := NewPrompt(PromptConfig{
			Message: "Wasup?",
			Options: []string{"Meh", "Bleh"},
			Frame:   FrameCharSetDefault(),
		})
		p.SetOptionAttr(0, term.Attributes{})
		p.SetOptionAttr(1, term.Attributes{})
	})

	t.Run("does not panic without frame", func(t *testing.T) {
		p := NewPrompt(PromptConfig{
			Message: "Wasup?",
			Options: []string{"Meh", "Bleh"},
		})
		p.SetOptionAttr(0, term.Attributes{})
		p.SetOptionAttr(1, term.Attributes{})
	})
}

func TestPromptDefaults(t *testing.T) {
	t.Run("panics on emptym message", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = NewPrompt(PromptConfig{
				Message: "", Options: []string{"a"},
			})
		})
	})
	t.Run("panics on empty options", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = NewPrompt(PromptConfig{
				Message: "blah", Options: []string{},
			})
		})
	})
}

func TestPromptInitReset(t *testing.T) {
	var p Prompt
	opts := make(map[string]*TestResponsive)
	makeTestOption := func(opt string, cfg PromptConfig) responsiveWithAttributes {
		t := testResponsive(([]rune)(opt)[0], 10)
		opts[opt] = t
		return t
	}
	p.init(makeTestOption, PromptConfig{
		Message: "?",
		Options: []string{"Y", "N"},
	})
	p.init(makeTestOption, PromptConfig{
		Message: "?!",
		Options: []string{"y", "n"},
	})

	t.Run("Draw", func(t *testing.T) {
		p.Resize(20, 10)
		w := term.NewStringWriter(21, 11)

		tests := []comptest.TestCase{
			{Expected: `
                     
                     
         ?!          
                     
                     
yyyyyyyyyynnnnnnnnnn 
yyyyyyyyyynnnnnnnnnn 
yyyyyyyyyynnnnnnnnnn 
yyyyyyyyyynnnnnnnnnn 
yyyyyyyyyynnnnnnnnnn 
                     `,
			},
		}
		comptest.TestComponent(t, &p, w, tests)
	})

	t.Run("SetAttr", func(t *testing.T) {
		attr := term.Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold}
		p.SetOptionAttr(0, attr)
		assert.Equal(t, attr, opts["y"].Attributes)
	})
}
