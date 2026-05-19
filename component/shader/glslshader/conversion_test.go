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

package glslshader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestConversion(t *testing.T) {
	t.Run("convert color to vector", func(t *testing.T) {
		col := term.NewRGBColor(255, 245, 235)
		vec := colToVec(col, 0)
		assert.Equal(t, vec, vec3(255, 245, 235))
	})

	t.Run("convert color to vector out of range wraps", func(t *testing.T) {
		col := term.NewRGBColor(300, 301, 302)
		vec := colToVec(col, 0)
		assert.Equal(t, vec, vec3(300%256, 301%256, 302%256))
	})

	t.Run("convert default color to vector", func(t *testing.T) {
		col := term.ColorDefault
		vec := colToVec(col, term.NewRGBColor(255, 0, 0))
		assert.Equal(t, vec, vec3(255, 0, 0))
	})

	t.Run("convert default color to vector no default attribute", func(t *testing.T) {
		col := term.ColorDefault

		// term.Attribute{} makes field Bg be the zero-value (0) which is
		// itself ColorDefault, so it will leave the color unresolved.
		vec := colToVec(col, 0)

		assert.Equal(t, vec, vec3(-1, -1, -1))
	})
}
