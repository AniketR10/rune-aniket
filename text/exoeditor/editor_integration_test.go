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

package exoeditor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term/vte"
)

// TestEditorTimeoutHangsUpLiveVimCleanly asserts that when the editor
// ignores the quit sequence, the graceful-quit timeout escalates to a
// SIGHUP that lets vim remove its .swp and finalize ~/.viminfo before
// the pty is torn down, leaving no .swp or .viminfo*.tmp behind.
func TestEditorTimeoutHangsUpLiveVimCleanly(t *testing.T) {
	if _, err := exec.LookPath("vim"); err != nil {
		t.Skip("vim binary not available")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	dir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	const relFile = "exo.txt"
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, relFile), []byte("hello\n"), 0o644))

	ws, scheme := newFileSchemeWorkspace(t, dir)
	fileURI, err := ws.URI(relFile)
	require.NoError(t, err)

	schedule := func(fn func()) bool { fn(); return true }
	cfg := vte.DefaultConfig()
	cfg.WidthHint = 80
	cfg.HeightHint = 24

	ed := new(
		"vim -Nu NONE {file}",
		"<esc>:{line}<enter>",
		// A quit sequence that does NOT quit vim, forcing the timeout
		// escalation path.
		"<esc>",
		schedule,
		ws,
		fileURI,
		stubNotifications{},
		stubPublisher{},
		scheme, // terminal
		scheme, // executor
		stubTabManager{},
		cfg,
		stubReloader{},
		nil,
		false,
		nil, nil, nil,
		200*time.Millisecond,
	)

	buf := cell.NewBuffer()
	h, err := ed.Edit(context.Background(), fileURI, buf, false, false)
	require.NoError(t, err)

	swapFile := filepath.Join(dir, "."+relFile+".swp")
	require.Eventually(t, func() bool {
		_, err := os.Stat(swapFile)
		return err == nil
	}, 5*time.Second, 50*time.Millisecond,
		"vim must create a swap file at %q while running", swapFile)

	require.NoError(t, h.Close())

	require.Eventually(t, func() bool {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".swp") {
				return false
			}
		}
		homeEntries, err := os.ReadDir(home)
		if err != nil {
			return false
		}
		for _, e := range homeEntries {
			name := e.Name()
			if strings.HasPrefix(name, ".viminf") &&
				strings.HasSuffix(name, ".tmp") {
				return false
			}
		}
		return true
	}, 10*time.Second, 100*time.Millisecond,
		"the timeout escalation must SIGHUP vim so it removes its .swp "+
			"and finalizes ~/.viminfo without leaving a .viminfo*.tmp")
}
