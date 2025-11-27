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
	"unstable.build/go-tui/component/comptest"
	"unstable.build/go-tui/term"
)

func TestInlineLeft(t *testing.T) {
	t.Run("nil slice doesn't panic", func(t *testing.T) {
		l := Inline(nil, SpanAlignmentLeft)
		l.Resize(4, 4)
		w := term.NewStringWriter(20, 9)
		tests := []comptest.TestCase{
			{
				nil, `
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
			},
		}
		comptest.TestComponent(t, l, w, tests)
	})

	t.Run("happy path", func(t *testing.T) {
		l := Inline([]Floating{
			&TestResponsive{WantWidth: 10, TestComponent: TestComponent{Ch: 'a'}},
			&TestResponsive{WantWidth: 5, TestComponent: TestComponent{Ch: 'b'}},
			&TestResponsive{WantWidth: 8, TestComponent: TestComponent{Ch: 'c'}},
			&TestResponsive{WantWidth: 1, TestComponent: TestComponent{Ch: 'd'}},
		}, SpanAlignmentLeft)
		actualWidth, actualHeight := l.Dimensions()
		assert.Equal(t, 24, actualWidth)
		assert.Equal(t, 0, actualHeight)
		l.Resize(20, 4)

		w := term.NewStringWriter(20, 9)

		tests := []comptest.TestCase{
			{
				nil, `
aaaaaaaaaabbbbbccccc
aaaaaaaaaabbbbbccccc
aaaaaaaaaabbbbbccccc
aaaaaaaaaabbbbbccccc
                    
                    
                    
                    
                    `,
			}, {
				func() { l.Resize(11, 9) }, `
aaaaaaaaaab         
aaaaaaaaaab         
aaaaaaaaaab         
aaaaaaaaaab         
aaaaaaaaaab         
aaaaaaaaaab         
aaaaaaaaaab         
aaaaaaaaaab         
aaaaaaaaaab         `,
			}, {
				func() { l.Resize(2, 2) }, `
aa                  
aa                  
                    
                    
                    
                    
                    
                    
                    `,
			},
		}
		comptest.TestComponent(t, l, w, tests)
	})
}
