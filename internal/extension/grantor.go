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
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
)

// Grantor encapsulates the ability grant or deny an extension access to
// resources. verifiedPublisher carries the trusted signing-key fingerprint
// of the extension's installed package, or "" when the extension binary is
// not attested by a signed package.
type Grantor interface {
	Grant(metadata extensionapi.Metadata, verifiedPublisher string) (bool, error)
}

// GrantAll returns Grantor that Grants all permission to all extensions.
func GrantAll() Grantor {
	return grantAll{}
}

type grantAll struct {
}

func (m grantAll) Grant(extensionapi.Metadata, string) (bool, error) {
	return true, nil
}
