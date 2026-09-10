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

package idelsp

import (
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

type file struct {
	uri        workspaceapi.URI
	languageID string
	docID      semanticapi.TextDocumentIdentifier
	content    string
	version    int32
	serverKey  serverKey
}

const firstFileVersion = 1

func newFile(
	uri workspaceapi.URI, content string,
	languageID string, key serverKey,
) *file {
	f := &file{
		version: firstFileVersion,
		docID: semanticapi.TextDocumentIdentifier{
			URI: convertURI(uri),
		},
		uri:        uri,
		content:    content,
		languageID: languageID,
		serverKey:  key,
	}

	return f
}

func convertURI(u workspaceapi.URI) string {
	// the workspace is always local for the language server
	// if URI is a remote uri, then the language server is executed
	// in the remote host as well.
	return fmt.Sprintf("file://%s", u.Path())
}

// uriToPath converts a file:// URI produced by convertURI back into a
// filesystem path suitable for workspaceapi.Cmd.Dir. A non-file or
// empty URI yields an empty string, which leaves the command's working
// directory unset.
func uriToPath(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}
