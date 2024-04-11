package main

import (
	"sync"

	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component/notifications"
)

var _ browser.Notifications = (*protectedNotifications)(nil)

type protectedNotifications struct {
	locker        sync.Locker
	notifications browser.Notifications
}

func (n *protectedNotifications) Notify(
	level notifications.Level, msg string, args ...interface{},
) error {
	n.locker.Lock()
	defer n.locker.Unlock()
	return n.notifications.Notify(level, msg, args...)
}

func (n *protectedNotifications) NotifyOnce(
	level notifications.Level, msg string, args ...interface{},
) error {
	n.locker.Lock()
	defer n.locker.Unlock()
	return n.notifications.NotifyOnce(level, msg, args...)
}
