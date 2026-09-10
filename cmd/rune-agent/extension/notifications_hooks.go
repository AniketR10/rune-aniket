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
	"context"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/cmd/rune-agent/hooks"
)

// notificationsWithHooks is a browserapi.Notifications wrapper that
// fans every Notify / NotifyOnce call out to configured Notification
// hooks. Hooks are observational only; the wrapper always returns the
// inner Notifications result.
type notificationsWithHooks struct {
	inner browserapi.Notifications
	hooks *hooks.Runner
	cwd   workspaceapi.URI
}

func newNotificationsWithHooks(inner browserapi.Notifications, r *hooks.Runner, cwd workspaceapi.URI) browserapi.Notifications {
	return &notificationsWithHooks{inner: inner, hooks: r, cwd: cwd}
}

func (n *notificationsWithHooks) Notify(level browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	id, err := n.inner.Notify(level, msg, args...)
	n.fire(level, msg, args)
	return id, err
}

func (n *notificationsWithHooks) NotifyOnce(level browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	id, err := n.inner.NotifyOnce(level, msg, args...)
	n.fire(level, msg, args)
	return id, err
}

func (n *notificationsWithHooks) UpdateNotificationProgress(id, message string, progress, total int64) error {
	return n.inner.UpdateNotificationProgress(id, message, progress, total)
}

func (n *notificationsWithHooks) fire(level browserapi.NotificationLevel, msg string, args []any) {
	n.hooks.Run(context.Background(), hooks.Payload{
		Cwd:           n.cwd,
		HookEventName: hooks.EventNotification,
		Level:         notificationLevelString(level),
		Message:       fmt.Sprintf(msg, args...),
	})
}

func notificationLevelString(level browserapi.NotificationLevel) string {
	switch level {
	case browserapi.LevelError:
		return "error"
	case browserapi.LevelWarn:
		return "warn"
	case browserapi.LevelInfo:
		return "info"
	}
	return ""
}
