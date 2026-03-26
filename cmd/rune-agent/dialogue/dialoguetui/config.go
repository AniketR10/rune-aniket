// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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

package dialoguetui

import (
	"time"

	"github.com/unstablebuild/blue/tui/component/markdown"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/inputbox"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// ComponentConfig holds configuration options for dialogue.Component.
type ComponentConfig struct {
	// MessagesRowConfig determines the positioning of the messages span
	// within dialogue component.
	MessagesRowConfig component.SpanConfig
	// InputRowColumns determines the width of the input row in columns,
	// from 1 to component.MaxCols. A zero value uses the default of 10.
	InputRowColumns int
	// SendMessageStringConfig determines the StringConfig of the sent messages in
	// the messages component.
	SendMessageStringConfig component.StringConfig
	// SendMessageSpanConfig allows for clients to add padding to send
	// messages and control its alignent in regards of this padding.
	SendMessageSpanConfig component.SpanConfig
	// SendMessageBottomPad adds N rows of empty space after each send
	// message in the messages list. Unlike SpanConfig.PadVertical this
	// padding lives outside the message's background, preventing the
	// background color from leaking into the gap.
	SendMessageBottomPad int

	// QueuedMessageStringConfig determines the StringConfig for queued
	// (pending) messages displayed while the agent is busy.
	QueuedMessageStringConfig component.StringConfig
	// QueuedMessageSpanConfig allows for clients to add padding to queued
	// messages and control their alignment.
	QueuedMessageSpanConfig component.SpanConfig
	// QueuedMessagePrefix is the prefix shown before each queued message.
	// Defaults to "⏳ " when empty.
	QueuedMessagePrefix string

	// ReceiveMessageStringConfig determines the StringConfig of the received
	// messages in the messages component.
	ReceiveMessageStringConfig component.StringConfig
	// ReceiveMessageSpanConfig allows for clients to add padding to receive
	// messages and control its alignent in regards of this padding.
	ReceiveMessageSpanConfig component.SpanConfig
	// ReceiveMessageBottomPad adds N rows of empty space after each
	// receive message in the messages list. Unlike SpanConfig.PadVertical
	// this padding lives outside the message's background, preventing
	// the background color from leaking into the gap.
	ReceiveMessageBottomPad int

	// ReasoningStringConfig determines the StringConfig for reasoning text
	// from reasoning models.
	ReasoningStringConfig component.StringConfig
	// ReasoningSpanConfig allows for clients to add padding to reasoning
	// text and control its alignment.
	ReasoningSpanConfig component.SpanConfig

	// ToolCallStringConfig determines the StringConfig for tool call headers
	// (e.g. "⚙ read_file").
	ToolCallStringConfig component.StringConfig
	// ToolCallSpanConfig allows for clients to add padding to tool call
	// headers and control their alignment.
	ToolCallSpanConfig component.SpanConfig
	// ToolCallArgsStringConfig determines the StringConfig for tool call
	// arguments displayed below the tool header.
	ToolCallArgsStringConfig component.StringConfig
	// ToolResultStringConfig determines the StringConfig for tool result text.
	ToolResultStringConfig component.StringConfig
	// ToolResultSpanConfig allows for clients to add padding to tool result
	// text and control its alignment.
	ToolResultSpanConfig component.SpanConfig
	// ToolResultMaxLines is the maximum number of lines of tool output shown.
	// A zero value defaults to 5.
	ToolResultMaxLines int

	// ErrorStringConfig determines the StringConfig for inline error messages.
	ErrorStringConfig component.StringConfig
	// ErrorSpanConfig allows for clients to add padding to error messages
	// and control their alignment.
	ErrorSpanConfig component.SpanConfig

	// WarningStringConfig determines the StringConfig for inline warning messages.
	WarningStringConfig component.StringConfig
	// WarningSpanConfig allows for clients to add padding to warning messages
	// and control their alignment.
	WarningSpanConfig component.SpanConfig

	// CommandOutputSpanConfig allows for clients to add padding to command output
	// and control its alignment.
	CommandOutputSpanConfig component.SpanConfig

	// ReasoningAnnotationStringConfig determines the StringConfig for the
	// reasoning toggle annotation (e.g. "ctrl-o to collapse").
	ReasoningAnnotationStringConfig component.StringConfig

	// CollapsedTreeAttr determines the attributes for tree connectors
	// (├─, └─, │) in the collapsed tool view. Defaults to gray.
	CollapsedTreeAttr term.Attributes
	// CollapsedSuccessAttr determines the attributes for the success
	// icon (✓) in the collapsed tool view. Defaults to green.
	CollapsedSuccessAttr term.Attributes
	// CollapsedErrorAttr determines the attributes for the error
	// icon (✗) in the collapsed tool view. Defaults to red.
	CollapsedErrorAttr term.Attributes
	// CollapsedToolNameAttr determines the attributes for the tool
	// name in the collapsed tool view. Defaults to bold.
	CollapsedToolNameAttr term.Attributes
	// CollapsedToolArgsAttr determines the attributes for tool
	// arguments and summary in the collapsed tool view. Defaults to olive.
	CollapsedToolArgsAttr term.Attributes

	// PromptToolCallStringConfig determines the StringConfig for the
	// ask_user_question tool call header ("? ask_user_question").
	PromptToolCallStringConfig component.StringConfig
	// CollapsedDroppedAttr determines the attributes for the icon
	// of a tool call whose result was dropped from history.
	// Defaults to gray.
	CollapsedDroppedAttr term.Attributes
	// CollapsedPromptAttr determines the attributes for the prompt
	// icon (?) in the collapsed tool view. Defaults to aqua.
	CollapsedPromptAttr term.Attributes
	// CollapsedMemoryAttr determines the attributes for the memory
	// icon (󰍛) in the collapsed tool view. Defaults to purple.
	CollapsedMemoryAttr term.Attributes
	// MemoryIDStringConfig determines the StringConfig for the memory
	// ID text displayed in both expanded and collapsed views.
	MemoryIDStringConfig component.StringConfig
	// MemoryContentStringConfig determines the StringConfig for the
	// memory content text displayed in expanded view.
	MemoryContentStringConfig component.StringConfig

	// SelectionConfig holds styling for inline Selection prompts.
	SelectionConfig SelectionConfig

	// MarkdownConfig, when non-nil, configures the markdown renderer
	// used for received messages. A nil value uses markdown.DefaultConfig().
	MarkdownConfig *markdown.Config

	// DurationPrecision, when positive, truncates tool call durations
	// to this precision (e.g. time.Second shows "1s" instead of "1.234s").
	// Zero uses the default adaptive rounding.
	DurationPrecision time.Duration

	// StartCollapsed, when true, causes tool call turns to render
	// in collapsed tree view by default. The user can toggle with ctrl-o.
	StartCollapsed bool

	// BackgroundColor, when set to a valid color, fills every cell with
	// this background color before drawing content.
	BackgroundColor tcell.Color

	InputBox InputBoxConfig
}

// InputBoxConfig holds configuration optons for a inputbox.Handler.
type InputBoxConfig struct {
	Placeholder       string
	PlaceholderConfig component.StringResponsiveConfig
	FrameAttr         term.Attributes
	FrameCharSet      component.FrameCharSet
	ContentAttr       term.Attributes
	WordCompleter     inputbox.WordCompleter
}

// Options returns a slice of inputobx.Option, which can
// be passed in a call to inputbo.New.
func (i InputBoxConfig) Options() (ret []inputbox.Option) {
	if i.Placeholder != "" {
		if i.PlaceholderConfig == (component.StringResponsiveConfig{}) {
			ret = append(ret, inputbox.WithPlaceholderText(i.Placeholder))
		} else {
			ret = append(ret, inputbox.WithPlaceholder(i.Placeholder, i.PlaceholderConfig))
		}
	}
	if i.ContentAttr != (term.Attributes{}) {
		ret = append(ret, inputbox.WithAttributes(i.ContentAttr))
	}
	if i.WordCompleter != nil {
		ret = append(ret, inputbox.WithWordCompleter(i.WordCompleter))
		ret = append(ret, inputbox.WithTabStyle(inputbox.TabPrints))
	}
	return
}
