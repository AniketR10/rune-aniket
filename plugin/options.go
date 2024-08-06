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

package plugin

import (
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/vte"
)

// Option represents a Handler configuration option.
type Option func(*handlerConfig)

// WithVTEConfig instructs Handler to use
// the given vte.Config as the underlying vte.Handler config.
func WithVTEConfig(cfg vte.Config) Option {
	return func(hcfg *handlerConfig) {
		hcfg.cfg = cfg
	}
}

// WithFrame returns an option that configures
// whether Handler draws a divider between the top
// bar and the vte.
func WithFrame(frame bool) Option {
	return func(cfg *handlerConfig) {
		cfg.frame = frame
	}
}

// WithFrameCharSet returns an option that configures
// Handler to use the given character set to draw
// a divider between the top bar and the vte. It's a no-op
// if frame is set to false.
func WithFrameCharSet(charSet component.FrameCharSet) Option {
	return func(cfg *handlerConfig) {
		cfg.frameCharSet = charSet
	}
}

// WithFrameAttr returns an option that configures
// the divider attributes.
func WithFrameAttr(attr term.Attributes) Option {
	return func(cfg *handlerConfig) {
		cfg.frameAttr = attr
	}
}

// WithBarAttr returns an option that configures
// the top bar attributes.
func WithBarAttr(attr term.Attributes) Option {
	return func(cfg *handlerConfig) {
		cfg.barAttr = attr
	}
}

type handlerConfig struct {
	cfg          vte.Config
	frame        bool
	frameCharSet component.FrameCharSet
	frameAttr    term.Attributes
	barAttr      term.Attributes
}

func defaultConfig() handlerConfig {
	return handlerConfig{
		cfg:          vte.DefaultConfig(),
		frame:        true,
		frameCharSet: component.FrameCharSetDefault(),
		frameAttr:    term.Attributes{},
	}
}
