package parser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/tcell/v3"
)

func TestParserIntegration(t *testing.T) {
	suite := []struct {
		description string
		input       []byte
		assert      func(t *testing.T, handler mockHandler)
	}{
		{
			"parse control attribute",
			[]byte{0x1b, '[', '1', 'm'},
			func(t *testing.T, handler mockHandler) {
				assert.Equal(t, &Attr{Type: BoldAttr}, handler.attr)
			},
		},
		{
			"parse_terminal_identity_csi standard",
			[]byte{0x1b, '[', '1', 'c'},
			func(t *testing.T, handler mockHandler) {
				assert.False(t, handler.identityReported)
			},
		},
		{
			"parse_terminal_identity_csi",
			[]byte{0x1b, '[', 'c'},
			func(t *testing.T, handler mockHandler) {
				assert.True(t, handler.identityReported)
			},
		},
		{
			"parse_terminal_identity_csi with 0",
			[]byte{0x1b, '[', '0', 'c'},
			func(t *testing.T, handler mockHandler) {
				assert.True(t, handler.identityReported)
			},
		},
		{
			"parse_terminal_identity_esc",
			[]byte{0x1b, 'Z'},
			func(t *testing.T, handler mockHandler) {
				assert.True(t, handler.identityReported)
			},
		},
		{
			"parse_terminal_identity_esc no skip params",
			[]byte{0x1b, '#', 'Z'},
			func(t *testing.T, handler mockHandler) {
				assert.False(t, handler.identityReported)
			},
		},
		{
			"parse truecolor attr",
			[]byte{
				0x1b, '[', '3', '8', ';', '2', ';', '1', '2', '8', ';', '6', '6', ';',
				'2', '5', '5', 'm',
			},
			func(t *testing.T, handler mockHandler) {
				expected := tcell.NewRGBColor(128, 66, 255)
				assert.Equal(t, &Attr{Type: ForegroundAttr, Color: expected}, handler.attr)
			},
		},
		{
			"parse designate G0 as line drawing",
			[]byte{0x1b, '(', '0'},
			func(t *testing.T, handler mockHandler) {
				assert.Equal(t, CharsetIndexG0, handler.index)
				assert.Equal(t, StandardCharsetSpecialCharacterAndLineDrawing, handler.charset)
			},
		},
		{
			"parse designate G1 as line drawing and invoke",
			[]byte{0x1b, ')', '0', 0x0e},
			func(t *testing.T, handler mockHandler) {
				assert.Equal(t, CharsetIndexG1, handler.index)
				assert.Equal(t, StandardCharsetSpecialCharacterAndLineDrawing, handler.charset)
			},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			var handler mockHandler
			handler.init()

			parser := NewParser(&handler, testSyncHandler{})

			for _, b := range test.input {
				parser.Advance(b)
			}

			test.assert(t, handler)
		})
	}
}

var _ Handler = (*mockHandler)(nil)

type mockHandler struct {
	nopHandler
	index            CharsetIndex
	charset          StandardCharset
	attr             *Attr
	identityReported bool
}

func (m *mockHandler) init() {
	m.index = CharsetIndexG0
	m.charset = StandardCharsetASCII
	m.attr = nil
	m.identityReported = false
}

func (m *mockHandler) TerminalAttribute(attr Attr) {
	m.attr = new(Attr)
	*m.attr = attr
}

func (m *mockHandler) ConfigureCharset(index CharsetIndex, charset StandardCharset) {
	m.index = index
	m.charset = charset
}

func (m *mockHandler) SetActiveCharset(index CharsetIndex) {
	m.index = index
}

func (m *mockHandler) IdentifyTerminal(identifySecondary bool) {
	m.identityReported = true
}

func (m *mockHandler) resetState() {
	m.init()
}

type testSyncHandler struct {
}

func (t testSyncHandler) SetTimeout(duration time.Duration) {
	panic("unreachable")
}

func (t testSyncHandler) ClearTimeout() {
	panic("unreachable")
}

func (t testSyncHandler) PendingTimeout() bool {
	return false
}
