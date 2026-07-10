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

package idelsp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLSPCodeActionToSemanticGroup asserts that rust-analyzer's
// codeActionGroup "group" field on a textDocument/codeAction response is
// decoded and carried through to the SDK type, and that a response
// without it (any non-rust-analyzer server) yields an empty group.
func TestLSPCodeActionToSemanticGroup(t *testing.T) {
	t.Parallel()

	t.Run("group is preserved", func(t *testing.T) {
		t.Parallel()
		// A rust-analyzer response: two import actions sharing a group.
		raw := `[
			{"title":"Import from collections::HashMap","group":"import"},
			{"title":"Import from collections::BTreeMap","group":"import"}
		]`
		var decoded []lspCodeAction
		require.NoError(t, json.Unmarshal([]byte(raw), &decoded))

		first := lspCodeActionToSemantic(decoded[0])
		second := lspCodeActionToSemantic(decoded[1])
		assert.Equal(t, "import", first.Group)
		assert.Equal(t, "import", second.Group)
		assert.Equal(t, "Import from collections::HashMap", first.Title)
	})

	t.Run("absent group yields empty string", func(t *testing.T) {
		t.Parallel()
		// A plain LSP response from e.g. gopls has no group field.
		raw := `[{"title":"Organize Imports","kind":"source.organizeImports"}]`
		var decoded []lspCodeAction
		require.NoError(t, json.Unmarshal([]byte(raw), &decoded))

		action := lspCodeActionToSemantic(decoded[0])
		assert.Empty(t, action.Group)
		assert.Equal(t, "Organize Imports", action.Title)
	})
}
