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

package apiclient

import (
	"strings"
	"testing"
)

func TestCallbackPageHTML(t *testing.T) {
	if !strings.HasPrefix(callbackPageHTML, "<!doctype html") {
		t.Fatalf("callbackPageHTML must start with <!doctype html, got %q",
			callbackPageHTML[:min(40, len(callbackPageHTML))])
	}
	for _, needle := range []string{
		"You're in",
		"return to Rune",
	} {
		if !strings.Contains(callbackPageHTML, needle) {
			t.Errorf("callbackPageHTML missing %q", needle)
		}
	}
	for _, absent := range []string{
		"{{.CheckoutURL}}",
		"/checkout",
		"window.location",
		"http-equiv=\"refresh\"",
	} {
		if strings.Contains(callbackPageHTML, absent) {
			t.Errorf("callbackPageHTML must not redirect, found %q", absent)
		}
	}
}
