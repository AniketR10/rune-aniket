package ssh

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListenerMultipleAccepts(t *testing.T) {
	var in, out bytes.Buffer
	l := newReaderWriterListener(&in, &out)

	for i := 0; i < 2; i++ {
		out.Reset()
		in.Reset()

		_, err := in.WriteString("JJ")
		require.NoError(t, err)

		// sut
		conn, err := l.Accept()
		require.NoError(t, err)

		n, err := conn.Write([]byte("\n"))
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, "\n", out.String())

		var buf [3]byte
		n, err = conn.Read(buf[:])
		require.NoError(t, err)
		require.Equal(t, 2, n)
		assert.Equal(t, "JJ", string(buf[:2]))

		err = conn.Close()
		require.NoError(t, err)
	}
}
