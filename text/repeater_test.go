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

package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
)

func TestRepeater(t *testing.T) {
	buf := cell.NewBuffer()
	scroll := component.NewScroll(buf)
	scroll.Resize(10, 10)
	cursor := NewCursor(scroll)
	repeater := NewRepeater(cursor, buf)

	cursor.InsertString("helloworld")
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "helloworldhelloworld", buf.String())

	cursor.Insert('X')
	cursor.MoveUp()
	cursor.MoveStartLine()
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "XhelloworldhelloworldX", buf.String())

	cursor.MoveStartLine()
	cursor.Select()
	cursor.MoveRight()
	cursor.DeleteSelection()
	assert.True(t, repeater.Repeat())
	assert.Equal(t, "loworldhelloworldX", buf.String())
	assert.True(t, cursor.MoveEndLine())
	cursor.cursor.X++
	require.False(t, repeater.Repeat())
	// nop because end is out of bounds
	assert.Equal(t, "loworldhelloworldX", buf.String())

	cursor.MoveLeft()
	cursor.MoveLeft()
	require.True(t, repeater.Repeat())
	assert.Equal(t, "loworldhelloworl", buf.String())
}
