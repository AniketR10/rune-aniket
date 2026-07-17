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

package idepkg

import (
	"fmt"

	"github.com/unstablebuild/blue/release"
)

// requirementsKey is the top-level package config key listing Rune
// package IDs that must be installed before the declaring package. It
// is package metadata: it is consumed at install time and stripped
// before the overlay merges into the user config.
const requirementsKey = "requirements"

// pkgConfigRequirements parses the top-level `requirements` list from a
// package config overlay (config.yaml or config.star). It returns nil
// when the key is absent and an error when the value is not a list of
// non-empty strings.
func pkgConfigRequirements(
	filename string, data []byte,
	pkgID string, pkgVersion release.Version,
	dataDir, editorMode string,
) ([]string, error) {
	cfg, err := loadIdePkgConfigOverlay(
		filename, data, map[string]any{},
		pkgID, pkgVersion, dataDir, editorMode,
	)
	if err != nil {
		return nil, fmt.Errorf("decode package config: %w", err)
	}
	raw, ok := cfg[requirementsKey]
	if !ok {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf(
			"requirements must be a list of package IDs, got %T", raw)
	}
	reqs := make([]string, 0, len(list))
	for _, v := range list {
		s, ok := v.(string)
		if !ok || s == "" {
			return nil, fmt.Errorf(
				"requirements entries must be non-empty strings, got %v (%T)", v, v)
		}
		reqs = append(reqs, s)
	}
	return reqs, nil
}
