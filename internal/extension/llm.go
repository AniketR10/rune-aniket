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
	"io"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	sdkllmrpc "github.com/unstablebuild/rune-go-sdk/api/llmapi/llmrpc"
	"unstable.build/rune/internal/llm/llmrpc"
	"unstable.build/rune/internal/rpc"
)

// LLMResources returns a map of Permission to a ResourceRegistrar capable
// of serving the given llmapi.Service to extensions and plugins.
func LLMResources(svc llmapi.Service) map[extensionapi.Permission]ResourceRegistrar {
	s := &llmResourceServer{svc: svc}
	return map[extensionapi.Permission]ResourceRegistrar{
		extensionapi.PermissionLLM: s,
	}
}

type llmResourceServer struct {
	svc llmapi.Service
}

func (s *llmResourceServer) Register(
	registrar rpc.ServiceRegistrar, _ sync.Locker,
) (io.Closer, error) {
	server := llmrpc.NewServer(s.svc)
	sdkllmrpc.RegisterLLMServer(registrar, server)
	return server, nil
}
