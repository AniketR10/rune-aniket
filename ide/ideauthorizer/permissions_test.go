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

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
)

const benchmarkComparisonScript = `go test -c -o /tmp/after.test ./handler/search/ && cd handler/search && for i in 1 2 3; do
  for v in before after; do
    /tmp/$v.test -test.run='^$' -test.bench='BenchmarkDrawRows' -test.benchmem -test.count=4 2>/dev/null | python3 -c "
import sys
for l in sys.stdin:
    if l.startswith('BenchmarkDrawRows'):
        p=l.split(); print('$v', p[0].split('-')[0], p[2], p[6])
"
  done
done | python3 -c "
import sys,collections,statistics
d=collections.defaultdict(list)
for l in sys.stdin:
    v,b,ns,al=l.split(); d[(b,v)].append(int(ns))
for b in ['BenchmarkDrawRowsASCII','BenchmarkDrawRowsWide']:
    be,af=d[(b,'before')],d[(b,'after')]
    print(f'{b:26} before med={statistics.median(be):>8.0f} | after med={statistics.median(af):>8.0f} | {100*(statistics.median(af)/statistics.median(be)-1):+.1f}%')
"`

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
			name: "command substitution recurses into inner commands",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "grep $(cat file)"},
			},
			want: []string{"cat", "grep"}, wantOK: true,
		},
		{
			name: "backtick command substitution recurses",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "grep `cat file`"},
			},
			want: []string{"cat", "grep"}, wantOK: true,
		},
		{
			name: "command substitution in double-quoted argument recurses",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", `grep -l "$(cat file)"`},
			},
			want: []string{"cat", "grep"}, wantOK: true,
		},
		{
			name: "variable expansion in argument is tolerated",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "grep $PATTERN file"},
			},
			want: []string{"grep"}, wantOK: true,
		},
		{
			name: "redirects to /dev/null and 2>&1 are tolerated",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash",
				Args: []string{"-c",
					"cd /tmp && gofmt -l $(find . -name '.go' -not -path './.git/' 2>/dev/null) 2>&1 | head -20"},
			},
			want: []string{"find", "gofmt", "head"}, wantOK: true,
		},
		{
			name: "regression: complex bash -c with cd, command substitution, redirects, and pipeline",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash",
				Args: []string{"-c",
					"cd /Users/ernestrc/.rune/worktrees/vim && gofmt -l $(find . -name '.go' -not -path './.git/' 2>/dev/null) 2>&1 | head -20"},
			},
			want: []string{"find", "gofmt", "head"}, wantOK: true,
		},
		{
			name: "process substitutions recurse into inner commands",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash",
				Args: []string{"-c",
					"gpg --show-keys key.asc | awk -F: '$1==\"fpr\"{print $10}'; diff <(gpg --show-keys package.asc | awk -F: '$1==\"fpr\"{print $10}') <(gpg --show-keys trusted.asc | awk -F: '$1==\"fpr\"{print $10}') && echo KEYRINGS-MATCH"},
			},
			want: []string{"awk", "diff", "echo", "gpg"}, wantOK: true,
		},
		{
			name: "variable expansion in command word is opaque",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "$CMD args"},
			},
			wantOK: false,
		},
		{
			name: "regression: benchmark comparison script with nested loops and python heredocs",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash",
				Args: []string{"-c", benchmarkComparisonScript},
			},
			want:   []string{"*.test", "go", "python3"},
			wantOK: true,
		},
		{
			name: "literal command substitution in command word decomposes",
			command: pluginPermissionCommandDetail{
				Path: "/bin/bash", Args: []string{"-c", "$(echo grep) foo"},
			},
			want: []string{"echo", "grep"}, wantOK: true,
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

func TestPluginPermissionProgramStorageKeyIgnoresArgs(t *testing.T) {
	t.Parallel()

	keyA := pluginPermissionProgramStorageKey("/bin/test",
		extensionapi.PermissionBrowserWindowManager)
	keyB := pluginPermissionProgramStorageKey("/bin/test",
		extensionapi.PermissionBrowserWindowManager)
	assert.Equal(t, keyA, keyB)

	assert.NotEqual(t, keyA, pluginPermissionProgramStorageKey("/bin/test",
		extensionapi.PermissionStorage),
		"key must differ by permission")
	assert.NotEqual(t, keyA, pluginPermissionProgramStorageKey("/bin/other",
		extensionapi.PermissionBrowserWindowManager),
		"key must differ by path")

	argKey := pluginPermissionStorageKey("/bin/test", []string{"a"},
		extensionapi.PermissionBrowserWindowManager)
	assert.NotEqual(t, keyA, argKey,
		"program-scoped key must not collide with the argv-scoped key")
}
