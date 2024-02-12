package notifications

import (
	"time"

	"github.com/ernestrc/tcell/v3"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

var _ component.Responsive = (*notificationComp)(nil)

type notificationComp struct {
	component.Responsive
	cfg          Config
	cancel       func()
	width        int
	height       int
	duration     time.Duration
	end          time.Time
	pausedAt     time.Time
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
			FrameCharSet:         cfg.FrameCharSet,
			MinWidth:             cfg.Width,
		},
	}
	return component.NewResponsiveString(msg, strConfig)
}

func newNotification(
	level Level, msg string, cfg Config, duration time.Duration,
	cancel func(),
) *notificationComp {

	var progressCell term.Cell
	progressCell.Attributes = cfg.BackgroundAttributes

	switch level {
	case LevelInfo:
		progressCell.Ch = '═'
		progressCell.Fg = tcell.ColorDefault
	case LevelWarn:
		progressCell.Ch = '━'
		progressCell.Fg = tcell.ColorYellow
	case LevelError:
		progressCell.Ch = '━'
		progressCell.Fg = tcell.ColorRed
	case LevelSuccess:
		progressCell.Ch = '━'
		progressCell.Fg = tcell.ColorGreen
	default:
		panic("unknown level")
	}

	progressCell.Width = 1

	start := time.Now()
	end := start.Add(duration)
	return &notificationComp{
		Responsive:   newString(cfg, msg),
		cfg:          cfg,
		cancel:       cancel,
		duration:     duration,
		end:          end,
		progressCell: progressCell,
	}
}

func (n *notificationComp) Resize(width, height int) {
	n.width = width
	n.height = height
	n.Responsive.Resize(width, height)
}

func (n *notificationComp) Draw(w term.Writer) {
	n.Responsive.Draw(w)

	if !n.cfg.ProgressBar || n.width < 4 {
		return
	}

	remaining := time.Until(n.end)
	if !n.pausedAt.IsZero() {
		remaining = n.end.Sub(n.pausedAt)
	}

	remainingRatio := float64(remaining) / float64(n.duration)

	progressWidth := int(float64(n.width) * remainingRatio)
	progressOffset := n.width - progressWidth
	for i := 1; i < progressWidth; i++ {
		pos := term.Coordinates{X: progressOffset + i - 1, Y: n.height - 1}
		w.SetCell(pos, n.progressCell)
	}
}
