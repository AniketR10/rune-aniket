// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

//go:build e2e

package ide_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/extension/extensionv2"
	"unstable.build/rune/internal/handler/handlertest"
	"unstable.build/rune/internal/ide"
)

func TestE2E(t *testing.T) {
	t.Parallel()
	t.Run("sed arg substitution and single quote grouping works", func(t *testing.T) {
		t.Parallel()
		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		config, err := os.CreateTemp(dir, "bcd")
		require.NoError(t, err)

		file, err := os.Create(filepath.Join(dir, "e2e.go"))
		require.NoError(t, err)

		// Drop a tiny test-local sed wrapper into the workspace dir so
		// the alias does not depend on whether gsed/GNU sed happens
		// to be installed on the host. /usr/bin/sed -i.bak is portable
		// between BSD (macOS) and GNU sed.
		sedStub := filepath.Join(dir, "sedstub")
		require.NoError(t, os.WriteFile(sedStub, []byte(
			"#!/bin/sh\n"+
				"/usr/bin/sed -i.bak \"$2\" \"$3\"\n"+
				"rm -f \"$3.bak\"\n",
		), 0o755))

		_, err = config.Seek(0, 0)
		require.NoError(t, err)
		_, err = config.WriteString(fmt.Sprintf(`
editor:
  mode: modal
command:
  key: ":"
  aliases:
    sed: "!! %s -i $1 $FILE"
`, sedStub))
		require.NoError(t, err)

		uri, err := workspaceapi.CurrentUserHostURI(file.Name())
		require.NoError(t, err)

		var mu sync.Mutex
		i, drain := newHostIDE(t, &mu, dir, config.Name())

		handler := i.Ready()
		// addWorkspace is async; wait for the cwd workspace install
		// to land before driving keyboard input or opening files.
		i.WaitWorkspaces()
		mu.Lock()
		require.NoError(t, i.Open(uri))
		mu.Unlock()
		cases := []handlertest.SequenceTestCase{
			{"ia<space>bc<space>abc<space>ab<space>c<space>abc<esc>:write<enter>",
				`┌━━━━━━━━──────────┐
│o e2e.go          │
├──────────────────┤
│a bc abc ab c ab▐ │
│                  │
│                  │
│                  │
│                  │
│            NORMAL│
└──────────────────┘`},
			{":sed<space>'s/c<space>a/C<space>A/g'<enter>" +
				":notificationcloseall<enter>:reloadfile!<enter>",
				`┌━━━━━━━━──────────┐
│o e2e.go          │
├──────────────────┤
│a bC AbC Ab C Ab▐ │
│                  │
│                  │
│                  │
│                  │
│            NORMAL│
└──────────────────┘`},
		}

		// Drive the sequence through a wrapper that locks per
		// Handle/Draw and drains in-flight async saves/reloads
		// between turns. Holding mu across the whole sequence
		// would deadlock with the host scheduler (which spawns
		// goroutines that acquire mu themselves).
		handlertest.RunHandlerSequence(t, e2eLockedHandler{
			Handler: handler, mu: &mu, ide: i, drain: drain,
		}, 20, 10, cases)
	})

	t.Run("plugins executed via ! and !! get auth env vars", func(t *testing.T) {
		t.Parallel()
		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		config, err := os.CreateTemp(dir, "bcd")
		require.NoError(t, err)

		_, err = config.Seek(0, 0)
		require.NoError(t, err)
		_, err = config.WriteString(`
editor:
  mode: modal
command:
  key: ":"
`)
		require.NoError(t, err)

		var mu sync.Mutex
		runner, err := extensionv2.NewRunner(context.Background(),
			&mu, dir,
			extensionv2.WithSocketEnv("IDETEST_SOCKET"),
			extensionv2.WithDataDirEnv("IDETEST_DATADIR"),
			extensionv2.WithAuthCertEnv("IDETEST_CERT"),
			extensionv2.WithAuthTokenEnv("IDETEST_TOKEN"),
		)
		require.NoError(t, err)
		i, _ := newHostIDE(t, &mu, dir, config.Name(),
			ide.WithExtensionsRunner(runner))

		handler := i.Ready()

		filename1, err := filepath.Abs(filepath.Join(dir, "ide.env"))
		require.NoError(t, err)
		file1, err := os.Create(filename1)
		require.NoError(t, err)
		require.NoError(t, file1.Close())

		filename2, err := filepath.Abs(filepath.Join(dir, "ide2.env"))
		require.NoError(t, err)
		file2, err := os.Create(filename2)
		require.NoError(t, err)
		require.NoError(t, file2.Close())

		// allow for extension servers to be ready
		time.Sleep(5 * time.Second)

		keys, err := term.ParseKeys(
			fmt.Sprintf(`:!<space>sh<space>-c<space>"env<space>|grep<space>IDETEST<space>|<space>tee<space>%s"<enter>`, filename1) +
				fmt.Sprintf(`:!!<space>sh<space>-c<space>"env<space>|grep<space>IDETEST<space>|<space>tee<space>%s"<enter>`, filename2),
		)
		require.NoError(t, err)

		mu.Lock()
		defer mu.Unlock()
		for _, key := range keys {
			_, handled := handler.Handle(term.Event{Ch: key.Ch, Mod: key.Mod, Key: key.Key, Type: term.EventKey})
			require.True(t, handled, key.String())
		}

		assertAuthVarsPresent(t, filename1)
		assertAuthVarsPresent(t, filename2)
	})

	t.Run("plugins executed via ! and !! cwd is the workspce", func(t *testing.T) {
		t.Parallel()
		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		config, err := os.CreateTemp(dir, "bcd")
		require.NoError(t, err)

		_, err = config.Seek(0, 0)
		require.NoError(t, err)
		_, err = config.WriteString(`
editor:
  mode: modal
command:
  key: ":"
`)
		require.NoError(t, err)

		var mu sync.Mutex
		runner, err := extensionv2.NewRunner(context.Background(),
			&mu, dir,
		)
		require.NoError(t, err)
		i, _ := newHostIDE(t, &mu, dir, config.Name(),
			ide.WithExtensionsRunner(runner))

		handler := i.Ready()
		i.WaitWorkspaces()

		filename1, err := filepath.Abs(filepath.Join(dir, "ide.cwd"))
		require.NoError(t, err)
		file1, err := os.Create(filename1)
		require.NoError(t, err)
		require.NoError(t, file1.Close())

		// allow for extension runner to be ready
		time.Sleep(5 * time.Second)

		keys, err := term.ParseKeys(
			fmt.Sprintf(`:!<space>sh<space>-c<space>"pwd<space>|<space>tee<space>%s"<enter>`, filename1),
		)
		require.NoError(t, err)

		mu.Lock()
		defer mu.Unlock()
		for _, key := range keys {
			_, handled := handler.Handle(term.Event{Ch: key.Ch, Mod: key.Mod, Key: key.Key, Type: term.EventKey})
			require.True(t, handled, key.String())
		}

		assertCwdVar(t, filename1, dir)
	})
}

func assertCwdVar(t *testing.T, filename string, expectedValue string) {
	t.Helper()

	var actual string
	assert.Eventually(t, func() bool {
		data, err := os.ReadFile(filename)
		if err != nil {
			return false
		}
		actual = strings.TrimSuffix(strings.Trim(string(data), " "), "\n")
		return actual == expectedValue
	}, 5*time.Second, 50*time.Millisecond,
		"expected %q in %s, got %q", expectedValue, filename, actual)
}

func assertAuthVarsPresent(t *testing.T, filename string) {
	t.Helper()

	var vars []string
	assert.Eventually(t, func() bool {
		data, err := os.ReadFile(filename)
		if err != nil {
			return false
		}
		content := strings.TrimSuffix(strings.Trim(string(data), " "), "\n")
		if content == "" {
			vars = nil
		} else {
			vars = strings.Split(content, "\n")
		}
		return len(vars) == 4
	}, 5*time.Second, 50*time.Millisecond,
		"expected auth env vars in %s, got %v", filename, vars)

	assert.Equal(t, 4, len(vars))
	for _, v := range vars {
		kv := strings.Split(v, "=")
		switch kv[0] {
		case "IDETEST_SOCKET",
			"IDETEST_DATADIR",
			"IDETEST_TOKEN":
			require.Len(t, kv, 2)
			assert.NotZero(t, kv[1])
		case "IDETEST_CERT":
		default:
			t.Errorf("extraneous idetest env var: %q", kv[0])
		}
	}
}
