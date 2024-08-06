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

package clipboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const registerID = "DancingWithWolves"

func testGetCopy(t *testing.T, clip Register, data Data) {
	assert.NoError(t, clip.Copy(registerID, data))

	actual, err := clip.Paste(registerID)
	require.NoError(t, err)
	assert.Equal(t, data, actual)
}

func testRegister(t *testing.T, clip Register) {
	t.Run("Sets a small value to the clipboard", func(t *testing.T) {
		str := "test1234"
		testGetCopy(t, clip, Data{Text: str})
	})

	t.Run("Sets a value with metadata to the clipboard", func(t *testing.T) {
		str := "test1234"
		testGetCopy(t, clip, Data{Text: str, Metadata: 1234})
	})

	t.Run("Sets a value to the clipboard with newlines, tabs and carriage returns", func(t *testing.T) {
		str := "a\nb\nc\nd\t\n\r\n"
		testGetCopy(t, clip, Data{Text: str})
	})

	t.Run("Sets a large value to the clipboard", func(t *testing.T) {
		str := []rune{}
		for i := 0; i < 10000; i++ {
			str = append(str, '\x00')
			str = append(str, 'a')
			str = append(str, '\n')
		}
		testGetCopy(t, clip, Data{Text: string(str)})
	})

	t.Run("once value is set, it can be retrieved multiple times", func(t *testing.T) {
		str := "test1234"
		testGetCopy(t, clip, Data{Text: str})

		for i := 0; i < 100; i++ {
			actual, err := clip.Paste(registerID)
			require.NoError(t, err)
			assert.Equal(t, Data{Text: str}, actual)
		}
	})
}

func TestEphemeralRegister(t *testing.T) {
	c := NewInMemory()
	testRegister(t, c)
}
