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

package extension

import (
	"context"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/handler/finder"
)

type splitCommandHandler struct {
	mu      sync.Mutex
	clients finder.Clients
	cfg     config.Config
	new     func(context.Context, textapi.Command, finder.Clients, browserapi.Window, config.Config) (finder.RedispatchHandler, error)
	handler finder.RedispatchHandler
	window  browserapi.Window
}

func (h *splitCommandHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	h.mu.Lock()
	if h.window != nil {
		handler := h.handler
		h.mu.Unlock()
		return handler.Redispatch(ctx, cmd)
	}
	h.mu.Unlock()

	handler, err := h.new(ctx, cmd, h.clients, cmd.Window, h.cfg)
	if err != nil {
		return err
	}
	cleaningHandler := browserapi.FuncHandler(handler, func() error {
		h.mu.Lock()
		h.window = nil
		h.handler = nil
		h.mu.Unlock()
		return handler.Close()
	})
	win, err := h.clients.WindowManager.Split(browserapi.OrientationBottom, cmd.Window, cleaningHandler)
	if err != nil {
		return err
	}

	h.mu.Lock()
	h.handler = handler
	h.window = win
	h.mu.Unlock()
	return nil
}

func (h *splitCommandHandler) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	return iterator.FromSlice[string](nil), nil
}
