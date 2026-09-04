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

package probe

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/ox-api/api/oxapi"
)

func TestRunCapturesError(t *testing.T) {
	res := run(context.Background(), "x", true, func(ctx context.Context) (string, error) {
		return "", errors.New("boom")
	})
	require.Equal(t, oxapi.CheckFail, res.Status)
	require.Equal(t, "boom", res.Detail)
	require.True(t, res.Critical)
}

func TestRunCapturesPanic(t *testing.T) {
	res := run(context.Background(), "x", false, func(ctx context.Context) (string, error) {
		panic("kaboom")
	})
	require.Equal(t, oxapi.CheckFail, res.Status)
	require.Equal(t, "panic", res.Detail)
}
