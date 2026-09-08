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

package gogit

import (
	"io/fs"

	"github.com/go-git/go-billy/v6"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
)

var _ billy.Filesystem = billyScheme{}

type billyScheme struct {
	schemeapi.Scheme
}

// only need to override methods returning workspaceapi.File or schemeapi.Scheme
func (b billyScheme) Chroot(path string) (billy.Filesystem, error) {
	s, err := b.Scheme.Chroot(path)
	if err != nil {
		return nil, err
	}
	return billyScheme{Scheme: s}, nil
}

func (b billyScheme) Create(filename string) (billy.File, error) {
	return b.Scheme.Create(filename)
}

func (b billyScheme) Open(filename string) (billy.File, error) {
	return b.Scheme.Open(filename)
}

func (b billyScheme) OpenFile(filename string, flag int, perm fs.FileMode) (billy.File, error) {
	return b.Scheme.OpenFile(filename, flag, perm)
}

func (b billyScheme) TempFile(dir, prefix string) (billy.File, error) {
	return b.Scheme.TempFile(dir, prefix)
}
