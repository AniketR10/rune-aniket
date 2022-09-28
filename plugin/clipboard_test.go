package plugin

import (
	"sync"
	"testing"

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

		mock := NewMockClipboardRegister(ctrl)
		m.SetRegister("1", mock)
		mock.EXPECT().Copy(gomock.Any()).Return(nil)
		mock.EXPECT().Paste().Return("blah", nil)
		testCopyPaste(t, "1", &pluginRegister{r: mock})
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
}
