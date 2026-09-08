// Copyright (C) 2017-2026 Unstable Build, LLC
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

package walkdir

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestFilterFromContext(t *testing.T) {
	if got := filterFromContext(context.Background()); got != nil {
		t.Fatalf("filterFromContext on empty ctx = %v, want nil", got)
	}

	f := matchFunc(func(string, bool) bool { return false })
	ctx := WithContextFilter(context.Background(), f)
	got := filterFromContext(ctx)
	if got == nil {
		t.Fatal("filterFromContext returned nil after WithContextFilter")
	}

	// nil filter should be a no-op.
	ctx2 := WithContextFilter(context.Background(), nil)
	if got := filterFromContext(ctx2); got != nil {
		t.Fatalf("filterFromContext after WithContextFilter(nil) = %v, want nil", got)
	}
}

func TestListFiles_appliesContextFilter(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"foo.go", "foo.go.swp", "bar.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	fs := localFS{root: dir}
	filter := matchFunc(func(relpath string, _ bool) bool {
		return strings.HasSuffix(relpath, ".swp")
	})

	ctx := WithContextFilter(context.Background(), filter)
	it, err := ListFiles(ctx, fs, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close() //nolint:errcheck

	got := drain(t, ctx, it)
	want := []string{"bar.txt", "foo.go"}
	if !equalStringSet(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestListFiles_filterSkipsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "kept"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored", "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kept", "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := localFS{root: dir}
	filter := matchFunc(func(relpath string, isDir bool) bool {
		return isDir && relpath == "ignored"
	})

	ctx := WithContextFilter(context.Background(), filter)
	it, err := ListFiles(ctx, fs, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer it.Close() //nolint:errcheck

	got := drain(t, ctx, it)
	want := []string{"kept/b.txt"}
	if !equalStringSet(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

type matchFunc func(relpath string, isDir bool) bool

func (m matchFunc) MatchRelPath(relpath string, isDir bool) bool { return m(relpath, isDir) }

func drain(t *testing.T, ctx context.Context, it interface {
	Next(context.Context) (string, bool)
	Err() error
}) []string {
	t.Helper()
	var got []string
	for {
		v, ok := it.Next(ctx)
		if !ok {
			break
		}
		got = append(got, v)
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iterator Err: %v", err)
	}
	return got
}

func equalStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}
