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

// Package ideupgrade implements the in-product self-upgrade flow for
// the Rune client. It periodically polls a static manifest object
// on the public downloads CDN (no server-side proxy), prompts the
// user when a newer version is available, and (on confirmation)
// downloads, verifies and replaces the running app bundle with
// rollback support.
package ideupgrade

// Manifest describes a release artifact for one "<os>-<arch>" target.
// The shape matches the JSON written by `cmd/rune/dist.sh` and
// uploaded to `<downloads-host>/<os>-<arch>/manifest.json`.
type Manifest struct {
	Version             string `json:"version"`
	Commit              string `json:"commit,omitempty"`
	OS                  string `json:"os"`
	Arch                string `json:"arch"`
	Filename            string `json:"filename"`
	URL                 string `json:"url"`
	SHA256              string `json:"sha256"`
	Size                int64  `json:"size"`
	PublishedAt         string `json:"published_at,omitempty"`
	MinSupportedVersion string `json:"min_supported_version,omitempty"`
	Changelog           string `json:"changelog,omitempty"`
}
