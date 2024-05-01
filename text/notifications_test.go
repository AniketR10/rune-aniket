package text

import (
	"fmt"
	"testing"

	"github.com/unstablebuild/blue/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/component/notifications"
)

func TestNotifyOnce(t *testing.T) {
	t.Run("delivers notifications only the first time it's invoked", func(t *testing.T) {
		svc := document.NewInMemoryService()
		b, err := NewComponent(nil, svc, nil, DefaultConfig())
		require.NoError(t, err)
		mock := newTestNotify()
		b.notifier = mock

		require.NoError(t, b.NotifyOnce(notifications.LevelError, "a"))
		assert.Equal(t, 1, mock.messages["a"])

		require.NoError(t, b.NotifyOnce(notifications.LevelError, "a"), fmt.Sprintf("%+v", svc))
		assert.Equal(t, 1, mock.messages["a"])
	})

	t.Run("delivers notifications with different args multiple times, if args are different", func(t *testing.T) {
		b, err := NewComponent(nil, document.NewInMemoryService(), nil, DefaultConfig())
		require.NoError(t, err)
		mock := newTestNotify()
		b.notifier = mock

		require.NoError(t, b.NotifyOnce(notifications.LevelError, "a %d", 0))
		assert.Equal(t, 1, mock.messages["a 0"])

		require.NoError(t, b.NotifyOnce(notifications.LevelError, "a %d", 1))
		assert.Equal(t, 1, mock.messages["a 1"])
		assert.Equal(t, 1, mock.messages["a 0"])
	})

	t.Run("delivers notifications with different args only once if args are the same", func(t *testing.T) {
		b, err := NewComponent(nil, document.NewInMemoryService(), nil, DefaultConfig())
		require.NoError(t, err)
		mock := newTestNotify()
		b.notifier = mock

		require.NoError(t, b.NotifyOnce(notifications.LevelError, "a %d", 0))
		assert.Equal(t, 1, mock.messages["a 0"])

		require.NoError(t, b.NotifyOnce(notifications.LevelError, "a %d", 0))
		assert.Equal(t, 1, mock.messages["a 0"])
	})

	t.Run("delivers notifications multiple times if subsequent uses Notify rather than NotifyOnce", func(t *testing.T) {
		b, err := NewComponent(nil, document.NewInMemoryService(), nil, DefaultConfig())
		require.NoError(t, err)
		mock := newTestNotify()
		b.notifier = mock

		require.NoError(t, b.NotifyOnce(notifications.LevelError, "a"))
		assert.Equal(t, 1, mock.messages["a"])

		b.Notify(notifications.LevelError, "a")
		assert.Equal(t, 2, mock.messages["a"])

		require.NoError(t, b.NotifyOnce(notifications.LevelError, "a"))
		assert.Equal(t, 2, mock.messages["a"])
	})
}

type testNotifier struct {
	messages map[string]int
}

func newTestNotify() *testNotifier {
	ret := new(testNotifier)
	ret.messages = make(map[string]int)
	return ret
}

func (t *testNotifier) Notify(level notifications.Level, msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	t.messages[formatted] = t.messages[formatted] + 1
	return
}
