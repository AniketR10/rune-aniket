// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAppLaunchArgs(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		zdotDir  string
		wantArgs []string
		wantOK   bool
	}{
		{
			name:     "darwin app with zdotdir",
			goos:     "darwin",
			zdotDir:  filepath.Join("home", ".rune", "zdot"),
			wantArgs: []string{
				"--rune-zdotdir=" + filepath.Join("home", ".rune", "zdot"),
				"-G", "-w", "",
			},
			wantOK: true,
		},
		{
			name:     "darwin without zdotdir",
			goos:     "darwin",
			zdotDir:  "",
			wantArgs: []string{
				"-G", "-w", "",
			},
			wantOK: true,
		},
		{
			name:     "linux app with zdotdir",
			goos:     "linux",
			zdotDir:  filepath.Join("home", ".rune", "zdot"),
			wantArgs: []string{
				"--rune-zdotdir=" + filepath.Join("home", ".rune", "zdot"),
				"-G", "-w", "",
			},
			wantOK: true,
		},
		{
			name:     "linux without zdotdir",
			goos:     "linux",
			zdotDir:  "",
			wantArgs: []string{
				"-G", "-w", "",
			},
			wantOK: true,
		},
		{
			name:    "windows unsupported",
			goos:    "windows",
			zdotDir: "",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotOK := appLaunchArgs(tt.goos, tt.zdotDir)
			if gotOK != tt.wantOK {
				t.Fatalf("appLaunchArgs() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("appLaunchArgs() = %#v, want %#v", gotArgs, tt.wantArgs)
			}
		})
	}
}

func TestBundleZdotDir(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		execPath string
		wantDir  string
		wantOK   bool
	}{
		{
			name:     "darwin app",
			goos:     "darwin",
			execPath: filepath.Join("Applications", "Rune.app", "Contents", "MacOS", "rune"),
			wantDir:  filepath.Join("Applications", "Rune.app", "Contents", "Resources", "zdot"),
			wantOK:   true,
		},
		{
			name:     "darwin non-app still resolves to relative resources",
			goos:     "darwin",
			execPath: filepath.Join("usr", "local", "bin", "rune"),
			wantDir:  filepath.Join("usr", "local", "Resources", "zdot"),
			wantOK:   true,
		},
		{
			name:     "linux freedesktop app",
			goos:     "linux",
			execPath: filepath.Join("opt", "Rune", "rune.app", "bin", "rune"),
			wantDir:  filepath.Join("opt", "Rune", "rune.app", "share", "zdot"),
			wantOK:   true,
		},
		{
			name:     "linux non app",
			goos:     "linux",
			execPath: filepath.Join("usr", "local", "bin", "rune"),
			wantOK:   false,
		},
		{
			name:     "windows unsupported",
			goos:     "windows",
			execPath: filepath.Join("C:", "Program Files", "Rune", "rune.exe"),
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDir, gotOK := bundleZdotDir(tt.goos, tt.execPath)
			if gotOK != tt.wantOK {
				t.Fatalf("bundleZdotDir() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotDir != tt.wantDir {
				t.Fatalf("bundleZdotDir() dir = %q, want %q", gotDir, tt.wantDir)
			}
		})
	}
}

// TestInstallZdotDirDoesNotWriteToSource reproduces the
// "Rune is damaged" Gatekeeper bug. Pointing ZDOTDIR inside the signed
// bundle let zsh write .zcompdump (and similar) into a read-only sealed
// directory, invalidating the code signature. installZdotDir mirrors the
// dotfiles into a writable location so the bundle stays untouched.
func TestInstallZdotDirDoesNotWriteToSource(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "zdot")

	for _, name := range zdotFiles {
		if err := os.WriteFile(filepath.Join(srcDir, name),
			[]byte("# "+name), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	srcBefore, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("read src: %v", err)
	}

	if err := installZdotDir(srcDir, dstDir); err != nil {
		t.Fatalf("installZdotDir: %v", err)
	}

	srcAfter, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("read src after: %v", err)
	}
	if len(srcAfter) != len(srcBefore) {
		t.Fatalf("source directory mutated: before=%d after=%d",
			len(srcBefore), len(srcAfter))
	}

	for _, name := range zdotFiles {
		dst := filepath.Join(dstDir, name)
		got, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("read %s: %v", dst, err)
		}
		if want := "# " + name; string(got) != want {
			t.Fatalf("dst %s = %q, want %q", name, got, want)
		}
	}
}

func TestInstallZdotDirOverwrites(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "zdot")

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	stale := filepath.Join(dstDir, ".zshrc")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatalf("seed stale: %v", err)
	}
	for _, name := range zdotFiles {
		if err := os.WriteFile(filepath.Join(srcDir, name),
			[]byte("fresh "+name), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	if err := installZdotDir(srcDir, dstDir); err != nil {
		t.Fatalf("installZdotDir: %v", err)
	}

	got, err := os.ReadFile(stale)
	if err != nil {
		t.Fatalf("read .zshrc: %v", err)
	}
	if want := "fresh .zshrc"; string(got) != want {
		t.Fatalf(".zshrc = %q, want %q (not overwritten)", got, want)
	}
}

func TestInstallZdotDirCreatesMissingDest(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "nested", "zdot")
	if err := os.WriteFile(filepath.Join(srcDir, ".zshenv"),
		[]byte("# zshenv"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := installZdotDir(srcDir, dstDir); err != nil {
		t.Fatalf("installZdotDir: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dstDir, ".zshenv"))
	if err != nil {
		t.Fatalf("read .zshenv: %v", err)
	}
	if want := "# zshenv"; string(got) != want {
		t.Fatalf(".zshenv = %q, want %q", got, want)
	}
}

func TestInstallZdotDirSkipsMissingSourceFiles(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "zdot")
	if err := os.WriteFile(filepath.Join(srcDir, ".zshrc"),
		[]byte("# zshrc only"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := installZdotDir(srcDir, dstDir); err != nil {
		t.Fatalf("installZdotDir: %v", err)
	}

	entries, err := os.ReadDir(dstDir)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != ".zshrc" {
		t.Fatalf("dst entries = %v, want only .zshrc", entries)
	}
}

func TestResolveDefaultConfigPathPrefersYAMLThenStar(t *testing.T) {
	dataDir := t.TempDir()
	yamlPath := filepath.Join(dataDir, "config.yaml")
	starPath := filepath.Join(dataDir, "config.star")

	got := resolveDefaultConfigPath(dataDir)
	if got != yamlPath {
		t.Fatalf("resolveDefaultConfigPath() = %q, want %q when neither exists",
			got, yamlPath)
	}

	if err := os.WriteFile(starPath, []byte("config = {}\n"), 0o644); err != nil {
		t.Fatalf("write star: %v", err)
	}
	got = resolveDefaultConfigPath(dataDir)
	if got != starPath {
		t.Fatalf("resolveDefaultConfigPath() = %q, want %q when only .star exists",
			got, starPath)
	}

	if err := os.WriteFile(yamlPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	got = resolveDefaultConfigPath(dataDir)
	if got != yamlPath {
		t.Fatalf("resolveDefaultConfigPath() = %q, want %q when both exist",
			got, yamlPath)
	}
}

func TestResolveDefaultConfigPathUsesDatadirDefaultLocation(t *testing.T) {
	dataDir := filepath.Join("home", ".rune")
	got := resolveDefaultConfigPath(dataDir)
	want := filepath.Join(dataDir, "config.yaml")
	if got != want {
		t.Fatalf("resolveDefaultConfigPath() = %q, want %q", got, want)
	}
}
