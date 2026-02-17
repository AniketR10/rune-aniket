// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package ide

import (
	"context"
	"errors"
	"fmt"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/component/notifications"
)

type notisManager struct {
	storage document.Service
	cfg     notifications.Config
	parent  workspaceManager
}

func newWorkspaceNotifications(
	storage document.Service,
	cfg notifications.Config,
	parent workspaceManager,
) *notisManager {
	return &notisManager{
		storage: storage,
		cfg:     cfg,
		parent:  parent,
	}
}

func (w *notisManager) new(
	uri workspaceapi.URI, c *notifications.Container,
) browserapi.Notifications {
	return &notis{
		storage: w.storage,
		root:    c,
		uri:     uri,
	}
}

func (w *notisManager) current() browserapi.Notifications {
	return &notisRouter{parent: w.parent}
}

type notisRouter struct {
	parent workspaceManager
}

type workspaceManager interface {
	focusHandler() tui.Handler
}

func (r *notisRouter) focusNotifications() browserapi.Notifications {
	focus := r.parent.focusHandler()
	if h, ok := focus.(*workspaceHandler); ok {
		return h.ex.notifications
	}
	// empty workspace
	return focus.(*ex).notifications
}

func (r *notisRouter) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return r.focusNotifications().Notify(level, msg, args...)
}

func (r *notisRouter) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return r.focusNotifications().NotifyOnce(level, msg, args...)
}

func (r *notisRouter) UpdateNotificationProgress(
	id, message string, progress, total int64,
) error {
	return r.focusNotifications().
		UpdateNotificationProgress(id, message, progress, total)
}

type notifier interface {
	Notify(level notifications.Level, msg string) string
	ID(level notifications.Level, msg string) string
	UpdateProgress(id, message string, progress, total int64) bool
}

type notis struct {
	root    notifier
	storage document.Service
	uri     workspaceapi.URI
}

// stand-in type for NotifyOnce
type storedNotification struct {
	ID string
}

// Notify formats the given msg and args and displays it on next Draw.
func (c *notis) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return c.root.Notify(notifications.Level(level), fmt.Sprintf(msg, args...)), nil
}

// NotifyOnce behaves like Notify, but only sends this notification once.
func (c *notis) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	ctx := context.Background()
	id := c.root.ID(notifications.Level(level), fmt.Sprintf(msg, args...))
	var value storedNotification
	value.ID = id
	err := c.storage.Create(ctx, id, value)
	if err == document.ErrAlreadyExists {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("storage create: %v", err)
	}
	return c.root.Notify(notifications.Level(level), fmt.Sprintf(msg, args...)), nil
}

// UpdateNotificationProgress satisfies browser.Notifications.
func (c *notis) UpdateNotificationProgress(
	id, message string, progress, total int64,
) error {
	ok := c.root.UpdateProgress(id, message, progress, total)
	if !ok {
		return errors.New("could not find notification with the " +
			"given id, or it already expired")
	}
	return nil
}
