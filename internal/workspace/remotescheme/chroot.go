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

package remotescheme

import (
	"context"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

var _ schemeapi.Scheme = (*remoteChroot)(nil)

// remoteChroot keeps a chrooted view on the parent's open-file registry.
// The SDK chroot client hands back raw files with their finalizers armed,
// so without this wrapper every descriptor opened through a chrooted view
// (gogit chroots the workspace for every diff) is closed by the GC behind
// our back, against a descriptor number the remote may have recycled.
type remoteChroot struct {
	schemeapi.Scheme
	parent     *remoteScheme
	generation uint64
}

func (c *remoteChroot) track(f workspaceapi.File, err error) (workspaceapi.File, error) {
	if err != nil {
		return nil, err
	}
	return c.parent.track(f, c.generation), nil
}

func (c *remoteChroot) Open(filename string) (workspaceapi.File, error) {
	return c.track(c.Scheme.Open(filename))
}

func (c *remoteChroot) Create(filename string) (workspaceapi.File, error) {
	return c.track(c.Scheme.Create(filename))
}

func (c *remoteChroot) OpenFile(filename string, flag int, perm os.FileMode) (
	workspaceapi.File, error,
) {
	return c.track(c.Scheme.OpenFile(filename, flag, perm))
}

func (c *remoteChroot) TempFile(dir, prefix string) (workspaceapi.File, error) {
	return c.track(c.Scheme.TempFile(dir, prefix))
}

func (c *remoteChroot) NewPty(ctx context.Context) (workspaceapi.Pty, error) {
	pty, err := c.Scheme.NewPty(ctx)
	if err == nil {
		pty.Master = c.parent.track(pty.Master, c.generation)
		pty.Slave = c.parent.track(pty.Slave, c.generation)
	}
	return pty, err
}

func (c *remoteChroot) NewFile(fd uintptr, filename string) workspaceapi.File {
	return c.parent.lookupFile(c.generation, fd, filename)
}

func (c *remoteChroot) Chroot(path string) (schemeapi.Scheme, error) {
	sub, err := c.Scheme.Chroot(path)
	if err != nil {
		return nil, err
	}
	return &remoteChroot{Scheme: sub, parent: c.parent, generation: c.generation}, nil
}
