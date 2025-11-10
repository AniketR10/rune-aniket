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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/component/comptest"
	"unstable.build/go-tui/term"
)

func TestIntegrationScroll(t *testing.T) {
	width, height := 8, 4
	tabspaces := 4
	wrap := false
	virtualScroll := Virtual[*Scroll]{C: newScroll(tabspaces, wrap, width, height)}
	virtualScroll.Resize(width, height)
	str := "AAAAAAAAAAAA\nBBBBBBBBBBBB\nCCCCCCCCCCCC\nDDDDDDDDDDDD"
	_, err := virtualScroll.C.ReadFrom(strings.NewReader(str))
	require.NoError(t, err)

	w := term.NewStringWriter(12, height)

	tests := []comptest.TestCase{
		{
			nil, `
AAAAAAAA    
BBBBBBBB    
CCCCCCCC    
DDDDDDDD    `,
		},
		{
			func() {
				virtualScroll.Resize(8, 3)
				virtualScroll.Move(term.Coordinates{1, 1})
			}, `
            
 AAAAAAAA   
 BBBBBBBB   
 CCCCCCCC   `,
		},
		{
			func() {
				virtualScroll.Resize(8, 4)
				virtualScroll.Move(term.Coordinates{3, 0})
			}, `
   AAAAAAAA 
   BBBBBBBB 
   CCCCCCCC 
   DDDDDDDD `,
		},
	}

	comptest.TestComponent(t, &virtualScroll, w, tests)
}

func TestVirtualDraw(t *testing.T) {
	width, height := 4, 4
	v := Virtual[*TestComponent]{C: &TestComponent{Ch: '$'}}
	v.Resize(width, height)

	w := term.NewStringWriter(width, height)

	tests := []comptest.TestCase{
		{
			nil, `
$$$$
$$$$
$$$$
$$$$`,
		},
		{
			func() {
				v.Resize(3, 3)
				v.Move(term.Coordinates{1, 1})
			}, `
    
 $$$
 $$$
 $$$`,
		},
	}
	comptest.TestComponent(t, &v, w, tests)
}
