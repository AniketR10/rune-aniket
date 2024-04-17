package scanner

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testDispatcher struct {
	dispatched []any
}

type dispatchedOsc struct {
	params [][]byte
	bell   bool
}

type dispatchedCsi struct {
	params        [][]uint16
	intermediates []byte
	ignore        bool
	action        rune
}

type dispatchedHook struct {
	params        [][]uint16
	intermediates []byte
	ignore        bool
	action        rune
}

type dispatchedEsc struct {
	intermediates []byte
	ignore        bool
	ch            byte
}

type dispatchedPut struct {
	ch byte
}

type dispatchedUnhook struct {
}

type dispatchedPrint struct {
	r rune
}
type dispatchedExecute struct {
	ch byte
}

func (d *testDispatcher) Print(r rune) {
	d.dispatched = append(d.dispatched, dispatchedPrint{r: r})
}

func (d *testDispatcher) Execute(ch byte) {
	d.dispatched = append(d.dispatched, dispatchedExecute{ch: ch})
}

func (d *testDispatcher) Hook(params [][]uint16, intermediates []byte, ignore bool, action rune) {
	d.dispatched = append(d.dispatched, dispatchedHook{params, intermediates, ignore, action})
}

func (d *testDispatcher) Put(ch byte) {
	d.dispatched = append(d.dispatched, dispatchedPut{ch: ch})
}

func (d *testDispatcher) Unhook() {
	d.dispatched = append(d.dispatched, dispatchedUnhook{})
}

func (d *testDispatcher) OSCDispatch(params [][]byte, bellTerminated bool) {
	d.dispatched = append(d.dispatched, dispatchedOsc{params, bellTerminated})
}

func (d *testDispatcher) CSIDispatch(params [][]uint16, intermediates []byte, ignore bool, action rune) {
	d.dispatched = append(d.dispatched, dispatchedCsi{params, intermediates, ignore, action})
}

func (d *testDispatcher) ESCDispatch(intermediates []byte, ignore bool, ch byte) {
	d.dispatched = append(d.dispatched, dispatchedEsc{intermediates, ignore, ch})
}

func (d *testDispatcher) UnknownAction(action Action) {
	panic(fmt.Sprintf("unknown action: %v", action))
}

func TestScanner(t *testing.T) {
	t.Run("parse osc", func(t *testing.T) {
		var d testDispatcher
		scanner := NewScanner(&d)
		oscBytes := []byte("\x1b]2;ernestrc@ernests-mbp.lan: ~/src/go-tui\x07")

		for _, ch := range oscBytes {
			scanner.Advance(ch)
		}

		require.Len(t, d.dispatched, 1)
		osc, ok := d.dispatched[0].(dispatchedOsc)
		require.True(t, ok)
		require.Len(t, osc.params, 2)
		assert.Equal(t, oscBytes[2:3], osc.params[0])
		assert.Equal(t, oscBytes[4:len(oscBytes)-1], osc.params[1])
	})

	t.Run("parse empty osc", func(t *testing.T) {
		var d testDispatcher
		scanner := NewScanner(&d)

		for _, ch := range []byte{0x1b, 0x5d, 0x07} {
			scanner.Advance(ch)
		}

		require.Len(t, d.dispatched, 1)
		osc, ok := d.dispatched[0].(dispatchedOsc)
		require.True(t, ok)
		require.Len(t, osc.params, 1)
		assert.Equal(t, []byte{}, osc.params[0])
	})

	t.Run("parse max osc params", func(t *testing.T) {
		var d testDispatcher
		scanner := NewScanner(&d)
		var buf bytes.Buffer

		buf.WriteString("\x1b]")
		for i := 0; i < MaxOSCParams+1; i++ {
			buf.WriteByte(';')
		}
		buf.WriteString("\x1b")

		for _, ch := range buf.Bytes() {
			scanner.Advance(ch)
		}

		require.Len(t, d.dispatched, 1)
		osc, ok := d.dispatched[0].(dispatchedOsc)
		require.True(t, ok)
		require.Len(t, osc.params, MaxOSCParams)
		for _, params := range osc.params {
			assert.Equal(t, []byte{}, params)
		}
	})

	t.Run("osc bell terminated", func(t *testing.T) {
		var d testDispatcher
		scanner := NewScanner(&d)
		input := []byte("\x1b]11;ff/00/ff\x07")

		for _, ch := range input {
			scanner.Advance(ch)
		}

		require.Len(t, d.dispatched, 1)
		osc, ok := d.dispatched[0].(dispatchedOsc)
		require.True(t, ok)
		assert.True(t, osc.bell)
	})

	t.Run("osc c0 terminated", func(t *testing.T) {
		var dispatcher testDispatcher
		scanner := NewScanner(&dispatcher)
		input := []byte("\x1b]11;ff/00/ff\x1b\\")

		for _, ch := range input {
			scanner.Advance(ch)
		}

		require.Len(t, dispatcher.dispatched, 2)
		osc, ok := dispatcher.dispatched[0].(dispatchedOsc)
		require.True(t, ok)
		require.False(t, osc.bell)
	})

	t.Run("parse OSC with utf8 arguments", func(t *testing.T) {
		var dispatcher testDispatcher
		scanner := NewScanner(&dispatcher)
		input := []byte("\r\x1b]2;echo '\xaf\\_(`ツ`)_/\xaf' && sleep 1\x07")

		for _, ch := range input {
			scanner.Advance(ch)
		}

		require.Len(t, dispatcher.dispatched, 2)
		_, ok := dispatcher.dispatched[0].(dispatchedExecute)
		require.True(t, ok)
		osc, ok := dispatcher.dispatched[1].(dispatchedOsc)
		require.True(t, ok)
		require.Equal(t, []byte{'2'}, osc.params[0])
		require.Equal(t, input[5:len(input)-1], osc.params[1])
	})

	t.Run("osc containting string terminator", func(t *testing.T) {
		var dispatcher testDispatcher
		scanner := NewScanner(&dispatcher)
		input := []byte("\x1b]2;\xe6\x9c\xab\x1b\\")

		for _, ch := range input {
			scanner.Advance(ch)
		}

		require.Len(t, dispatcher.dispatched, 2)
		osc, ok := dispatcher.dispatched[0].(dispatchedOsc)
		require.True(t, ok)
		require.Equal(t, input[4:len(input)-2], osc.params[1])
	})

	t.Run("exceeds max buffer size", func(t *testing.T) {
		NUM_BYTES := MaxOSCRaw + 100
		input_START := []byte("\x1b]52;s")
		input_END := []byte("\x07")

		var d testDispatcher
		scanner := NewScanner(&d)

		for _, b := range input_START {
			scanner.Advance(b)
		}

		for i := 0; i < NUM_BYTES; i++ {
			scanner.Advance('a')
		}

		for _, b := range input_END {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)
		osc, ok := d.dispatched[0].(dispatchedOsc)
		require.True(t, ok)
		require.Len(t, osc.params, 2)
		assert.Equal(t, []byte("52"), osc.params[0])
		assert.Equal(t, NUM_BYTES+len(input_END), len(osc.params[1]))
	})

	t.Run("parse CSI max params", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteString("\x1b[")
		buf.WriteString(strings.Repeat("1;", MaxParams-1))
		buf.WriteString("p")

		var d testDispatcher
		scanner := NewScanner(&d)

		for _, b := range buf.Bytes() {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)
		csi, ok := d.dispatched[0].(dispatchedCsi)
		require.True(t, ok)
		assert.Equal(t, len(csi.params), MaxParams)
		assert.False(t, csi.ignore)
	})

	t.Run("parse CSI params ignore long params", func(t *testing.T) {
		params := strings.Repeat("1;", MaxParams)
		input := []byte(fmt.Sprintf("\x1b[%vp", params))

		var d testDispatcher
		scanner := NewScanner(&d)

		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)
		csi, ok := d.dispatched[0].(dispatchedCsi)
		require.True(t, ok)
		assert.Equal(t, len(csi.params), MaxParams)
		assert.True(t, csi.ignore)
	})

	t.Run("parse CSI params trailing semicolon", func(t *testing.T) {
		input := []byte("\x1b[4;m")
		var d testDispatcher
		scanner := NewScanner(&d)

		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)
		csi, ok := d.dispatched[0].(dispatchedCsi)
		require.True(t, ok)
		assert.Equal(t, csi.params, [][]uint16{{4}, {0}})
	})

	t.Run("parse CSI params leading semicolon", func(t *testing.T) {
		input := []byte("\x1b[;4m")
		var d testDispatcher
		scanner := NewScanner(&d)

		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)
		csi, ok := d.dispatched[0].(dispatchedCsi)
		require.True(t, ok)
		assert.Equal(t, csi.params, [][]uint16{{0}, {4}})
	})

	t.Run("parse long CSI param", func(t *testing.T) {
		input := []byte("\x1b[9223372036854775808m")
		var d testDispatcher
		scanner := NewScanner(&d)

		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)
		csi, ok := d.dispatched[0].(dispatchedCsi)
		require.True(t, ok)
		assert.Equal(t, csi.params, [][]uint16{{math.MaxUint16}})
	})

	t.Run("CSI reset", func(t *testing.T) {
		input := []byte("\x1b[3;1\x1b[?1049h")
		var d testDispatcher
		scanner := NewScanner(&d)

		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)
		csi, ok := d.dispatched[0].(dispatchedCsi)
		require.True(t, ok)
		assert.Equal(t, []byte{'?'}, csi.intermediates)
		assert.Equal(t, [][]uint16{{1049}}, csi.params)
		assert.False(t, csi.ignore)
	})

	t.Run("CSI subparameters", func(t *testing.T) {
		input := []byte("\x1b[38:2:255:0:255;1m")
		var d testDispatcher
		scanner := NewScanner(&d)

		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)
		csi, ok := d.dispatched[0].(dispatchedCsi)
		require.True(t, ok)
		assert.Equal(t, [][]uint16{{38, 2, 255, 0, 255}, {1}}, csi.params)
		assert.Empty(t, csi.intermediates)
		assert.False(t, csi.ignore)
	})

	t.Run("parse DCS max params", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteString("\x1bP")
		buf.WriteString(strings.Repeat("1;", MaxParams+1))
		buf.WriteString("p")
		var d testDispatcher
		scanner := NewScanner(&d)

		for _, b := range buf.Bytes() {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)
		dcs, ok := d.dispatched[0].(dispatchedHook)
		require.True(t, ok)
		assert.Equal(t, len(dcs.params), MaxParams)
		for i, param := range dcs.params {
			assert.Equal(t, []uint16{1}, param, i)
		}
		assert.True(t, dcs.ignore)
	})

	t.Run("dcs reset", func(t *testing.T) {
		var d testDispatcher
		scanner := NewScanner(&d)

		input := []byte("\x1b[3;1\x1bP1$tx\x9c")
		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 3)

		dcsHook, ok := d.dispatched[0].(dispatchedHook)
		require.True(t, ok)
		assert.Equal(t, []byte{'$'}, dcsHook.intermediates)
		assert.Equal(t, [][]uint16{{1}}, dcsHook.params)
		assert.False(t, dcsHook.ignore)

		assert.Equal(t, dispatchedPut{'x'}, d.dispatched[1])
		assert.Equal(t, dispatchedUnhook{}, d.dispatched[2])
	})

	t.Run("parse dcs", func(t *testing.T) {
		var d testDispatcher
		scanner := NewScanner(&d)

		input := []byte("\x1bP0;1|17/ab\x9c")
		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 7)

		dcsHook, ok := d.dispatched[0].(dispatchedHook)
		require.True(t, ok)
		assert.Equal(t, [][]uint16{{0}, {1}}, dcsHook.params)
		assert.Equal(t, '|', dcsHook.action)

		for i, b := range []byte("17/ab") {
			assert.Equal(t, dispatchedPut{ch: b}, d.dispatched[1+i])
		}

		assert.Equal(t, dispatchedUnhook{}, d.dispatched[6])
	})

	t.Run("intermediate reset on dcs exit", func(t *testing.T) {
		var d testDispatcher
		scanner := NewScanner(&d)

		input := []byte("\x1bP=1sZZZ\x1b+\x5c")
		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 6)

		esc, ok := d.dispatched[5].(dispatchedEsc)
		require.True(t, ok)
		assert.Equal(t, []byte{'+'}, esc.intermediates)
	})

	t.Run("esc reset", func(t *testing.T) {
		var d testDispatcher
		scanner := NewScanner(&d)

		input := []byte("\x1b[3;1\x1b(A")
		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)

		esc, ok := d.dispatched[0].(dispatchedEsc)
		require.True(t, ok)
		assert.Equal(t, []byte{'('}, esc.intermediates)
		assert.Equal(t, byte('A'), esc.ch)
		assert.False(t, esc.ignore)
	})

	t.Run("params buffer filled with subparam", func(t *testing.T) {
		var d testDispatcher
		scanner := NewScanner(&d)

		input := []byte("\x1b[::::::::::::::::::::::::::::::::x\x1b")
		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)

		csi, ok := d.dispatched[0].(dispatchedCsi)
		require.True(t, ok)
		assert.Empty(t, csi.intermediates)
		assert.Equal(t, [][]uint16{{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}}, csi.params)
		assert.Equal(t, rune('x'), csi.action)
		assert.True(t, csi.ignore)
	})

	t.Run("csi attr dispatch", func(t *testing.T) {
		var d testDispatcher
		scanner := NewScanner(&d)

		input := []byte{
			0x1b, '[', '3', '8', ';', '2', ';', '1', '2', '8', ';', '6', '6', ';',
			'2', '5', '5', 'm',
		}
		for _, b := range input {
			scanner.Advance(b)
		}

		require.Len(t, d.dispatched, 1)
		csi, ok := d.dispatched[0].(dispatchedCsi)
		require.True(t, ok)
		assert.Equal(t, [][]uint16{{38}, {2}, {128}, {66}, {255}}, csi.params)
		assert.Empty(t, csi.intermediates)
		assert.False(t, csi.ignore)
		assert.Equal(t, 'm', csi.action)
	})
}
