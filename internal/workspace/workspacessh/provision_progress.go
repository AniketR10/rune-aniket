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

package workspacessh

import (
	"encoding/json"
	"fmt"
)

// provisionSentinel tags a stderr line as a structured provisioning progress
// update. It lets the local scanner cheaply distinguish progress JSON from
// human-readable stderr without misreading unrelated JSON as progress.
const provisionSentinel = "provision"

// readySentinel tags a stderr line as the ServerReady signal. It is distinct
// from provisionSentinel because readiness is a server-lifecycle event, not
// package-install progress: it must fire on every remote launch, including
// ones with no --install manifest, right before the gRPC server starts
// serving on stdout.
const readySentinel = "ready"

// Provisioning phases carried by ProvisionProgress.Phase.
const (
	ProvisionPhaseInstalling  = "installing"
	ProvisionPhaseDownloading = "downloading"
	ProvisionPhaseActivating  = "activating"
	ProvisionPhaseFinalizing  = "finalizing"
	ProvisionPhaseDone        = "done"
	ProvisionPhaseFailed      = "failed"
)

// ProvisionProgress is a single structured progress update streamed from the
// remote `rune -x` install loop over stderr (the only live back-channel while
// provisioning runs, before the gRPC server on stdout starts serving). It is
// serialized as one JSON object per line (JSON Lines).
type ProvisionProgress struct {
	Rune    string `json:"rune"` // sentinel, always provisionSentinel
	Index   int    `json:"i"`
	Total   int    `json:"n"`
	Package string `json:"pkg"`
	Version string `json:"ver"`
	Phase   string `json:"phase"`
	// Detail carries the underlying error text for a failed phase so the
	// notification can explain why an install failed. Empty otherwise.
	Detail string `json:"detail,omitempty"`
	// Done and Of carry byte-level sub-progress within the current package
	// (e.g. bytes downloaded / total) for the downloading phase. Units names
	// the sub-progress unit (e.g. "KiB"). All are omitted (and default to
	// zero/empty) for phases without sub-progress, so lines emitted by a
	// remote built before these fields existed still parse unchanged.
	Done  int    `json:"d,omitempty"`
	Of    int    `json:"o,omitempty"`
	Units string `json:"u,omitempty"`
}

// EncodeProvisionProgress marshals p as a single JSON line terminated by '\n'.
// The sentinel is always set so the line can be recognized on the local side.
func EncodeProvisionProgress(p ProvisionProgress) (string, error) {
	p.Rune = provisionSentinel
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

// ParseProvisionProgressLine attempts to decode a single stderr line as a
// ProvisionProgress. It returns ok=false when the line is not valid JSON or is
// not tagged with the provisioning sentinel, so unrelated JSON on stderr is
// never mistaken for progress.
func ParseProvisionProgressLine(line []byte) (ProvisionProgress, bool) {
	var p ProvisionProgress
	if err := json.Unmarshal(line, &p); err != nil {
		return ProvisionProgress{}, false
	}
	if p.Rune != provisionSentinel {
		return ProvisionProgress{}, false
	}
	return p, true
}

// Message returns the user-facing notification text for this update.
func (p ProvisionProgress) Message() string {
	pkg := p.Package
	if p.Version != "" {
		pkg = p.Package + "@" + p.Version
	}
	switch p.Phase {
	case ProvisionPhaseInstalling:
		return fmt.Sprintf("Installing toolchain (%d/%d): %s", p.Index, p.Total, pkg)
	case ProvisionPhaseDownloading:
		if p.Of > 0 {
			return fmt.Sprintf("Downloading %s (%d/%d %s)", pkg, p.Done, p.Of, p.Units)
		}
		return fmt.Sprintf("Downloading %s (%d %s)", pkg, p.Done, p.Units)
	case ProvisionPhaseActivating:
		return fmt.Sprintf("Activated %s", pkg)
	case ProvisionPhaseFinalizing:
		return "Finalizing workspace…"
	case ProvisionPhaseDone:
		return fmt.Sprintf("Installed %d/%d toolchain packages", p.Index, p.Total)
	case ProvisionPhaseFailed:
		if p.Detail != "" {
			return fmt.Sprintf("Failed to install %s: %s", pkg, p.Detail)
		}
		return fmt.Sprintf("Failed to install %s", pkg)
	default:
		return fmt.Sprintf("Provisioning %s", pkg)
	}
}

// Level returns the notification severity for this update: warnings for a
// failed package, informational otherwise.
func (p ProvisionProgress) Level() NotificationLevel {
	if p.Phase == ProvisionPhaseFailed {
		return NotificationWarning
	}
	return NotificationInfo
}

// serverReady is the sentinel-tagged control record the remote `rune -x`
// server writes to stderr immediately before it begins serving workspacerpc
// on stdout. The local side blocks the first RPC until it observes this line,
// so a client is never handed back while the remote is still provisioning and
// stdout carries no gRPC server. It is serialized as one JSON object per line.
type serverReady struct {
	Rune string `json:"rune"` // sentinel, always readySentinel
}

// encodeServerReady marshals a serverReady record as a single JSON line
// terminated by '\n'. The sentinel is always set so the line can be
// recognized on the local side.
func encodeServerReady() (string, error) {
	b, err := json.Marshal(serverReady{Rune: readySentinel})
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

// parseServerReadyLine reports whether line is the sentinel-tagged serverReady
// record. It returns false when the line is not valid JSON or is not tagged
// with the readiness sentinel, so unrelated JSON on stderr is never mistaken
// for the serving-ready signal.
func parseServerReadyLine(line []byte) bool {
	var r serverReady
	if err := json.Unmarshal(line, &r); err != nil {
		return false
	}
	return r.Rune == readySentinel
}
