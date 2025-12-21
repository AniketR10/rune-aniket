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

package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

// Prompt implements a prompt / question with options component.
type Prompt struct {
	optComp      []WithAttributes
	effective    tui.Component
	messageRow   *Row
	cfg          PromptConfig
	makeOptionFn func(msg string, cfg PromptConfig) responsiveWithAttributes
}

// PromptConfig holds configuration for initializing a Prompt.
type PromptConfig struct {
	Message string
	Options []string
	Frame   FrameCharSet
	// AspectRatio of the floating prompt. By default DefaultAspectRatio is used.
	AspectRatio          float64
	BackgroundAttributes term.Attributes
	MinWidth             int
}

type responsiveWithAttributes interface {
	Responsive
	WithAttributes
}

// Init initializes this prompt with cfg. Note that PromptConfig.Options must
// always contain at least one option and PromptConfig.Message must not be empty.
// If one of these two rules is violated this method panics.
func (p *Prompt) Init(cfg PromptConfig) {
	p.init(func(msg string, cfg PromptConfig) responsiveWithAttributes {
		return NewResponsiveString(msg, StringResponsiveConfig{
			NoSplitWords: true,
			StringConfig: StringConfig{
				Alignment:            SpanAlignmentCentered,
				FrameCharSet:         cfg.Frame,
				BackgroundAttributes: p.cfg.BackgroundAttributes,
				Attributes:           term.Attributes{Bg: p.cfg.BackgroundAttributes.Bg},
				PaddingHorizontal:    2,
			},
		})
	}, cfg)
}

// NewPrompt allocates storage for a new prompt and initializes it.
// See Init for more details.
func NewPrompt(cfg PromptConfig) *Prompt {
	p := new(Prompt)
	p.Init(cfg)
	return p
}

// SetOptionAttr sets the attributes of option at index i.
func (p *Prompt) SetOptionAttr(i int, attr term.Attributes) {
	p.optComp[i].SetAttr(attr)
}

// Resize satisfies tui.Component
func (p *Prompt) Resize(width, height int) {
	p.effective.Resize(width, height)
}

// Draw satisfies tui.Component
func (p *Prompt) Draw(w term.Writer) {
	p.effective.Draw(w)
}

// Dimensions satisfies Floating.
func (p *Prompt) Dimensions() (int, int) {
	width, height := p.effective.(Floating).Dimensions()
	if width < p.cfg.MinWidth {
		width = p.cfg.MinWidth
	}
	return width, height
}

func (p *Prompt) initOptions(cfg PromptConfig, row *Row) {
	if len(cfg.Options) == 0 {
		panic("Options should be greater than zero")
	}

	// we want to divide the space evently between options
	// but longer options should take more space, so paddings
	// are not off
	var totalLength, totalColumns int
	for _, opt := range cfg.Options {
		totalLength += len(opt)
	}

	columns := make([]int, len(cfg.Options))
	for i := range cfg.Options {
		columns[i] = int(float64(MaxCols) / float64(len(cfg.Options)))
		totalColumns += columns[i]
	}

	// allow for Init to be used as reset
	p.optComp = make([]WithAttributes, 0, len(cfg.Options))

	remainder := MaxCols - totalColumns
	for i, opt := range cfg.Options {
		compi := p.makeOptionFn(opt, cfg)
		p.optComp = append(p.optComp, compi)
		effectiveCols := columns[i]
		if remainder > 0 {
			effectiveCols++
			remainder--
		}
		row.AddComponent(NewFloatingResponsive(compi, cfg.AspectRatio), effectiveCols)
	}
}

func (p *Prompt) init(
	makeOptionFn func(msg string, cfg PromptConfig) responsiveWithAttributes,
	cfg PromptConfig,
) {
	if cfg.Message == "" {
		panic("Message cannot be empty")
	}
	if cfg.AspectRatio == 0 {
		cfg.AspectRatio = DefaultAspectRatio
	}
	p.cfg = cfg
	p.makeOptionFn = makeOptionFn

	container := NewContainer()

	messageResponsive := NewResponsiveString(cfg.Message, StringResponsiveConfig{
		NoSplitWords: true,
		StringConfig: StringConfig{
			PaddingVertical:      4,
			PaddingHorizontal:    4,
			Alignment:            SpanAlignmentCentered,
			BackgroundAttributes: p.cfg.BackgroundAttributes,
			Attributes:           term.Attributes{Bg: p.cfg.BackgroundAttributes.Bg},
		}})

	p.messageRow = container.AddRow()
	p.messageRow.AddComponent(
		NewFloatingResponsive(messageResponsive, cfg.AspectRatio), MaxCols)

	optionsRow := container.AddRow()

	p.initOptions(cfg, optionsRow)

	p.effective = container

	p.effective = NewBackground(container, term.Cell{
		Attributes: p.cfg.BackgroundAttributes,
	})
}
