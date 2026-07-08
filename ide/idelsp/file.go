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
