// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package browser

import (
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/handler"
)

// DefaultConfig returns the default Config.
func DefaultConfig() Config {
	return Config{
		Wallpaper:             NopWallpaper(),
		FocusTabAttr:          term.Attributes{Fg: term.ColorWhite},
		NonFocusTabAttr:       term.Attributes{Fg: term.ColorRed},
		FocusTabIconAttr:      term.Attributes{},
		NonFocusTabIconAttr:   term.Attributes{},
		FocusTabHighlightAttr: term.Attributes{Fg: term.ColorYellow},
		FocusTabHighlightChar: '━',
		FrameUnionCharSet:     component.DefaultFrameUnionCharSet(),
		WindowManagerConfig:   handler.DefaultWindowManagerConfig(),
		FrameUnion:            handler.DefaultWindowManagerConfig().Frame,
		PromptConfig: PromptConfig{
			TextAttr:       term.Attributes{},
			HighlightAttr:  term.Attributes{Bg: term.ColorRed, Fg: term.ColorWhite},
			BackgroundAttr: term.Attributes{},
			MinWidth:       60,
		},
		Notifications: logNotifications{},
	}
}

// PromptConfig holds configuration for the browser's Prompt component.
type PromptConfig struct {
	TextAttr       term.Attributes
	HighlightAttr  term.Attributes
	BackgroundAttr term.Attributes
	MinWidth       int
}

// Wallpaper is a tui.Component wallpaper factory.
// NOTE: we might want to move it to the component package
// and call it a Factory.
type Wallpaper struct {
	BackgroundAttr term.Attributes
	NewComponent   func() tui.Component
}

// NopWallpaper is a Wallpaper of component.Nop.
func NopWallpaper() Wallpaper {
	return Wallpaper{
		NewComponent: func() tui.Component {
			return component.Nop()
		},
	}
}

// Config holds configuration for an browser.Component.
type Config struct {
	Notifications
	Wallpaper Wallpaper

	FocusTabAttr          term.Attributes
	NonFocusTabAttr       term.Attributes
	FocusTabIconAttr      term.Attributes
	NonFocusTabIconAttr   term.Attributes
	FocusTabHighlightAttr term.Attributes
	FocusTabHighlightChar rune
	// TabOverrideIcon, when non-zero, forces every tab icon
	// rendered by this Component to this rune, regardless of the
	// icon passed to NewTab by callers.
	TabOverrideIcon  rune
	TabBarOffset     int
	TabBarHeight     int
	TabNameSeparator string
	FrameUnion       bool
	OnTabsClick      func(int) bool

	// RightInset reserves a column of this width in cells to the right
	// of the window manager, leaving room for another UI element to
	// float over it. The tab bar still spans the full width, so the
	// reserved column starts right below it.
	RightInset int

	// DropTargetAttr styles the veil drawn over the window under the
	// cursor while files are dragged over this browser.
	DropTargetAttr term.Attributes
	// DropTargetLabelAttr styles the message centered in the
	// drop-target veil. The veil's own background is kept.
	DropTargetLabelAttr term.Attributes
	// DropTargetLabels maps a tab URI scheme to the message centered in
	// the drop-target veil. The empty key is the fallback for windows
	// whose scheme has no entry.
	DropTargetLabels map[string]string

	PromptConfig

	component.FrameUnionCharSet
	handler.WindowManagerConfig
}

type logNotifications struct {
}

func (n logNotifications) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	var l log.Level
	switch level {
	case browserapi.LevelWarn:
		l = log.WarnLevel
	case browserapi.LevelError:
		l = log.ErrorLevel
	case browserapi.LevelInfo:
		l = log.InfoLevel
	case browserapi.LevelSuccess:
		l = log.InfoLevel
	}
	log.WithField(logging.KeyClass, "notifications").Logf(l, msg, args...)
	return "", nil
}

func (n logNotifications) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return n.Notify(level, msg, args...)
}

func (n logNotifications) UpdateNotificationProgress(
	id, message string, progress, total int64,
) error {
	return nil
}
