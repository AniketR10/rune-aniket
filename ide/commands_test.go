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
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/go-tui/text/texttest"
)

func TestCommands(t *testing.T) {
	t.Run("all commands have a handler defined", func(t *testing.T) {
		for cmd, handler := range exCommands {
			assert.NotNil(t, handler.handler, cmd)
		}
	})
	t.Run("all debug commands have a handler defined", func(t *testing.T) {
		for cmd, handler := range exDebugCommands {
			assert.NotNil(t, handler.handler, cmd)
		}
	})
	t.Run("debug commands do not collide with normal commands", func(t *testing.T) {
		for name := range exDebugCommands {
			_, ok := exCommands[name]
			assert.False(t, ok,
				"debug command %q must not also be in exCommands", name)
		}
	})
}

// TestDebugCommandsSubscriptionGating asserts that the debugCommands
// flag flips registration of `panic` and `crash`. We probe via
// DispatchCommand rather than invoking the handlers themselves,
// because the handlers intentionally crash the process.
func TestDebugCommandsSubscriptionGating(t *testing.T) {
	t.Run("disabled hides debug commands", func(t *testing.T) {
		b := newExForTesting(t, texttest.NopEditor())
		win, _ := b.ex.Browser().Focus()
		// newExForTesting calls subscribeCommands() with the
		// default ex.debugCommands == false.
		for _, name := range []string{"panic", "crash", "datarace"} {
			handled, err := b.ex.comp.DispatchCommand(context.Background(),
				textapi.Command{
					Name:   name,
					Window: win,
				})
			require.NoError(t, err)
			assert.False(t, handled,
				"%s must not be subscribed when debugCommands is false", name)
		}
	})

	t.Run("enabled subscribes debug commands", func(t *testing.T) {
		// Compare re-subscription error counts: enabling
		// debugCommands must surface exactly len(exDebugCommands)
		// extra "command already registered" errors on a second
		// subscribe pass. Probing through DispatchCommand is not an
		// option because the panic/crash handlers crash the process.
		off := newExForTesting(t, texttest.NopEditor())
		offErrCount := countSubscribeErrors(t, off.ex.subscribeCommands())

		on := newExForTesting(t, texttest.NopEditor())
		on.ex.debugCommands = true
		// First pass with debug on registers the gated commands.
		// newExForTesting already called subscribeCommands() with
		// debug off, so this second pass collides on exCommands
		// but installs panic and crash for the first time.
		_ = on.ex.subscribeCommands()
		// Third pass collides on exCommands AND exDebugCommands.
		onErrCount := countSubscribeErrors(t, on.ex.subscribeCommands())

		assert.Equal(t, offErrCount+len(exDebugCommands), onErrCount,
			"re-subscription must surface one error per debug command "+
				"when debugCommands is enabled")
	})
}

// countSubscribeErrors returns the number of "command already
// registered" errors wrapped by err (the multierror returned by
// subscribeCommands).
func countSubscribeErrors(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	return strings.Count(err.Error(), "command already registered")
}
