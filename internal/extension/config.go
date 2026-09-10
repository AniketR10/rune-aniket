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

package extension

import (
	"io"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/config/configrpc"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"unstable.build/rune/internal/rpc"
)

type configResourceServer struct {
	cfg config.Config
}

func newConfigResourceServer(cfg config.Config) *configResourceServer {
	ret := new(configResourceServer)
	ret.cfg = cfg
	return ret
}

func (s *configResourceServer) Register(
	registrar rpc.ServiceRegistrar, lock sync.Locker,
) (io.Closer, error) {
	server := configrpc.NewServer(s.cfg, lock)
	configrpc.RegisterConfigServer(registrar, server)
	return nopCloser{}, nil
}

// ConfigResources returns a map of Permission to a ResourceServer
// capable of serving each of the b Config's resources.
func ConfigResources(b config.Config) map[extensionapi.Permission]ResourceRegistrar {
	s := newConfigResourceServer(b)
	return map[extensionapi.Permission]ResourceRegistrar{
		extensionapi.PermissionConfig: s,
	}
}

type nopCloser struct {
}

func (c nopCloser) Close() error {
	return nil
}
