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
	"time"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

// DefaultConfig returns the default Config.
func DefaultConfig() Config {
	return Config{
		Wallpaper:           NopWallpaper(),
		FocusTabAttr:        term.Attributes{Fg: tcell.ColorWhite},
		NonFocusTabAttr:     term.Attributes{Fg: tcell.ColorRed},
		FrameUnionCharSet:   component.DefaultFrameUnionCharSet(),
		WindowManagerConfig: handler.DefaultWindowManagerConfig(),
		PromptConfig: PromptConfig{
			TextAttr:       term.Attributes{},
			HighlightAttr:  term.Attributes{Bg: tcell.ColorRed, Fg: tcell.ColorWhite},
			BackgroundAttr: term.Attributes{},
			MinWidth:       60,
		},
		Notifications: notifications.Config{
			AutoClose:            5 * time.Second,
			ProgressBar:          true,
			Width:                50,
			Attributes:           term.Attributes{},
			BackgroundAttributes: term.Attributes{},
			FrameCharSet:         component.FrameCharSetDefault(),
			Interrupter:          term.NopInterrupter(),
		},
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
	Wallpaper Wallpaper

	FocusTabAttr     term.Attributes
	NonFocusTabAttr  term.Attributes
	TabBarOffset     int
	TabBarHeight     int
	TabNameSeparator string

	PromptConfig

	component.FrameUnionCharSet
	handler.WindowManagerConfig
	Notifications notifications.Config
}
