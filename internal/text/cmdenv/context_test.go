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

package cmdenv

import (
	"context"
	"testing"
)

func TestWithCommandSubstitutionRoundTrip(t *testing.T) {
	if AllowsCommandSubstitution(context.Background()) {
		t.Fatal("plain context must not allow command substitution")
	}
	ctx := WithCommandSubstitution(context.Background())
	if !AllowsCommandSubstitution(ctx) {
		t.Fatal("WithCommandSubstitution(ctx) must opt ctx in")
	}
	if AllowsCommandSubstitution(context.TODO()) {
		t.Fatal("TODO context must not allow command substitution")
	}
}
