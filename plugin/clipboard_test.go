package plugin

import (
	"errors"
	"sync"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/text"
)

func testCopyPaste(t *testing.T, registerID string, m text.Clipboard) {
	data := text.ClipboardData{Text: "blah", Metadata: []string{"tag"}}
	err := m.Copy(registerID, data)
	require.NoError(t, err)

	out, err := m.Paste(registerID)
	require.NoError(t, err)
	assert.Equal(t, data, out)
}

func TestClipboardManager(t *testing.T) {
	t.Run("should initialize any register upon Copy", func(t *testing.T) {
		for _, registerID := range []string{text.DefaultRegisterID, "otherRegisterID"} {
			m := NewClipboardManager()
			testCopyPaste(t, registerID, m)
		}
	})

	t.Run("should replace original inmemory register", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := NewClipboardManager()
		testCopyPaste(t, "1", m)

		now := time.Now()
		mock := NewMockClipboardRegister(ctrl)
		m.SetRegister("1", mock)
		mock.EXPECT().Copy(gomock.Any(), gomock.Any()).Return(nil)
		mock.EXPECT().Paste().Return("blah", now, nil)
		testCopyPaste(t, "1", m)
	})

	t.Run("should be able to copy/paste register if original client is closed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := NewClipboardManager()

		var wg sync.WaitGroup
		mock := proto.NewMockMuxConn(ctrl)
		mock.EXPECT().Close()

		client := &clipboardRegisterClient{
			cancelMonitor: wg.Done,
			cc:            mock,
		}

		wg.Add(1)
		m.SetRegister("1", client)
		client.Close()

		wg.Wait()

		rm, ok := m.registers["1"]
		require.True(t, ok)
		testCopyPaste(t, "1", rm)
		assert.NoError(t, m.Close())
	})

	t.Run("Paste it should always return last Copy, even if Copy fails in the first register", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := NewClipboardManager()

		mock1 := NewMockClipboardRegister(ctrl)
		mock2 := NewMockClipboardRegister(ctrl)
		now := time.Now()
		m.SetRegister("1", mock2)
		m.SetRegister("1", mock1)
		mock1.EXPECT().Copy(gomock.Any(), gomock.Any()).Return(nil)
		mock1.EXPECT().Paste().Return("blah", now, nil)
		mock2.EXPECT().Copy(gomock.Any(), gomock.Any()).Return(errors.New("kaboom"))
		mock2.EXPECT().Paste().Return("old", now.Add(-1*time.Second), nil)

		data := text.ClipboardData{Text: "blah", Metadata: []string{"tag"}}
		err := m.Copy("1", data)
		// Copy should never fail, as we use the in-memory data as backup
		require.NoError(t, err)

		out, err := m.Paste("1")
		require.NoError(t, err)
		assert.Equal(t, data, out)
	})

	t.Run("Paste it should always return last Copy, even if any register Paste fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := NewClipboardManager()

		mock1 := NewMockClipboardRegister(ctrl)
		mock2 := NewMockClipboardRegister(ctrl)
		now := time.Now()
		m.SetRegister("1", mock2)
		m.SetRegister("1", mock1)
		mock1.EXPECT().Copy(gomock.Any(), gomock.Any()).Return(nil)
		mock1.EXPECT().Paste().Return("blah", now, nil)
		mock2.EXPECT().Copy(gomock.Any(), gomock.Any()).Return(nil)
		mock2.EXPECT().Paste().Return("", now.Add(-1*time.Second), errors.New("kaboom"))

		data := text.ClipboardData{Text: "blah", Metadata: []string{"tag"}}
		err := m.Copy("1", data)
		require.NoError(t, err)

		// Paste should never fail, as we use the in-memory data as backup
		out, err := m.Paste("1")
		require.NoError(t, err)
		assert.Equal(t, data, out)
	})

	t.Run("Paste should return latest Copy, even if copied out of band", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := NewClipboardManager()

		mock1 := NewMockClipboardRegister(ctrl)
		mock2 := NewMockClipboardRegister(ctrl)
		now := time.Now()
		m.SetRegister("1", mock2)
		m.SetRegister("1", mock1)
		mock1.EXPECT().Copy(gomock.Any(), gomock.Any()).Return(nil)
		mock1.EXPECT().Paste().Return("blah", now, nil)
		mock2.EXPECT().Copy(gomock.Any(), gomock.Any()).Return(nil)
		mock2.EXPECT().Paste().Return("FUTURE DATA", now.Add(1*time.Second), nil)

		data := text.ClipboardData{Text: "blah", Metadata: []string{"tag"}}
		err := m.Copy("1", data)
		require.NoError(t, err)

		out, err := m.Paste("1")
		require.NoError(t, err)
		assert.Equal(t, text.ClipboardData{Text: "FUTURE DATA"}, out)
	})
}
