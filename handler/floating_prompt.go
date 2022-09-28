package handler

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

type floatingPrompt struct {
	Prompt
	back component.Background
	span component.Span
}

// FloatingPrompt returns a Prompt which floats based on a
// given SpanConfig. The returned tui.Handler should
// be used (Draw) instead of background when the prompt
// is to be draw and Resize should always be called on
// the returned tui.Handler rather than on background.
func FloatingPrompt(
	cfg PromptConfig, scfg component.SpanConfig,
) tui.Handler {
	ret := new(floatingPrompt)
	ret.Prompt.Init(cfg)
	ret.back.Init(&ret.Prompt, term.Cell{})
	ret.span.Init(&ret.back, scfg)
	return ret
}

func (f *floatingPrompt) Resize(width, height int) {
	f.span.Resize(width, height)
}

func (f *floatingPrompt) Draw(w term.Writer) {
	f.span.Draw(w)
}
