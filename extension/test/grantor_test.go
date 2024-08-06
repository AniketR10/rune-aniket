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

package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/extension"
)

var (
	grants = map[string]extension.Permissions{
		"extensionA": {
			extension.Permission("read"):  struct{}{},
			extension.Permission("write"): struct{}{},
		},
		"extensionB": {
			extension.Permission("read"): struct{}{},
		},
		"extensionC": {},
	}
	res = map[extension.Permission]extension.ResourceRegistrar{
		extension.Permission("read"):  new(MockResourceServer),
		extension.Permission("write"): new(MockResourceServer),
	}
)

func testGrantor(t *testing.T, g extension.Grantor) {
	t.Run("should deny if extension not in grant list", func(t *testing.T) {
		_, ok := g.Grant("shits", extension.Permission("write"))
		assert.False(t, ok)
	})

	t.Run("should deny if permission not in list", func(t *testing.T) {
		_, ok := g.Grant("extensionA", extension.Permission("append"))
		assert.False(t, ok)
	})

	t.Run("should deny if permission not in extension's grant list", func(t *testing.T) {
		_, ok := g.Grant("extensionB", extension.Permission("write"))
		assert.False(t, ok)

		_, ok = g.Grant("extensionC", extension.Permission("read"))
		assert.False(t, ok)
	})

	t.Run("should grant if permission in list", func(t *testing.T) {
		res, ok := g.Grant("extensionA", extension.Permission("read"))
		assert.True(t, ok)
		assert.NotNil(t, res)

		res, ok = g.Grant("extensionB", extension.Permission("read"))
		assert.True(t, ok)
		assert.NotNil(t, res)

	})
}

func TestGrantor(t *testing.T) {
	g := extension.NewInmemoryGrantor(grants, res)

	testGrantor(t, g)

	t.Run("should panic if trying to make a Grantor with mismatch of capabilities/grants", func(t *testing.T) {
		assert.Panics(t, func() {
			res := map[extension.Permission]extension.ResourceRegistrar{
				extension.Permission("read"): new(MockResourceServer),
			}
			_ = extension.NewInmemoryGrantor(grants, res)
		})
	})

}
