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

package main

import "github.com/unstablebuild/rune-go-sdk/api/browserapi"

type bootstrapNotifications struct {
	b *bootstrapHandler
}

func (n bootstrapNotifications) target() browserapi.Notifications {
	if n.b.realIDE != nil {
		return n.b.realIDE.Notifications()
	}
	return n.b.preIDE.Notifications()
}

func (n bootstrapNotifications) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return n.target().Notify(level, msg, args...)
}

func (n bootstrapNotifications) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return n.target().NotifyOnce(level, msg, args...)
}

func (n bootstrapNotifications) UpdateNotificationProgress(
	id, message string, progress, total int64,
) error {
	return n.target().UpdateNotificationProgress(id, message, progress, total)
}
