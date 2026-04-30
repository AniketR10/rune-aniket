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
