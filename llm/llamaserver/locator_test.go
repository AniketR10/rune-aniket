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

package llamaserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// fakeLibDir is a pkgLibDirLister backed by a fixed slice of paths (or an
// error) so locator tests never touch a real package manager.
type fakeLibDir struct {
	paths []string
	err   error
}

func (f fakeLibDir) LibDir(
	context.Context, string,
) (iterator.Iterator[string], error) {
	if f.err != nil {
		return nil, f.err
	}
	return iterator.FromSlice(f.paths), nil
}

func TestNewPkgLocator_PanicsOnNil(t *testing.T) {
	assert.Panics(t, func() { NewPkgLocator(nil) })
}

func TestPkgLocator_ResolvesBinaryInLibDir(t *testing.T) {
	// LibDir yields the file paths the package shipped on the workspace host;
	// the locator matches by base name without touching the local filesystem.
	bin := "/remote/host/lib/llama-server/" + serverBinaryName
	loc := NewPkgLocator(fakeLibDir{paths: []string{
		"/remote/host/lib/llama-server/README.md",
		bin,
	}})
	got, err := loc.Locate(context.Background())
	require.NoError(t, err)
	assert.Equal(t, bin, got)
}

func TestPkgLocator_NotInstalledWhenAbsent(t *testing.T) {
	loc := NewPkgLocator(fakeLibDir{paths: nil})
	_, err := loc.Locate(context.Background())
	assert.ErrorIs(t, err, ErrServerNotInstalled)
}

func TestPkgLocator_NotInstalledWhenBinaryMissing(t *testing.T) {
	// LibDir returns files, but none is llama-server.
	loc := NewPkgLocator(fakeLibDir{paths: []string{"/lib/llama-server/notes.txt"}})
	_, err := loc.Locate(context.Background())
	assert.ErrorIs(t, err, ErrServerNotInstalled)
}

func TestPkgLocator_NotInstalledWhenLibDirErrors(t *testing.T) {
	loc := NewPkgLocator(fakeLibDir{err: assert.AnError})
	_, err := loc.Locate(context.Background())
	assert.ErrorIs(t, err, ErrServerNotInstalled)
}

func TestFixedLocator(t *testing.T) {
	loc := NewFixedLocator("/usr/bin/llama-server")
	got, err := loc.Locate(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/llama-server", got)

	_, err = NewFixedLocator("").Locate(context.Background())
	assert.ErrorIs(t, err, ErrServerNotInstalled)
}
