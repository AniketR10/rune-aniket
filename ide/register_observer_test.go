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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/go-tui/text"
)

// registerStubEditor is a text.Editor whose registration calls succeed,
// so the observer records the command.
type registerStubEditor struct{ externalEditorStub }

func (registerStubEditor) SubscribeCommand(textapi.CommandManual, text.CommandHandler) error {
	return nil
}

func (registerStubEditor) RegisterREPLCommand(textapi.CommandManual, textapi.REPLHandler) error {
	return nil
}

func TestCommandRegisterObserverWait(t *testing.T) {
	t.Run("returns immediately when already registered", func(t *testing.T) {
		obs := newCommandRegisterObserver(registerStubEditor{})
		require.NoError(t,
			obs.SubscribeCommand(textapi.CommandManual{Name: "cmd"}, nil))

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, obs.Wait(ctx, "cmd"))
	})

	t.Run("unblocks when the command registers later", func(t *testing.T) {
		obs := newCommandRegisterObserver(registerStubEditor{})

		done := make(chan error, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			done <- obs.Wait(ctx, "cmd")
		}()

		require.NoError(t,
			obs.RegisterREPLCommand(textapi.CommandManual{Name: "cmd"}, nil))

		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("Wait did not return after the command registered")
		}
	})

	t.Run("returns ctx error when the command never registers", func(t *testing.T) {
		obs := newCommandRegisterObserver(registerStubEditor{})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		require.ErrorIs(t, obs.Wait(ctx, "cmd"), context.DeadlineExceeded)
	})
}
