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

package workspacessh

import (
	"encoding/json"
	"fmt"
)

// provisionSentinel tags a stderr line as a structured provisioning progress
// update. It lets the local scanner cheaply distinguish progress JSON from
// human-readable stderr without misreading unrelated JSON as progress.
const provisionSentinel = "provision"

// Provisioning phases carried by ProvisionProgress.Phase.
const (
	ProvisionPhaseInstalling = "installing"
	ProvisionPhaseActivating = "activating"
	ProvisionPhaseDone       = "done"
	ProvisionPhaseFailed     = "failed"
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
	case ProvisionPhaseActivating:
		return fmt.Sprintf("Activated %s", pkg)
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
