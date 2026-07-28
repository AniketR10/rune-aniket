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

package ide

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

	"unstable.build/go-tui/text/texttest"
)

// TestPromptOnCloseNoReopenDuringShutdown asserts that a non-modal
// file-change / are-you-sure prompt does not re-open a replacement prompt
// when its OnClose fires during a programmatic Close(). The re-entrancy
// was the wedge behind the shutdown freeze: Close() ran the window OnClose
// hooks which re-opened prompts on a browser mid-teardown.
func TestPromptOnCloseNoReopenDuringShutdown(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///dirty.go")
	require.NoError(t, err)

	t.Run("areYouSurePrompt reopens when interactive", func(t *testing.T) {
		b := newExForTesting(t, texttest.NopEditor())
		defer b.Close()
		browserComp := b.ex.comp.Browser()

		b.ex.openSurePrompt(uri, nil, "changed on", true)
		require.Equal(t, 1, browserComp.FloatingWindows())

		h := &areYouSurePrompt{ex: b.ex, uri: uri, op: "changed on", reload: true}
		require.NoError(t, h.OnClose())
		assert.Equal(t, 2, browserComp.FloatingWindows(),
			"interactive dismissal should re-open the paired prompt")
	})

	t.Run("areYouSurePrompt no reopen on shutdown", func(t *testing.T) {
		b := newExForTesting(t, texttest.NopEditor())
		defer b.Close()
		browserComp := b.ex.comp.Browser()

		b.ex.openSurePrompt(uri, nil, "changed on", true)
		require.Equal(t, 1, browserComp.FloatingWindows())

		b.ex.closed = true
		h := &areYouSurePrompt{ex: b.ex, uri: uri, op: "changed on", reload: true}
		require.NoError(t, h.OnClose())
		assert.Equal(t, 1, browserComp.FloatingWindows(),
			"Close() must not re-open a replacement prompt")
	})

	t.Run("fileChangedPrompt no reopen on shutdown", func(t *testing.T) {
		b := newExForTesting(t, texttest.NopEditor())
		defer b.Close()
		browserComp := b.ex.comp.Browser()

		b.ex.openFileChangedPrompt(uri, nil, "changed on", true)
		require.Equal(t, 1, browserComp.FloatingWindows())

		b.ex.closed = true
		h := &fileChangedPrompt{ex: b.ex, uri: uri, op: "changed on", reload: true}
		require.NoError(t, h.OnClose())
		assert.Equal(t, 1, browserComp.FloatingWindows(),
			"Close() must not re-open a replacement prompt")
	})
}
