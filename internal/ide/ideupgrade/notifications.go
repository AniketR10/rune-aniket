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

package ideupgrade

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
)

// notificationProgressWriter renders upgrade progress into a single
// pinned notification. It is the fallback used when the upgrade was
// started from the auto-check prompt rather than from the console,
// where there is no REPL progress bar to write to.
//
// It is not safe for concurrent use: every sample originates from the
// goroutine running the upgrade.
type notificationProgressWriter struct {
	n       browserapi.Notifications
	message string

	notifID  string
	lastEmit time.Time
	lastUnit string
}

var _ repl.ProgressWriter = (*notificationProgressWriter)(nil)

func (w *notificationProgressWriter) Progress(progress, total int64, units string) {
	if total <= 0 || progress > total {
		return
	}
	final := progress == total
	phaseChange := units != w.lastUnit
	if !final && !phaseChange &&
		time.Since(w.lastEmit) < upgradeProgressInterval {
		return
	}
	w.lastEmit = time.Now()
	w.lastUnit = units

	if w.notifID == "" {
		id, err := w.n.Notify(browserapi.LevelInfo, "%s", w.message)
		if err != nil {
			log.WithError(err).Warn("ideupgrade: notify upgrade start")
			return
		}
		w.notifID = id
	}
	_ = w.n.UpdateNotificationProgress(
		w.notifID, fmt.Sprintf("%s: %s", w.message, units), progress, total)
}

// close dismisses the pinned notification so a failed upgrade does not
// leave a stalled progress entry behind.
func (w *notificationProgressWriter) close() {
	if w.notifID == "" {
		return
	}
	_ = w.n.UpdateNotificationProgress(w.notifID, "", 1, 1)
}

type scheduledNotifications struct {
	notifications browserapi.Notifications
	ids           map[string]string
	schedule      func(func()) bool
}

func newScheduledNotifications(
	notifications browserapi.Notifications,
	schedule func(func()) bool,
) *scheduledNotifications {
	return &scheduledNotifications{
		notifications: notifications,
		schedule:      schedule,
		ids:           make(map[string]string),
	}
}

func (s scheduledNotifications) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	if s.notifications == nil {
		return "", nil
	}
	id := uuid.New().String()
	ok := s.schedule(func() {
		actualID, _ := s.notifications.Notify(level, msg, args...)
		s.ids[id] = actualID
	})
	if !ok {
		return "", errors.New("could not schedule notification")
	}
	return id, nil
}

func (s scheduledNotifications) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	if s.notifications == nil {
		return "", nil
	}
	id := uuid.New().String()
	ok := s.schedule(func() {
		actualID, _ := s.notifications.NotifyOnce(level, msg, args...)
		s.ids[id] = actualID
	})
	if !ok {
		return "", errors.New("could not schedule notification")
	}
	return id, nil
}

func (s scheduledNotifications) UpdateNotificationProgress(
	id, message string, progress, total int64,
) error {
	if s.notifications == nil {
		return nil
	}
	actualID := s.ids[id]
	if actualID == "" {
		return errors.New("notification not found")
	}
	ok := s.schedule(func() {
		_ = s.notifications.UpdateNotificationProgress(actualID, message, progress, total)
		s.ids[id] = actualID
	})
	if !ok {
		return errors.New("could not schedule notification update")
	}
	return nil
}
