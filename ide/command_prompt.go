// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/handler/command"
)

// commandPromptShaderConfig bundles the gating flag and the optional
// RadarFrameParams overrides for the command-prompt shader. Unset
// overrides (colorSet == false, angularWidth == 0, cycles == 0) mean
// "keep the default from glslshader.DefaultRadarFrameParams".
type commandPromptShaderConfig struct {
	enabled      bool
	color        term.Color
	colorSet     bool
	angularWidth float64
	cycles       int
}

// commandPromptSeparatorCharset configures the four glyphs that
// stitch the prompt's manual/list separator row into the surrounding
// frame. From left to right within the separator row they are placed
// at columns 0, 1, W-2, W-1 of the floating window. The defaults
// produce a single-line T-junction stitch (├ ─ ─ ┤).
type commandPromptSeparatorCharset struct {
	Left            rune
	HorizontalLeft  rune
	HorizontalRight rune
	Right           rune
}

// defaultCommandPromptSeparatorCharset returns the stitch glyphs used
// when no override is configured.
func defaultCommandPromptSeparatorCharset() commandPromptSeparatorCharset {
	return commandPromptSeparatorCharset{
		Left:            '├',
		HorizontalLeft:  '─',
		HorizontalRight: '─',
		Right:           '┤',
	}
}

// commandPromptConfig groups every command-prompt-only knob the ex
// constructor consumes. It is intentionally kept out of text.Config
// because none of these settings need to round-trip through the
// editor's text configuration.
type commandPromptConfig struct {
	shader    commandPromptShaderConfig
	separator commandPromptSeparatorCharset
	// keyBindingHint returns the long-form key label bound to a full
	// command line, or "" when unbound. nil disables the hints.
	keyBindingHint     func(commandLine string) string
	keyBindingHintAttr term.Attributes
	// keyBindingHintFocusAttr styles the key hint on the focused row.
	keyBindingHintFocusAttr term.Attributes
}

func newCommandPromptHandler(
	cmd *command.Prompt, e *ex, onClose func() error,
) browser.Floating {
	padded := handler.NewSpan(cmd, component.SpanConfig{
		PadHorizontal:    2,
		ContentAlignment: component.AlignmentCentered,
	})
	bg := component.NewBackground(padded, term.Cell{
		Attributes: term.Attributes{
			Bg:    e.config.CommandOverlay.ElementAttr.Bg,
			Attrs: e.config.CommandOverlay.ElementAttr.Attrs,
		},
	})
	if !e.config.WindowManagerConfig.Frame {
		return browser.FuncFloating(
			browser.FuncHandler(
				handler.WithComponent(padded, bg),
				onClose,
			),
			padded.Dimensions,
		)
	}
	framed := component.NewFrame(bg)
	framed.FrameCharSet = e.config.FrameCharSet
	framed.Attributes = e.config.FrameAttr

	stitched := &promptStitcher{
		cmd:       cmd,
		padded:    padded,
		framed:    framed,
		separator: e.commandPromptCfg.separator,
	}
	return browser.FuncFloating(
		browser.FuncHandler(stitched, onClose),
		framed.Dimensions,
	)
}

type promptStitcher struct {
	cmd       *command.Prompt
	padded    *handler.Span
	framed    *component.Frame
	separator commandPromptSeparatorCharset
	width     int
	height    int
}

var _ tui.Handler = (*promptStitcher)(nil)

func (s *promptStitcher) Resize(width, height int) {
	s.width, s.height = width, height
	s.framed.Resize(width, height)
}

func (s *promptStitcher) Draw(w term.Writer) {
	s.framed.Draw(w)
	if s.width < 3 || s.height < 3 {
		return
	}
	sepY, ok := s.cmd.SeparatorY()
	if !ok {
		return
	}
	y := sepY + 1
	if y <= 0 || y >= s.height-1 {
		return
	}
	w.SetCell(term.Coordinates{X: 0, Y: y}, term.Cell{
		Ch:         s.separator.Left,
		Attributes: s.framed.Attributes,
	})
	w.SetCell(term.Coordinates{X: 1, Y: y}, term.Cell{
		Ch:         s.separator.HorizontalLeft,
		Attributes: s.framed.Attributes,
	})
	w.SetCell(term.Coordinates{X: s.width - 2, Y: y}, term.Cell{
		Ch:         s.separator.HorizontalRight,
		Attributes: s.framed.Attributes,
	})
	w.SetCell(term.Coordinates{X: s.width - 1, Y: y}, term.Cell{
		Ch:         s.separator.Right,
		Attributes: s.framed.Attributes,
	})
}

func (s *promptStitcher) Handle(ev term.Event) (bool, bool) {
	if ev.Type == term.EventMouse {
		// The frame border occupies row 0 / W-1 and column 0 / H-1
		// of the outer window. Translate mouse coordinates into
		// the inner content space the padded handler expects, and
		// clamp into bounds for press/release on border cells.
		ev.MouseX = clampMouseAxis(ev.MouseX-1, s.width-2)
		ev.MouseY = clampMouseAxis(ev.MouseY-1, s.height-2)
	}
	return s.padded.Handle(ev)
}

func (s *promptStitcher) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	pos, style, show := s.padded.Cursor()
	pos.X++
	pos.Y++
	return pos, style, show
}

func (s *promptStitcher) Selection() (string, bool) {
	return s.padded.Selection()
}

func clampMouseAxis(v, max int) int {
	if v < 0 {
		return 0
	}
	if max < 0 {
		return 0
	}
	if v > max {
		return max
	}
	return v
}
