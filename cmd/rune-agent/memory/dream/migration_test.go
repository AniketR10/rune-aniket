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

package dream

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationRegistry(t *testing.T) {
	t.Run("every version has a migration entry", func(t *testing.T) {
		for v := 2; v <= templateVersion; v++ {
			m, ok := migrations[v]
			require.True(t, ok, "missing migration for version %d", v)
			assert.NotEmpty(t, m.FixPrompt, "version %d has empty FixPrompt", v)
			assert.NotEmpty(t, m.Description, "version %d has empty Description", v)
			assert.Equal(t, v-1, m.From, "version %d From", v)
			assert.Equal(t, v, m.To, "version %d To", v)
		}
	})
}
