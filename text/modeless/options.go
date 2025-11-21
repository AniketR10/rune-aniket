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

package modeless

import (
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/clipboard"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

// modelessConfig holds configuration for Editor.
type modelessConfig struct {
	attr               term.Attributes
	resAttr            term.Attributes
	barAttr            term.Attributes
	registry           text.WorkspaceCommandRegistry
	wrap               bool
	enableInitialFolds bool
	enableAuxBar       bool
	auxBarConfig       text.AuxBarConfig
	enableGitBar       bool
	commandBar         bool
	gitBarConfig       text.GitBarConfig
	workspace          workspaceapi.URI
	clipboard          clipboard.Register
	scheduleNextTick   func(fn func()) bool
}

// defaultmodelessHandlerImplConfig is a sane configuration defaults for modelessHandlerImpl.
func defaultConfig() modelessConfig {
	return modelessConfig{
		resAttr: term.Attributes{
			Attrs: tcell.AttrReverse,
		},
		commandBar: true,
		clipboard:  clipboard.NewInMemory(),
		scheduleNextTick: func(fn func()) bool {
			fn()
			return true
		},
	}
}

// Option represents a Editor configuration option.
type Option func(*modelessConfig)

// WithResAttr sets the search result cell attributes to be rendered.
func WithResAttr(attr term.Attributes) Option {
	return func(cfg *modelessConfig) {
		cfg.resAttr = attr
	}
}

// WithWorkspaceCommandRegistry sets the command registry to register workspace-level
// commands.
func WithWorkspaceCommandRegistry(
	cwd workspaceapi.URI, registry text.WorkspaceCommandRegistry,
) Option {
	return func(cfg *modelessConfig) {
		cfg.registry = registry
		cfg.workspace = cwd
	}
}

// WithScheduleNextTick defines the function to schedule and serializes asynchronous work.
func WithScheduleNextTick(fn func(func()) bool) Option {
	return func(cfg *modelessConfig) {
		cfg.scheduleNextTick = fn
	}
}

// WithAuxiliaryBar determines whether to draw an auxiliary bar on the left or not.
func WithAuxiliaryBar(enabled bool, config text.AuxBarConfig) Option {
	return func(cfg *modelessConfig) {
		cfg.enableAuxBar = enabled
		cfg.auxBarConfig = config
	}
}

// WithGitBar determines whether to render the changes between the
// open file's worktree and HEAD.
func WithGitBar(enabled bool, config text.GitBarConfig) Option {
	return func(cfg *modelessConfig) {
		cfg.enableGitBar = enabled
		cfg.gitBarConfig = config
	}
}

// WithBarAttr sets the command bar cell attributes to be rendered.
func WithBarAttr(attr term.Attributes) Option {
	return func(cfg *modelessConfig) {
		cfg.barAttr = attr
	}
}

// WithCommandBar enables or disables the command bar.
func WithCommandBar(enabled bool) Option {
	return func(cfg *modelessConfig) {
		cfg.commandBar = enabled
	}
}

// WithAttr sets the default cell attributes to be rendered.
func WithAttr(attr term.Attributes) Option {
	return func(cfg *modelessConfig) {
		cfg.attr = attr
	}
}

// WithClipboard sets the editor.Clipboard implementation to use.
func WithClipboard(clip clipboard.Register) Option {
	return func(cfg *modelessConfig) {
		cfg.clipboard = clip
	}
}

// WithWrap enables or disables word wrapping mode.
func WithWrap(wrap bool) Option {
	return func(cfg *modelessConfig) {
		cfg.wrap = wrap
	}
}

// WithHideInitialFolds determines whether to hide the initial folds
// determined by the language query.
func WithHideInitialFolds(enabled bool) Option {
	return func(cfg *modelessConfig) {
		cfg.enableInitialFolds = enabled
	}
}
