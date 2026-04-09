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

package registerset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
)

func TestRegisterSet(t *testing.T) {
	root := clipboard.NewInMemory()
	registers := New(root)

	require.NoError(t, registers.Copy("a", clipboard.Data{Text: "named"}))
	require.NoError(t, registers.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "unnamed"}))
	require.NoError(t, registers.Copy(ClipboardRegisterID, clipboard.Data{Text: "system"}))
	require.NoError(t, registers.Copy(BlackHoleRegisterID, clipboard.Data{Text: "ignored"}))

	data, err := registers.Paste("a")
	require.NoError(t, err)
	assert.Equal(t, "named", data.Text)

	data, err = registers.Paste("A")
	require.NoError(t, err)
	assert.Equal(t, "named", data.Text)

	data, err = registers.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "system", data.Text)

	data, err = registers.Paste(UnnamedRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "system", data.Text)

	data, err = root.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "system", data.Text)

	data, err = registers.Paste(ClipboardRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "system", data.Text)

	data, err = registers.Paste(BlackHoleRegisterID)
	require.NoError(t, err)
	assert.Empty(t, data.Text)
}

func TestRegisterSetDefaultRegisterIsSystemClipboard(t *testing.T) {
	root := clipboard.NewInMemory()
	registers := New(root)

	require.NoError(t, registers.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "from vi"}))

	data, err := root.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "from vi", data.Text)

	require.NoError(t, root.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "from system"}))

	data, err = registers.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "from system", data.Text)

	data, err = registers.Paste(UnnamedRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "from system", data.Text)
}
