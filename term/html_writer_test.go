package term

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultBackgroundCursorAtRoot = `<pre style="background:#000000;color:#FFFFFF;"><span style="background: red; animation: blinker 1s linear infinite;">%s</span>%s</pre>`
const defaultBackgroundNoCursor = `<pre style="background:#000000;color:#FFFFFF;">%s</pre>`

func expectInnerHTMLWithCursor(t *testing.T, writer *HTMLWriter, expectedHTML string) {
	require.NoError(t, writer.Flush())
	assert.Equal(t, expectedHTML, writer.HTML())
}

func expectInnerHTML(t *testing.T, writer *HTMLWriter, onCursor string, html string) {
	expectedHTML := fmt.Sprintf(defaultBackgroundCursorAtRoot, onCursor, html)
	expectInnerHTMLWithCursor(t, writer, expectedHTML)
}

func newWriterNoCursor() *HTMLWriter {
	writer := NewHTMLWriter(1, 1)
	writer.cursor = -1
	return writer
}

func TestHTMLWriter(t *testing.T) {
	t.Run("writes a cell", func(t *testing.T) {
		writer := NewHTMLWriter(1, 1)
		writer.SetCell(Coordinates{}, Cell{Ch: 'a'})
		expectInnerHTML(t, writer, "a", "")
	})

	t.Run("overwrites cells", func(t *testing.T) {
		writer := NewHTMLWriter(1, 1)
		writer.SetCell(Coordinates{}, Cell{Ch: 'a'})
		writer.SetCell(Coordinates{}, Cell{Ch: 'b'})
		expectInnerHTML(t, writer, "b", "")
	})

	t.Run("ignores out-of-bounds writes", func(t *testing.T) {
		writer := NewHTMLWriter(1, 1)
		writer.SetCell(Coordinates{Y: 1}, Cell{Ch: 'X'})
		expectInnerHTML(t, writer, " ", "")
	})

	t.Run("escapes &,>,< runes", func(t *testing.T) {
		needEscape := []rune{'&', '<', '>'}
		writer := NewHTMLWriter(len(needEscape), 1)
		for i, c := range needEscape {
			writer.SetCell(Coordinates{X: i}, Cell{Ch: c})
		}
		expectInnerHTML(t, writer, "&amp", "&lt&gt")
	})

	t.Run("writes cell attributes in CSS", func(t *testing.T) {
		tsuite := []struct {
			attr        Attribute
			expectedCSS string
		}{
			{0, "X"},            /* no style */
			{ColorDefault, "X"}, /* no style */
			{ColorWhite, "<span style=\"background:#FFFFFF;\">X</span>"}, /*white is default foreground */
			{ColorBlack, "<span style=\"color:#000000;\">X</span>"},      /* black is default background */
			{ColorBlue, "<span style=\"background:#0000FF;color:#0000FF;\">X</span>"},
			{ColorCyan, "<span style=\"background:#00FFFF;color:#00FFFF;\">X</span>"},
			{ColorGreen, "<span style=\"background:#00FF00;color:#00FF00;\">X</span>"},
			{ColorMagenta, "<span style=\"background:#FF00FF;color:#FF00FF;\">X</span>"},
			{ColorRed, "<span style=\"background:#FF0000;color:#FF0000;\">X</span>"},
			{ColorYellow, "<span style=\"background:#FFFF00;color:#FFFF00;\">X</span>"},
			{AttrBold, "<span style=\"font-weight:bold;\">X</span>"},
			{AttrUnderline, "<span style=\"text-decoration:underline;\">X</span>"},
			{AttrReverse, "<span style=\"background:#FFFFFF;color:#000000;\">X</span>"},
			{AttrBold | AttrReverse | AttrUnderline,
				"<span style=\"font-weight:bold;text-decoration:underline;background:#FFFFFF;color:#000000;\">X</span>"},
			{AttrBold | ColorCyan,
				"<span style=\"font-weight:bold;background:#00FFFF;color:#00FFFF;\">X</span>"},
		}

		for i, tcase := range tsuite {
			t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
				writer := newWriterNoCursor()
				writer.SetCell(Coordinates{}, Cell{Ch: 'X', Fg: tcase.attr, Bg: tcase.attr})
				expectedHTML := fmt.Sprintf(defaultBackgroundNoCursor, tcase.expectedCSS)
				expectInnerHTMLWithCursor(t, writer, expectedHTML)
			})
		}
	})
}

func TestHTMLWriterClear(t *testing.T) {
	writer := newWriterNoCursor()
	writer.Clear(Attributes{Bg: ColorCyan, Fg: ColorMagenta})
	expectedHTML := `<pre style="background:#00FFFF;color:#FF00FF;"> </pre>`
	expectInnerHTMLWithCursor(t, writer, expectedHTML)
}

func TestHTMLWriterSetCursor(t *testing.T) {
	t.Run("sets cursor", func(t *testing.T) {
		writer := NewHTMLWriter(2, 2)
		writer.SetCursor(Coordinates{Y: 1, X: 1})
		expectedHTML := "<pre style=\"background:#000000;color:#FFFFFF;\">  \n <span style=\"background: red; animation: blinker 1s linear infinite;\"> </span></pre>"
		expectInnerHTMLWithCursor(t, writer, expectedHTML)
	})

	t.Run("ignores out-of-bounds coordinates", func(t *testing.T) {
		writer := NewHTMLWriter(2, 2)
		writer.SetCursor(Coordinates{Y: 10, X: 1})
		expectedHTML := "<pre style=\"background:#000000;color:#FFFFFF;\">  \n  </pre>"
		expectInnerHTMLWithCursor(t, writer, expectedHTML)
	})

	t.Run("ignores negative coordinates", func(t *testing.T) {
		writer := NewHTMLWriter(2, 2)
		writer.SetCursor(Coordinates{X: -1})
		expectedHTML := "<pre style=\"background:#000000;color:#FFFFFF;\">  \n  </pre>"
		expectInnerHTMLWithCursor(t, writer, expectedHTML)
	})
}
