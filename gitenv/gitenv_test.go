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

package gitenv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnviron(t *testing.T) {
	stripped := []string{
		"GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_COMMON_DIR",
		"GIT_CONFIG_PARAMETERS",
		"GIT_CONFIG_COUNT",
		"GIT_CONFIG_KEY_0",
		"GIT_CONFIG_VALUE_0",
		"GIT_QUARANTINE_PATH",
		"GIT_CEILING_DIRECTORIES",
	}
	kept := []string{
		"GIT_SSH_COMMAND",
		"GIT_AUTHOR_NAME",
		"GIT_CONFIG_GLOBAL",
		"GIT_TERMINAL_PROMPT",
		"SOME_OTHER_VAR",
	}
	for _, name := range append(append([]string{}, stripped...), kept...) {
		t.Setenv(name, "value-of-"+name)
	}

	got := map[string]bool{}
	for _, kv := range Environ() {
		got[kv] = true
	}

	for _, name := range stripped {
		assert.False(t, got[name+"=value-of-"+name],
			"%s must be stripped: it pins git to the caller's repository", name)
	}
	for _, name := range kept {
		assert.True(t, got[name+"=value-of-"+name],
			"%s must be preserved: it is user-level configuration", name)
	}
}
