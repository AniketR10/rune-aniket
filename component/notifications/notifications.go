package notifications

import (
	"time"

	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

var _ component.Responsive = (*notification)(nil)

type notification struct {
	component.Responsive
	width        int
	height       int
	cfg          Config
	end          time.Time
	progressCell term.Cell
}

func newString(cfg Config, msg string) component.Responsive {
	strConfig := component.StringResponsiveConfig{
		NoSplitWords: true,
		StringConfig: component.StringConfig{
			Alignment:            component.SpanAlignmentCentered,
			BackgroundRune:       ' ',
			Attributes:           cfg.Attributes,
			BackgroundAttributes: cfg.BackgroundAttributes,
			PaddingHorizontal:    2,
			PaddingVertical:      2,
			FrameCharSet:         cfg.FrameCharSet,
		},
	}
	return component.StringResponsive(msg, strConfig)
}

func newNotification(level Level, msg string, cfg Config) *notification {

	var progressCell term.Cell

	switch level {
	case LevelInfo:
		progressCell = term.Cell{Ch: '═', Fg: term.ColorDefault}
	case LevelWarn:
		progressCell = term.Cell{Ch: '━', Fg: term.ColorYellow}
	case LevelError:
		progressCell = term.Cell{Ch: '━', Fg: term.ColorRed}
	case LevelSuccess:
		progressCell = term.Cell{Ch: '━', Fg: term.ColorGreen}
	default:
		panic("unknown level")
	}

	start := time.Now()
	end := start.Add(cfg.AutoClose)
	return &notification{
		Responsive:   newString(cfg, msg),
		cfg:          cfg,
		end:          end,
		progressCell: progressCell,
	}
}

func (n *notification) Resize(width, height int) {
	n.width = width
	n.height = height
	n.Responsive.Resize(width, height)
}

func (n *notification) Draw(w term.Writer) {
	n.Responsive.Draw(w)

	if n.width < 4 {
		return
	}

	remaining := time.Until(n.end)
	remainingRatio := float64(remaining) / float64(n.cfg.AutoClose)

	progressWidth := int(float64(n.width) * remainingRatio)
	progressOffset := n.width - progressWidth
	for i := 1; i < progressWidth; i++ {
		pos := term.Coordinates{X: progressOffset + i - 1, Y: n.height - 1}
		w.SetCell(pos, n.progressCell)
	}
}
