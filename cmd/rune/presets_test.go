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

package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var presetFiles = []string{
	"preset_modal.yaml",
	"preset_standard_darwin.yaml",
	"preset_standard_linux.yaml",
	"preset_emacs.yaml",
}

// commentedOption matches a commented-out YAML mapping entry or nested
// comment, as opposed to the prose in a section banner.
var commentedOption = regexp.MustCompile(
	`^# ( *(?:#.*|(?:[a-z_0-9]+|"[^"]*"|'[^']*') *:.*))$`)

func TestPresetCommentedBlocksUncomment(t *testing.T) {
	for _, name := range presetFiles {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var shipped map[string]any
		if err := yaml.Unmarshal(raw, &shipped); err != nil {
			t.Fatalf("%s as shipped: %v", name, err)
		}
		var out []string
		for _, l := range strings.Split(string(raw), "\n") {
			if m := commentedOption.FindStringSubmatch(l); m != nil {
				out = append(out, m[1])
				continue
			}
			if strings.HasPrefix(l, "#") {
				continue
			}
			out = append(out, l)
		}
		var full map[string]any
		if err := yaml.Unmarshal([]byte(strings.Join(out, "\n")), &full); err != nil {
			t.Errorf("%s fully uncommented: %v", name, err)
			continue
		}
		t.Logf("%s: shipped=%d keys, uncommented=%d keys",
			name, len(shipped), len(full))
	}
}
