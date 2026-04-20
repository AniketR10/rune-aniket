// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package ide

import (
	"unstable.build/go-tui/ide/starlarkconfig"
)

// starlarkConfigSource is a thin in-package alias over ideconfig.Source so
// existing call sites and tests can keep using lowercase fields.
type starlarkConfigSource struct {
	src      []byte
	filename string
	params   map[string]any
	base     map[string]any
}

// decodeStarlarkConfig parses and executes src via the shared
// ide/ideconfig decoder. See ideconfig.Source for the two modes.
func decodeStarlarkConfig(s starlarkConfigSource) (map[string]any, error) {
	filename := s.filename
	if filename == "" {
		filename = "rune.star"
	}
	return starlarkconfig.Decode(starlarkconfig.Source{
		Src:      s.src,
		Filename: filename,
		Params:   s.params,
		Base:     s.base,
	})
}
