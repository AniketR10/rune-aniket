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

package auth

import "math"

const (
	// MaxRecvMsgSize is used as max grpc received message size for grpc servers.
	MaxRecvMsgSize = 1024 * 1024 * 16 // four times the default

	// MaxSendMsgSize is used as max grpc sent message size for grpc servers.
	MaxSendMsgSize = math.MaxInt32 // same as default
)
