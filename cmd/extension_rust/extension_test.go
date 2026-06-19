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

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func TestDetectRustProject(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*fakeFS)
		want  bool
	}{
		{"empty", func(*fakeFS) {}, false},
		{"cargo manifest", func(f *fakeFS) { f.addFile("Cargo.toml") }, true},
		{"rs source only", func(f *fakeFS) { f.addEntry("main.rs", false) }, true},
		{"non-rust files only", func(f *fakeFS) { f.addEntry("README.md", false) }, false},
		{"cargo dir is not a manifest", func(f *fakeFS) { f.addDir("Cargo.toml") }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeFS()
			tc.setup(fs)
			assert.Equal(t, tc.want, detectRustProject(context.Background(), fs))
		})
	}
}

func TestToolchainInstalled(t *testing.T) {
	t.Run("empty home is not installed", func(t *testing.T) {
		assert.False(t, toolchainInstalled(newFakeFS(), ""))
	})
	t.Run("no toolchains dir is not installed", func(t *testing.T) {
		fs := newFakeFS()
		assert.False(t, toolchainInstalled(fs, "/rustup"))
	})
	t.Run("empty toolchains dir is not installed", func(t *testing.T) {
		fs := newFakeFS().addReadDir("/rustup/toolchains")
		assert.False(t, toolchainInstalled(fs, "/rustup"))
	})
	t.Run("toolchain present is installed", func(t *testing.T) {
		fs := newFakeFS().addReadDir("/rustup/toolchains",
			fakeDirEntry{name: "stable-aarch64-apple-darwin", dir: true})
		assert.True(t, toolchainInstalled(fs, "/rustup"))
	})
}

func TestReadLspPath(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		p, ok := readLspPath(nil, newFakeNotifications())
		assert.False(t, ok)
		assert.Empty(t, p)
	})
	t.Run("override set", func(t *testing.T) {
		cfg := newStubConfig(map[string]string{"lsp_path": "/opt/ra/rust-analyzer"})
		p, ok := readLspPath(cfg, newFakeNotifications())
		assert.True(t, ok)
		assert.Equal(t, "/opt/ra/rust-analyzer", p)
	})
	t.Run("empty override ignored", func(t *testing.T) {
		cfg := newStubConfig(map[string]string{"lsp_path": ""})
		p, ok := readLspPath(cfg, newFakeNotifications())
		assert.False(t, ok)
		assert.Empty(t, p)
	})
}

func TestResolveRustAnalyzer(t *testing.T) {
	t.Run("lsp_path override wins", func(t *testing.T) {
		cfg := newStubConfig(map[string]string{"lsp_path": "/opt/ra"})
		got := resolveRustAnalyzer(cfg, newFakeNotifications(), "/data")
		assert.Equal(t, "/opt/ra", got)
	})

	t.Run("defaults to bundled binary", func(t *testing.T) {
		got := resolveRustAnalyzer(nil, newFakeNotifications(), "/data")
		assert.Equal(t, "/data/bin/rust-analyzer", got)
	})

	t.Run("empty lsp_path uses bundled binary", func(t *testing.T) {
		cfg := newStubConfig(map[string]string{"lsp_path": ""})
		got := resolveRustAnalyzer(cfg, newFakeNotifications(), "/data")
		assert.Equal(t, "/data/bin/rust-analyzer", got)
	})
}

func TestResolveSysroot(t *testing.T) {
	ctx := context.Background()
	ex := newFakeExecutor().respond(
		"rustc --print sysroot",
		scriptedCmd{stdout: "/home/u/.rustup/toolchains/stable\n"})
	assert.Equal(t, "/home/u/.rustup/toolchains/stable", resolveSysroot(ctx, ex))

	exFail := newFakeExecutor().respond("rustc --print sysroot", scriptedCmd{err: assertErr})
	assert.Empty(t, resolveSysroot(ctx, exFail))
}

func TestRustInitializeParams(t *testing.T) {
	params, err := rustInitializeParams("file:///ws", "/data/bin/rust-analyzer", "/sysroot")
	require.NoError(t, err)
	assert.Equal(t, "file:///ws", params.RootURI)

	var opts map[string]any
	require.NoError(t, json.Unmarshal(params.InitializeOptions, &opts))
	assert.Equal(t, "rust", opts["langID"])
	assert.Equal(t, "/data/bin/rust-analyzer", opts["command"])
	assert.Equal(t, "/sysroot", opts["sysroot"])
	assert.Equal(t, true, opts["checkOnSave"])
	check, _ := opts["check"].(map[string]any)
	assert.Equal(t, "clippy", check["command"])

	noSysroot, err := rustInitializeParams("file:///ws", "ra", "")
	require.NoError(t, err)
	var opts2 map[string]any
	require.NoError(t, json.Unmarshal(noSysroot.InitializeOptions, &opts2))
	_, hasSysroot := opts2["sysroot"]
	assert.False(t, hasSysroot)
}

// rustInitializeCommandHasNoSpaces guards the idelsp command tokenizer,
// which splits InitializeOptions.command on spaces. A bundled path with
// no subcommand keeps the command a single argv element.
func TestRustInitializeCommandHasNoSpaces(t *testing.T) {
	params, err := rustInitializeParams("file:///ws", "/data/bin/rust-analyzer", "")
	require.NoError(t, err)
	var opts map[string]any
	require.NoError(t, json.Unmarshal(params.InitializeOptions, &opts))
	assert.NotContains(t, opts["command"], " ")
}

func TestRefreshSymlinks(t *testing.T) {
	ctx := context.Background()

	t.Run("links every user binary", func(t *testing.T) {
		fs := newFakeFS()
		ex := newFakeExecutor()
		require.NoError(t, refreshSymlinks(ctx, ex, fs, "/cargo", "/data"))
		require.Contains(t, fs.mkdirAll, "/data/bin")
		calls := ex.callsSnapshot()
		for _, name := range userBinaries {
			assert.Contains(t, calls,
				"ln -sf /cargo/bin/"+name+" /data/bin/"+name)
		}
	})

	t.Run("noop without homes", func(t *testing.T) {
		ex := newFakeExecutor()
		require.NoError(t, refreshSymlinks(ctx, ex, newFakeFS(), "", "/data"))
		assert.Empty(t, ex.callsSnapshot())
	})
}

func TestBootstrapRustupInstallsWhenAbsent(t *testing.T) {
	ctx := context.Background()
	fs := newFakeFS()
	ex := newFakeExecutor()
	notify := newFakeNotifications()

	require.NoError(t, bootstrapRustup(
		ctx, "rustup", ex, notify, fs, "/rustup", "/cargo", "/data"))

	calls := ex.callsSnapshot()
	assert.Contains(t, calls, "rustup toolchain install stable --profile minimal")
	assert.Contains(t, calls, "rustup component add rust-src clippy rustfmt")
	assert.Contains(t, calls, "ln -sf /cargo/bin/cargo /data/bin/cargo")

	msgs := notify.progressMessages()
	require.NotEmpty(t, msgs)
	assert.Equal(t, "Rust toolchain ready", msgs[len(msgs)-1])
	assertMonotonicProgress(t, notify)
}

func TestBootstrapRustupSkipsWhenInstalled(t *testing.T) {
	ctx := context.Background()
	fs := newFakeFS().addReadDir("/rustup/toolchains",
		fakeDirEntry{name: "stable-x86_64", dir: true})
	ex := newFakeExecutor()
	notify := newFakeNotifications()

	require.NoError(t, bootstrapRustup(
		ctx, "rustup", ex, notify, fs, "/rustup", "/cargo", "/data"))

	calls := ex.callsSnapshot()
	for _, c := range calls {
		assert.NotContains(t, c, "toolchain install")
	}
	assert.Contains(t, calls, "ln -sf /cargo/bin/cargo /data/bin/cargo")
	assert.Empty(t, notify.progressMessages())
}

func TestExtendWorkspaceSkipsNonRust(t *testing.T) {
	fs := newFakeFS()
	lsp := &captureLSP{}
	ext := &rustExtension{}
	registered := false
	err := ext.extendWorkspaceWith(context.Background(),
		fs, newFakeExecutor(), newFakeNotifications(), lsp,
		"/data", "/rustup", "/cargo", nil,
		func(textapi.CommandManual, textapi.REPLHandler) error {
			registered = true
			return nil
		})
	require.NoError(t, err)
	assert.False(t, registered)
	_, count := lsp.captured()
	assert.Zero(t, count)
}

func TestExtendWorkspaceRegistersAndInitializes(t *testing.T) {
	fs := newFakeFS().
		addFile("Cargo.toml").
		addFile("/data/bin/rust-analyzer").
		addReadDir("/rustup/toolchains",
			fakeDirEntry{name: "stable-x86_64", dir: true})
	ex := newFakeExecutor().respond(
		"rustc --print sysroot", scriptedCmd{stdout: "/sysroot\n"})
	lsp := &captureLSP{}
	notify := newFakeNotifications()

	var manuals []textapi.CommandManual
	ext := &rustExtension{}
	err := ext.extendWorkspaceWith(context.Background(),
		fs, ex, notify, lsp, "/data", "/rustup", "/cargo", nil,
		func(m textapi.CommandManual, _ textapi.REPLHandler) error {
			manuals = append(manuals, m)
			return nil
		})
	require.NoError(t, err)
	require.Len(t, manuals, 1)
	assert.Equal(t, rustCommandName, manuals[0].Name)

	params, count := lsp.captured()
	require.Equal(t, 1, count)
	var opts map[string]any
	require.NoError(t, json.Unmarshal(params.InitializeOptions, &opts))
	assert.Equal(t, "/data/bin/rust-analyzer", opts["command"])
	assert.Equal(t, "/sysroot", opts["sysroot"])
}

func TestRustHandlerUnknownCommand(t *testing.T) {
	_, h := newRustHandler(newFakeExecutor(), newFakeNotifications(), "/ws", "rustup", nil)
	_, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "rust", Args: []string{"frobnicate"}}, repl.NopProgressWriter())
	require.Error(t, err)
}

func TestRustHandlerForwardsToRustup(t *testing.T) {
	ex := newFakeExecutor().respond("rustup show", scriptedCmd{stdout: "stable (default)"})
	_, h := newRustHandler(ex, newFakeNotifications(), "/ws", "rustup", nil)
	it, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "rust", Args: []string{"show"}}, repl.NopProgressWriter())
	require.NoError(t, err)
	out, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Contains(t, ex.callsSnapshot(), "rustup show")
}

func TestRustHandlerReloadInvokesCallback(t *testing.T) {
	reloaded := false
	_, h := newRustHandler(newFakeExecutor(), newFakeNotifications(), "/ws", "rustup",
		func(context.Context) error { reloaded = true; return nil })
	_, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "rust", Args: []string{"reload"}}, repl.NopProgressWriter())
	require.NoError(t, err)
	assert.True(t, reloaded)
}

func TestRustHandlerDefaultTriggersReload(t *testing.T) {
	reloaded := false
	ex := newFakeExecutor().respond("rustup default nightly", scriptedCmd{stdout: "ok"})
	_, h := newRustHandler(ex, newFakeNotifications(), "/ws", "rustup",
		func(context.Context) error { reloaded = true; return nil })
	_, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "rust", Args: []string{"default", "nightly"}}, repl.NopProgressWriter())
	require.NoError(t, err)
	assert.True(t, reloaded)
}

func TestRustHandlerComplete(t *testing.T) {
	_, h := newRustHandler(newFakeExecutor(), newFakeNotifications(), "/ws", "rustup", nil)
	ctx := context.Background()

	it, err := h.Complete(ctx, "", []string{"to"})
	require.NoError(t, err)
	got, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Equal(t, []string{"toolchain"}, got)

	it, err = h.Complete(ctx, "", []string{"component", "a"})
	require.NoError(t, err)
	got, err = iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Equal(t, []string{"add"}, got)
}

// TestE2E_RefreshSymlinks links the user binaries against a real
// filesystem and executor, asserting the symlinks resolve back to the
// cargo bin sources.
func TestE2E_RefreshSymlinks(t *testing.T) {
	dataDir := t.TempDir()
	cargoHome := t.TempDir()
	cargoBin := filepath.Join(cargoHome, "bin")
	require.NoError(t, os.MkdirAll(cargoBin, 0o755))
	for _, name := range userBinaries {
		require.NoError(t, os.WriteFile(filepath.Join(cargoBin, name), []byte("#!/bin/sh\n"), 0o755))
	}

	err := refreshSymlinks(context.Background(),
		newDirExecutor(dataDir), realFS{root: dataDir}, cargoHome, dataDir)
	require.NoError(t, err)

	for _, name := range userBinaries {
		link := filepath.Join(dataDir, "bin", name)
		target, err := os.Readlink(link)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(cargoBin, name), target)
	}
}

// TestE2E_ResolveSysroot resolves the toolchain sysroot through a real
// rustc, skipping when rustc is absent.
func TestE2E_ResolveSysroot(t *testing.T) {
	findRustc(t)
	got := resolveSysroot(context.Background(), newDirExecutor(t.TempDir()))
	assert.NotEmpty(t, got)
}

// TestE2E_RustHandlerShow runs `rust show` against a real rustup.
func TestE2E_RustHandlerShow(t *testing.T) {
	findRustup(t)
	dir := t.TempDir()
	_, h := newRustHandler(newDirExecutor(dir), newFakeNotifications(), dir, "rustup", nil)
	it, err := h.HandleCommand(context.Background(),
		repl.Command{Name: "rust", Args: []string{"show"}}, repl.NopProgressWriter())
	require.NoError(t, err)
	out, err := iterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	require.Len(t, out, 1)
}
