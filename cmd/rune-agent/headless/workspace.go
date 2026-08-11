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

// Package headless runs rune-agent non-interactively against a live
// Rune host.
//
// "Headless" means "not running as an extension"; it does not mean "not
// running as a plugin". Rune plugin commands (! and !!) export
// RUNE_SOCKET, RUNE_DATADIR, RUNE_INSTALLDIR, RUNE_CERT and RUNE_TOKEN,
// so a headless process can build the whole dependency graph from
// extensionapi.NewWorkspace and reuse the host's provider auth, LSP and
// syntax parser.
package headless

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"golang.org/x/oauth2"
)

// ExtensionID identifies headless runs to the host. It is deliberately
// distinct from the served extension's ID so that the client-side
// storage partition is isolated and headless dialogues never surface in
// the user's chat list.
const ExtensionID = "rune-agent-headless"

// ErrNotInRune reports that the process was not started from a Rune
// plugin command and so has no host to connect back to.
var ErrNotInRune = errors.New(
	"must be invoked from a Rune plugin command (! or !!)")

// Connect builds a Workspace from the plugin environment exported by the
// Rune host. The returned Workspace is lazy: no RPC happens until a
// capability is used.
func Connect() (*extensionapi.Workspace, error) {
	socket := os.Getenv("RUNE_SOCKET")
	if socket == "" {
		return nil, ErrNotInRune
	}
	datadir := os.Getenv("RUNE_DATADIR")
	if datadir == "" {
		return nil, ErrNotInRune
	}
	cfg := extensionapi.Config{
		Socket:     socket,
		DataDir:    datadir,
		InstallDir: os.Getenv("RUNE_INSTALLDIR"),
	}
	if tok := os.Getenv("RUNE_TOKEN"); tok != "" {
		cfg.Token = &oauth2.Token{AccessToken: tok}
	}
	if cert := os.Getenv("RUNE_CERT"); cert != "" {
		data, err := base64.StdEncoding.DecodeString(cert)
		if err != nil {
			return nil, fmt.Errorf("decode RUNE_CERT: %w", err)
		}
		cfg.Certificate = data
	}
	return extensionapi.NewWorkspace(cfg, extensionapi.Metadata{
		ExtensionID: ExtensionID,
	})
}
