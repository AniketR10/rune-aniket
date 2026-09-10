// Copyright (C) 2017-2026 The Rune Authors
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

package textrpc

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi/textrpc"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"google.golang.org/grpc"
	"unstable.build/rune/internal/cell"
)

// Client satisfies text.Editor by calling a remote editor over grpc.
type Client struct {
	textrpc.Client
}

// NewClient allocates storage for a new Client and initializes it.
func NewClient(ctx context.Context, cc grpc.ClientConnInterface) *Client {
	ret := new(Client)
	ret.Init(ctx, cc)
	return ret
}

// Init initializes this Client with broker and client.
func (c *Client) Init(ctx context.Context, cc grpc.ClientConnInterface) {
	c.Client.Init(ctx, cc)
}

// Edit requests editor server to edit buf.
func (c *Client) Edit(
	file workspaceapi.URI, buf *cell.Buffer, readOnly, recovered bool,
) (textapi.Handler, error) {
	req := NewEditRequest(file, buf, readOnly, recovered)

	_, err := c.Client.Client().Edit(context.Background(), &req)
	if err != nil {
		return nil, err
	}

	return textrpc.Token{URI: file}, nil
}

// SubscribeCommand registers a command handler over the underlying RPC client.
// This compatibility wrapper preserves Rune's older local client API.
func (c *Client) SubscribeCommand(
	man textapi.CommandManual, h textapi.CommandHandler,
) error {
	return c.RegisterCommand(man, h)
}

// RegisterCommand registers a command handler over the underlying RPC client.
func (c *Client) RegisterCommand(
	man textapi.CommandManual, h textapi.CommandHandler,
) error {
	return c.Client.RegisterCommand(man, h)
}

// RegisterREPLCommand registers a REPL command handler over the underlying RPC client.
func (c *Client) RegisterREPLCommand(
	man textapi.CommandManual, h textapi.REPLHandler,
) error {
	return c.Client.RegisterREPLCommand(man, h)
}
