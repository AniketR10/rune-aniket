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
	"strconv"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
)

// progressSample records one UpdateNotificationProgress call.
type progressSample struct {
	progress int64
	total    int64
}

// recordingNotifications is a browserapi.Notifications that records the
// number of Notify calls and every UpdateNotificationProgress sample so
// tests can assert the download notification is anchored once and gets
// live progress.
type recordingNotifications struct {
	mu         sync.Mutex
	notifyN    int
	nextID     int
	progresses []progressSample
}

func (n *recordingNotifications) Notify(_ browserapi.NotificationLevel, _ string, _ ...any) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notifyN++
	n.nextID++
	return "noti-" + strconv.Itoa(n.nextID), nil
}

func (n *recordingNotifications) NotifyOnce(level browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	return n.Notify(level, msg, args...)
}

func (n *recordingNotifications) UpdateNotificationProgress(_, _ string, progress, total int64) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.progresses = append(n.progresses, progressSample{progress: progress, total: total})
	return nil
}

func (n *recordingNotifications) snapshot() (notifyN int, samples []progressSample) {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]progressSample, len(n.progresses))
	copy(out, n.progresses)
	return n.notifyN, out
}
