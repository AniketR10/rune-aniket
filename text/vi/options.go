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

package vi

import (
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/clipboard"
	"unstable.build/go-tui/term"
)

// viConfig holds configuration for Vi.
type viConfig struct {
	attr                 term.Attributes
	resAttr              term.Attributes
	barAttr              term.Attributes
	clipboard            clipboard.Register
	scheduleNextTick     func(func()) bool
	defaultRegister      string
	superimposedMessages bool
	debug                bool
	wrap                 bool
	cursorCorrections    bool
	barHidden            bool
	skipNulls            bool
	enableInitialFolds   bool
	enableAuxBar         bool
	enableAuxBarFolds    bool
}

// defaultviHandlerImplConfig is a sane configuration defaults for viHandlerImpl.
func defaultviHandlerImplConfig() viConfig {
	return viConfig{
		resAttr: term.Attributes{
			Attrs: tcell.AttrReverse,
		},
		clipboard: clipboard.NewInMemory(),
		scheduleNextTick: func(fn func()) bool {
			fn()
			return true
		},
		defaultRegister:   clipboard.DefaultRegisterID,
		skipNulls:         true,
		cursorCorrections: true,
	}
}

// Option represents a Vi handler configuration option.
type Option func(*viConfig)

// WithResAttr sets the search result cell attributes to be rendered.
func WithResAttr(attr term.Attributes) Option {
	return func(cfg *viConfig) {
		cfg.resAttr = attr
	}
}

// WithScheduleNextTick defines the function to schedule and serializes asynchronous work.
func WithScheduleNextTick(fn func(func()) bool) Option {
	return func(cfg *viConfig) {
		cfg.scheduleNextTick = fn
	}
}

// WithAuxiliaryBar determines whether to draw an auxiliary bar on the left or not.
func WithAuxiliaryBar(enabled bool) Option {
	return func(cfg *viConfig) {
		cfg.enableAuxBar = enabled
	}
}

// WithAuxiliaryBarFolds determines whether to draw folds at the auxiliary.
func WithAuxiliaryBarFolds(enabled bool) Option {
	return func(cfg *viConfig) {
		cfg.enableAuxBarFolds = enabled
	}
}

// WithBarAttr sets the command bar cell attributes to be rendered.
func WithBarAttr(attr term.Attributes) Option {
	return func(cfg *viConfig) {
		cfg.barAttr = attr
	}
}

// WithBarHidden sets the command bar to be hidden.
func WithBarHidden(hide bool) Option {
	return func(cfg *viConfig) {
		cfg.barHidden = hide
	}
}

// WithSuperimposedMessages changes the behaviour to instead of drawing
// a bottom bar permanently on which messages are written,
// messages are superimposed on the last row of the scroll content.
//
// The default value is false, so a full bar is drawn.
func WithSuperimposedMessages(value bool) Option {
	return func(cfg *viConfig) {
		cfg.superimposedMessages = value
	}
}

// WithAttr sets the default cell attributes to be rendered.
func WithAttr(attr term.Attributes) Option {
	return func(cfg *viConfig) {
		cfg.attr = attr
	}
}

// WithClipboard sets the editor.Clipboard implementation to use.
func WithClipboard(clip clipboard.Register) Option {
	return func(cfg *viConfig) {
		cfg.clipboard = clip
	}
}

// WithDebug disables cursor position correction to aid with cursor debugging.
func WithDebug(debug bool) Option {
	return func(cfg *viConfig) {
		cfg.debug = debug
	}
}

// WithWrap enables or disables word wrapping mode.
func WithWrap(wrap bool) Option {
	return func(cfg *viConfig) {
		cfg.wrap = wrap
	}
}

// WithCursorCorrections enables or disables cursor out of bounds corrections.
// By default it's enabled, unless this option is passed; when disabled, clients
// must manage it themselves.
//
// This behaviour is force disabled if WithDebug Option is used.
func WithCursorCorrections(enabled bool) Option {
	return func(cfg *viConfig) {
		cfg.cursorCorrections = enabled
	}
}

// WithAutoSkipNullCells determines whether vi should automatically
// shift the cursor on top a null cell (no content) in
// normal, yank, search, g and delete modes. Default is on.
//
// This behaviour is force disabled if WithDebug Option is used,
// or if WithCursorCorrections disables cursor corrections.
func WithAutoSkipNullCells(skip bool) Option {
	return func(cfg *viConfig) {
		cfg.skipNulls = skip
	}
}

// WithHideInitialFolds determines whether to hide the initial folds
// determined by the language query.
func WithHideInitialFolds(enabled bool) Option {
	return func(cfg *viConfig) {
		cfg.enableInitialFolds = enabled
	}
}
