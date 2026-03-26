// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agentools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBashToolHint(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string // substring expected in hint, or empty for no hint
	}{
		// grep-family
		{"grep", "grep -rn 'func main' .", "search_content"},
		{"rg", "rg --type go 'Handler'", "search_content"},
		{"ag", "ag 'TODO' src/", "search_content"},
		{"ack", "ack 'FIXME'", "search_content"},

		// find
		{"find", "find . -name '*.go'", "find_files"},

		// cat/head/tail
		{"cat", "cat main.go", "read_file"},
		{"head", "head -n 50 main.go", "read_file"},
		{"tail", "tail -20 main.go", "read_file"},

		// formatters
		{"gofmt", "gofmt -w main.go", "format_file"},
		{"goimports", "goimports -w .", "format_file"},
		{"go fmt", "go fmt ./...", "format_file"},

		// sed
		{"sed", "sed -i 's/old/new/g' file.go", "apply_patch"},

		// wc -l
		{"wc -l", "wc -l main.go", "read_file"},

		// negative cases — should NOT trigger any hint
		{"go test", "go test ./...", ""},
		{"go run", "go run main.go", ""},
		{"make", "make build", ""},
		{"npm install", "npm install", ""},
		{"echo", "echo hello", ""},
		{"git status", "git status", ""},
		{"docker build", "docker build -t app .", ""},
		{"find_no_space", "findutils-config", ""}, // "find" without trailing space
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := bashToolHint(tt.command)
			if tt.want == "" {
				assert.Empty(t, hint, "expected no hint for %q", tt.command)
			} else {
				assert.Contains(t, hint, tt.want,
					"hint for %q should mention %q", tt.command, tt.want)
			}
		})
	}
}
