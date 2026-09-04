// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package ide

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/rune/text"
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
