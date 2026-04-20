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

package ideauthorizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginPermissionEffectiveCommands(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		command pluginPermissionCommandDetail
		want    []string
		wantOK  bool
	}{
		{
			name: "plain command returns basename",
			command: pluginPermissionCommandDetail{
				Path: "/usr/bin/grep", Args: []string{"foo"},
			},
			want: []string{"grep"}, wantOK: true,
		},
		{
			name: "shell wrapper with single inner command",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "grep foo file.txt"},
			},
			want: []string{"grep"}, wantOK: true,
		},
		{
			name: "shell wrapper with compound and-chain",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash",
				Args: []string{"-c",
					`make test && grep "bla" file && bash -c "cd /tmp/abc && rm -rf ."`},
			},
			want: []string{"grep", "make", "rm"}, wantOK: true,
		},
		{
			name: "pipeline decomposes",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "grep foo f.txt | wc -l"},
			},
			want: []string{"grep", "wc"}, wantOK: true,
		},
		{
			name: "if-clause recurses into branches",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash",
				Args: []string{"-c", "if true; then ls a; else grep b file; fi"},
			},
			want: []string{"grep", "ls"}, wantOK: true,
		},
		{
			name: "for-clause recurses into body",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "for i in a b; do echo x; done"},
			},
			want: []string{"echo"}, wantOK: true,
		},
		{
			name: "subshell recurses into body",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "(echo a; grep b file)"},
			},
			want: []string{"echo", "grep"}, wantOK: true,
		},
		{
			name: "exec unwraps to inner command",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "exec /usr/bin/grep foo"},
			},
			want: []string{"grep"}, wantOK: true,
		},
		{
			name: "no-fork builtins don't contribute",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "cd /tmp && grep foo"},
			},
			want: []string{"grep"}, wantOK: true,
		},
		{
			name: "command substitution is opaque",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "grep $(cat file)"},
			},
			wantOK: false,
		},
		{
			name: "variable expansion in command word is opaque",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "$CMD args"},
			},
			wantOK: false,
		},
		{
			name: "eval is rejected",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "eval echo foo"},
			},
			wantOK: false,
		},
		{
			name: "source is rejected",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "source /tmp/a && echo"},
			},
			wantOK: false,
		},
		{
			name: "plain shell invocation without -c is opaque",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"--login"},
			},
			wantOK: false,
		},
		{
			name: "unparseable script is opaque",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "if then fi"},
			},
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := pluginPermissionEffectiveCommands(tc.command)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
