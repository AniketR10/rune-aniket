// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
