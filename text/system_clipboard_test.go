// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
)

func TestSystemClipboardDelegatesWhenAvailable(t *testing.T) {
	backing := clipboard.NewInMemory()
	clip := newSystemClipboard(backing, nil)

	require.NoError(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "x"}))
	data, err := clip.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	require.Equal(t, "x", data.Text)
}

func TestSystemClipboardOpenErrorStillFunctionsViaMemory(t *testing.T) {
	clip := newSystemClipboard(nil, errors.New("no provider"))

	err := clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "x"})
	require.EqualError(t, err, "system clipboard: no provider",
		"a failed open must surface through Copy with a user-friendly message")

	data, err := clip.Paste(clipboard.DefaultRegisterID)
	require.EqualError(t, err, "system clipboard: no provider",
		"a failed open must surface through Paste with a user-friendly message")
	require.Equal(t, "x", data.Text,
		"the in-memory fallback must still service copy/paste within Rune")
}

func TestSystemClipboardCopyPasteErrorStillFunctionsViaMemory(t *testing.T) {
	sysErr := errors.New("write failed")
	clip := newSystemClipboard(failingRegister{err: sysErr}, nil)

	err := clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "x"})
	require.EqualError(t, err, "system clipboard: write failed",
		"a failed system Copy must surface a user-friendly error")

	data, err := clip.Paste(clipboard.DefaultRegisterID)
	require.EqualError(t, err, "system clipboard: write failed",
		"a failed system Paste must surface a user-friendly error")
	require.Equal(t, "x", data.Text,
		"the in-memory fallback must still service copy/paste within Rune")
}

type failingRegister struct {
	err error
}

func (f failingRegister) Copy(string, clipboard.Data) error { return f.err }

func (f failingRegister) Paste(string) (clipboard.Data, error) {
	return clipboard.Data{}, f.err
}

// recordingRegister records every register ID written to the underlying OS
// clipboard so tests can assert what is forwarded to the system layer.
type recordingRegister struct {
	clipboard.Register
	copied []string
}

func (r *recordingRegister) Copy(registerID string, data clipboard.Data) error {
	r.copied = append(r.copied, registerID)
	return r.Register.Copy(registerID, data)
}

// TestSystemClipboardOnlyDefaultRegisterReachesOS reproduces the bug where
// typing in the modal compose editor leaked to the OS clipboard: the vi
// editor writes the typed run to its "." register on insert-mode exit, and
// the system clipboard forwarded every register to the OS. Only the default
// register represents the single OS clipboard; all other registers must stay
// in the in-memory shadow.
func TestSystemClipboardOnlyDefaultRegisterReachesOS(t *testing.T) {
	sys := &recordingRegister{Register: clipboard.NewInMemory()}
	clip := newSystemClipboard(sys, nil)

	require.NoError(t, clip.Copy(".", clipboard.Data{Text: "typed text"}))
	require.Empty(t, sys.copied,
		"a non-default register write must not reach the OS clipboard")

	require.NoError(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "yanked"}))
	require.Equal(t, []string{clipboard.DefaultRegisterID}, sys.copied,
		"only the default register write may reach the OS clipboard")
}
