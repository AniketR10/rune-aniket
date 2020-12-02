package term

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	hexBlack   = "#000000"
	hexWhite   = "#FFFFFF"
	hexRed     = "#FF0000"
	hexGreen   = "#00FF00"
	hexYellow  = "#FFFF00"
	hexBlue    = "#0000FF"
	hexMagenta = "#FF00FF"
	hexCyan    = "#00FFFF"

	defaultBg = hexBlack
	defaultFg = hexWhite

	defaultCursorStyle = "background: red;"
)

// HTMLWriter implements term.Writer by rendering an HTML representation.
type HTMLWriter struct {
	cellbuf       []Cell
	buffer        bytes.Buffer
	cursor        int
	width, height int
	defaultAttr   Attributes
	cursorStyle   string
}

// NewHTMLWriter allocates storage for a new HTMLWriter and initializes it.
func NewHTMLWriter(width, height int) (t *HTMLWriter) {
	t = new(HTMLWriter)
	t.defaultAttr = Attributes{Bg: ColorBlack, Fg: ColorWhite}
	t.Resize(width, height)
	t.cursorStyle = defaultCursorStyle
	return
}

// SetCursorStyle sets the CSS style of the cursor.
func (w *HTMLWriter) SetCursorStyle(style string) {
	w.cursorStyle = style
}

// Resize satisfies Writer.
func (w *HTMLWriter) Resize(width, height int) {
	w.width, w.height = width, height
	w.cellbuf = make([]Cell, width*height)
}

// SetCell satisfies Writer.
func (w *HTMLWriter) SetCell(pos Coordinates, cell Cell) {
	if outOfBounds(w.height, w.width, pos) {
		return
	}
	idx := pos.Y*w.width + pos.X
	w.cellbuf[idx] = cell
}

func (w *HTMLWriter) getHexColor(attr, def Attribute) (hex string, ok bool) {
	// NOTE: assumes SetOutputMode(OutputNormal) behaviour
	color := attr & 0x0F
	ok = true
	switch color {
	case ColorBlack:
		hex = hexBlack
	case ColorRed:
		hex = hexRed
	case ColorGreen:
		hex = hexGreen
	case ColorYellow:
		hex = hexYellow
	case ColorBlue:
		hex = hexBlue
	case ColorMagenta:
		hex = hexMagenta
	case ColorCyan:
		hex = hexCyan
	case ColorWhite:
		hex = hexWhite
	/* case ColorDefault: */
	default:
		ok = false
	}

	if attr == def {
		ok = false
	}

	return
}

func (w *HTMLWriter) convertToCSS(attr Attributes, ignoreDefault bool) (
	css string, needsFg, needsBg bool,
) {
	var builder strings.Builder

	if attr.Fg&AttrBold != 0 {
		attr.Fg &^= AttrBold
		needsFg = true
		builder.WriteString("font-weight:bold;")
	}

	if attr.Fg&AttrUnderline != 0 {
		attr.Fg &^= AttrUnderline
		needsFg = true
		builder.WriteString("text-decoration:underline;")
	}

	if attr.Bg&AttrUnderline != 0 {
		/* ignore */
		attr.Bg &^= AttrUnderline
	}

	if attr.Bg&AttrBold != 0 {
		/* ignore */
		attr.Bg &^= AttrBold
	}

	fgReverse := attr.Fg&AttrReverse != 0
	bgReverse := attr.Bg&AttrReverse != 0
	if fgReverse || bgReverse {
		if fgReverse {
			attr.Fg &^= AttrReverse
		}
		if bgReverse {
			attr.Bg &^= AttrReverse
		}

		// at this point all attributes should be removed
		if attr.Fg == 0 {
			attr.Fg = w.defaultAttr.Fg
		}
		if attr.Bg == 0 {
			attr.Bg = w.defaultAttr.Bg
		}
		tempBg := attr.Bg
		attr.Bg = attr.Fg
		attr.Fg = tempBg
	}

	bgHex, needsBg := w.getHexColor(attr.Bg, w.defaultAttr.Bg)
	if !ignoreDefault || needsBg {
		builder.WriteString("background:")
		builder.WriteString(bgHex)
		builder.WriteString(";")
	}

	fgHex, needsColorFg := w.getHexColor(attr.Fg, w.defaultAttr.Fg)
	if !ignoreDefault || needsColorFg {
		builder.WriteString("color:")
		builder.WriteString(fgHex)
		builder.WriteString(";")
	}

	return builder.String(),
		needsFg || needsColorFg || !ignoreDefault,
		needsBg || !ignoreDefault
}

func (w *HTMLWriter) writeCellStyle(i int, c Cell) bool {
	if w.cursor == i {
		w.buffer.WriteString("<span style=\"")
		w.buffer.WriteString(w.cursorStyle)
		w.buffer.WriteString("\">")
		return true
	}

	styleStr, needsFg, needsBg := w.convertToCSS(Attributes{Bg: c.Bg, Fg: c.Fg}, true)
	if !needsFg && !needsBg {
		return false
	}

	if !needsBg && c.Ch == ' ' {
		return false
	}

	w.buffer.WriteString("<span style=\"")
	w.buffer.WriteString(styleStr)
	w.buffer.WriteString("\">")

	return true
}

func escapeRuneHTML(c rune) string {
	switch c {
	case '&':
		return "&amp"
	case '<':
		return "&lt"
	case '>':
		return "&gt"
	default:
		panic(fmt.Sprintf("unable to escape rune: %c", c))
	}
}

func (w *HTMLWriter) writeCells() {
	for i, c := range w.cellbuf {
		if i != 0 && i%w.width == 0 {
			w.buffer.WriteRune('\n')
		}
		switch c.Ch {
		case '<', '>', '&':
			ok := w.writeCellStyle(i, c)
			w.buffer.WriteString(escapeRuneHTML(c.Ch))
			if ok {
				w.buffer.WriteString("</span>")
			}
			continue
		case '\t', '\n', 0:
			c.Ch = ' '
		}

		// TODO we should try to optimize to conflate
		// the contingent spans with the same style.
		ok := w.writeCellStyle(i, c)
		w.buffer.WriteRune(c.Ch)
		if ok {
			w.buffer.WriteString("</span>")
		}
	}
}

// Flush satisfies Writer.
func (w *HTMLWriter) Flush() (err error) {
	defStyle, _, _ := w.convertToCSS(w.defaultAttr, false)
	w.buffer.WriteString("<pre style=\"")
	w.buffer.WriteString(defStyle)
	w.buffer.WriteString("\">")

	w.writeCells()
	w.buffer.WriteString("</pre>")
	return
}

// Clear satisfies Writer.
func (w *HTMLWriter) Clear(attr Attributes) error {
	w.cellbuf = make([]Cell, w.width*w.height)
	w.buffer.Reset()
	w.defaultAttr = attr
	return nil
}

// SetCursor satisfies Writer.
func (w *HTMLWriter) SetCursor(pos Coordinates) {
	i := pos.X + pos.Y*w.width
	if i < len(w.cellbuf) {
		w.cursor = i
	} else {
		w.cursor = -1
	}
}

// HTML returns the flushed contents of this writer in HTML.
func (w *HTMLWriter) HTML() string {
	return w.buffer.String()
}
