// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package notifications

import (
	"math"
	"time"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

var _ component.Responsive = (*notificationComp)(nil)

type notificationComp struct {
	component.Responsive
	cfg                    Config
	cancel                 func()
	width                  int
	height                 int
	duration               time.Duration
	manualProgress         int64
	manualProgressTotal    int64
	end                    time.Time
	pausedAt               time.Time
	progressCellStart      term.Cell
	progressCellEnd        term.Cell
	progressCellCurrent    term.Cell
	progressCellCurrentTip term.Cell
	progressCellRemain     term.Cell
}

func newString(cfg Config, msg string) component.Responsive {
	strConfig := component.StringResponsiveConfig{
		NoSplitWords: true,
		StringConfig: component.StringConfig{
			Alignment:            component.AlignmentCentered,
			BackgroundRune:       ' ',
			Attributes:           cfg.Attributes,
			BackgroundAttributes: cfg.BackgroundAttributes,
			PaddingHorizontal:    2,
			FrameCharSet:         cfg.FrameCharSet,
			MinWidth:             cfg.Width,
		},
	}
	return component.NewResponsiveString(msg, strConfig)
}

func newNotification(
	level Level, msg string, cfg Config, duration time.Duration,
	cancel func(),
) component.Responsive {

	// template for each progress rune
	var progressCell term.Cell
	switch level {
	case LevelInfo:
		progressCell.Attrs = cfg.ColorInfo.Attrs
		progressCell.Fg = cfg.ColorInfo.Fg
		progressCell.Bg = cfg.ColorInfo.Bg
	case LevelSuccess:
		progressCell.Attrs = cfg.ColorSuccess.Attrs
		progressCell.Fg = cfg.ColorSuccess.Fg
		progressCell.Bg = cfg.ColorSuccess.Bg
	case LevelWarn:
		progressCell.Attrs = cfg.ColorWarning.Attrs
		progressCell.Fg = cfg.ColorWarning.Fg
		progressCell.Bg = cfg.ColorWarning.Bg
	case LevelError:
		progressCell.Attrs = cfg.ColorError.Attrs
		progressCell.Fg = cfg.ColorError.Fg
		progressCell.Bg = cfg.ColorError.Bg
	default:
		panic("unknown level")
	}
	progressCell.Width = 1

	progressCellStart := progressCell
	progressCellStart.Ch = cfg.ProgressRunes.Start
	progressCellEnd := progressCell
	progressCellEnd.Ch = cfg.ProgressRunes.End
	progressCellRemain := progressCell
	progressCellRemain.Ch = cfg.ProgressRunes.Remain
	progressCellCurrent := progressCell
	progressCellCurrent.Ch = cfg.ProgressRunes.Current
	progressCellCurrentTip := progressCell
	progressCellCurrentTip.Ch = cfg.ProgressRunes.CurrentTip

	start := time.Now()
	end := start.Add(duration)
	return &notificationComp{
		Responsive:             newString(cfg, msg),
		cfg:                    cfg,
		cancel:                 cancel,
		duration:               duration,
		end:                    end,
		progressCellStart:      progressCellStart,
		progressCellEnd:        progressCellEnd,
		progressCellRemain:     progressCellRemain,
		progressCellCurrent:    progressCellCurrent,
		progressCellCurrentTip: progressCellCurrentTip,
	}
}

func (n *notificationComp) Resize(width, height int) {
	n.width = width
	n.height = height
	n.Responsive.Resize(width, height)
}

func (n *notificationComp) Draw(w term.Writer) {
	n.Responsive.Draw(w)

	if !n.cfg.ProgressBar {
		return
	}

	if n.width >= 3 && n.height >= 3 {
		attrs := term.Attributes{
			Attrs: n.progressCellStart.Attrs,
			Fg:    n.progressCellStart.Fg,
			Bg:    n.progressCellStart.Bg,
		}
		component.DrawFrame(w, n.cfg.FrameCharSet, attrs, n.width-1, n.height-1)
	}

	remaining := time.Until(n.end)
	if !n.pausedAt.IsZero() {
		remaining = n.end.Sub(n.pausedAt)
	}
	total := int64(n.duration)
	current := total - int64(remaining)
	size := n.width

	if n.manualProgressTotal != 0 {
		current = n.manualProgress
		total = n.manualProgressTotal
	}
	currCount := int(math.Ceil(
		float64(current) / float64(total) * float64(size),
	))
	if size < currCount {
		return
	}
	remaCount := size - currCount

	start := term.Coordinates{X: 0, Y: n.height - 1}
	w.SetCell(start, n.progressCellStart)

	for x := 1; x < currCount; x++ {
		pos := term.Coordinates{X: x, Y: n.height - 1}
		w.SetCell(pos, n.progressCellCurrent)
	}
	if currCount != 0 {
		tip := term.Coordinates{X: currCount, Y: n.height - 1}
		w.SetCell(tip, n.progressCellCurrentTip)
	}

	for x := currCount + 1; x < currCount+remaCount-1; x++ {
		pos := term.Coordinates{X: x, Y: n.height - 1}
		w.SetCell(pos, n.progressCellRemain)
	}

	end := term.Coordinates{X: size - 1, Y: n.height - 1}
	w.SetCell(end, n.progressCellEnd)
}
