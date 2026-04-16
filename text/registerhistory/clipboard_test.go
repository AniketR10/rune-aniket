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

package registerhistory

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/go-tui/text/registerset"
)

type errorClipboard struct{}

func (errorClipboard) Paste(string) (clipboard.Data, error) {
	return clipboard.Data{}, errors.New("clipboard error")
}

func (errorClipboard) Copy(string, clipboard.Data) error {
	return errors.New("clipboard error")
}

func TestClipboardHistory(t *testing.T) {
	root := clipboard.NewInMemory()
	clip := NewClipboard(root)

	history, ok := AsHistory(clip)
	require.True(t, ok, "history clipboard must implement History")
	assert.Equal(t, 0, history.HistoryLen())

	require.NoError(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "first"}))
	require.NoError(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "second"}))
	require.NoError(t, clip.Copy(registerset.ClipboardRegisterID, clipboard.Data{Text: "third"}))

	assert.Equal(t, 3, history.HistoryLen())

	data, ok := history.HistoryAt(0)
	require.True(t, ok)
	assert.Equal(t, "third", data.Text)

	data, ok = history.HistoryAt(1)
	require.True(t, ok)
	assert.Equal(t, "second", data.Text)

	data, ok = history.HistoryAt(2)
	require.True(t, ok)
	assert.Equal(t, "first", data.Text)

	_, ok = history.HistoryAt(3)
	assert.False(t, ok)

	_, ok = history.HistoryAt(-1)
	assert.False(t, ok)
}

func TestNamedRegistersAreNotTracked(t *testing.T) {
	root := clipboard.NewInMemory()
	clip := NewClipboard(root)

	history, ok := AsHistory(clip)
	require.True(t, ok)

	require.NoError(t, clip.Copy("a", clipboard.Data{Text: "named"}))
	require.NoError(t, clip.Copy(registerset.BlackHoleRegisterID, clipboard.Data{Text: "blackhole"}))
	assert.Equal(t, 0, history.HistoryLen())
}

func TestClipboardForwardsCopyAndPaste(t *testing.T) {
	root := clipboard.NewInMemory()
	clip := NewClipboard(root)

	require.NoError(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "copied"}))

	data, err := root.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "copied", data.Text)

	data, err = clip.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "copied", data.Text)
}

func TestFailedCopiesAreNotTracked(t *testing.T) {
	clip := NewClipboard(errorClipboard{})

	history, ok := AsHistory(clip)
	require.True(t, ok)

	require.Error(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "failed"}))
	assert.Equal(t, 0, history.HistoryLen())
}

func TestNewClipboardIsIdempotent(t *testing.T) {
	root := clipboard.NewInMemory()
	clip := NewClipboard(root)
	assert.Same(t, clip, NewClipboard(clip))
}

func TestAsHistoryPlainClipboard(t *testing.T) {
	root := clipboard.NewInMemory()
	_, ok := AsHistory(root)
	assert.False(t, ok, "plain clipboard should not implement History")
}
