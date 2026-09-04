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

// Package authcallback renders the branded HTML page shown in the user's
// browser after an OAuth login callback completes.
package authcallback

import (
	_ "embed"
	"html/template"
	"io"
	"net/http"
	"strings"
)

//go:embed callback_page.html
var callbackPageHTML string

var callbackPageTmpl = template.Must(
	template.New("callback_page.html").Parse(callbackPageHTML))

// Page describes the branded copy shown on the OAuth success page.
type Page struct {
	Label   string
	Title   string
	Heading string
	Message string
}

// Render returns the fully rendered HTML document for the success page.
func Render(p Page) string {
	var buf strings.Builder
	if err := callbackPageTmpl.Execute(&buf, p); err != nil {
		return ""
	}
	return buf.String()
}

// WriteSuccess writes the branded success page to w as an HTML response.
func WriteSuccess(w http.ResponseWriter, p Page) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, Render(p))
}
