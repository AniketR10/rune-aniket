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

package command

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"text/template"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/component/markdown"
)

// Manual represents a command's manual and documentation.
type Manual struct {
	Name string

	// Summary is a short 80-100 character description.
	Summary string

	// Synopsis is a single line synopsis of how
	// this CLI is to be used. It should ONLY include
	// the semantic information about how arguments are parsed.
	//
	// Example: [<options>] [<revision-range>] [[--] <path>...]
	Synopsis string

	// Commands is a list of accepted commands or nil
	// if no commands are expected.
	Commands []Manual

	// AliasOf defines this command as an alias of the
	// given command or sequence of commands.
	AliasOf []string
}

const manualStringTemplate = `USAGE
{{ .Name }} {{ .Synopsis }}

DESCRIPTION{{ with .AliasOf }}{{ $length := len . }} {{if eq $length 1 }}
Alias of {{ index . 0 }}{{else}}
Alias of the following sequence of commands:

{{ range . }}- {{ . }}
{{ end }}{{ end }}{{end}}
{{ .Summary }}{{ with .Commands }}

SUB-COMMANDS{{ range . }}
    - {{ .Name }}{{ end }}
{{ end }}`

const manualMarkdownTemplate = "## Usage\n" +
	"`{{ .Name }} {{ .Synopsis }}`\n" +
	"## Description\n" +
	"{{ with .AliasOf }}{{ $length := len . }} {{if eq $length 1 }}\n" +
	"Alias of {{ index . 0 }}{{else}}\n" +
	"Alias of the following sequence of commands:\n" +
	"\n" +
	"{{ range . }}- {{ . }}\n" +
	"{{ end }}{{ end }}{{end}}\n" +
	"{{ .Summary }}{{ with .Commands }}\n" +
	"## Subcommands {{ range . }}\n" +
	" - {{ .Name }}{{ end }}\n" +
	"{{ end }}\n"

var (
	strTemplate      *template.Template
	markdownTemplate *template.Template
)

func init() {
	strTemplate = template.Must(template.New("command-str").
		Parse(manualStringTemplate))
	markdownTemplate = template.Must(template.New("command-md").
		Parse(manualMarkdownTemplate))
}

func (p *Prompt) makeManualComponent(
	man Manual,
	frameCharSet component.FrameCharSet,
	attr term.Attributes,
) (ret component.Responsive) {
	var builder strings.Builder
	var str string
	var tmpl *template.Template
	if p.config.NoMarkdown {
		tmpl = strTemplate
	} else {
		tmpl = markdownTemplate
	}
	err := writeTemplate(&builder, man, tmpl)
	if err != nil {
		str = fmt.Sprintf("ERROR: build manual: %v", err)
	} else {
		str = builder.String()
	}

	cfg := markdown.DefaultConfig()
	cfg.HeaderPrefix = false
	cfg.ParagraphSpacing = 0
	cfg.InlineCode = term.Attributes{
		Fg: term.ColorSilver,
		Bg: term.ColorGray,
	}
	markdown, err := markdown.NewWithConfig(str, cfg)
	if err != nil || p.config.NoMarkdown {
		// this could be either a programmer error or
		// a third-party command that's injecting incorrect markdown into
		// the command manual.
		if err != nil {
			slog.Error("parse manual markdown", "error", err)
		}
		minWidth := minWidthManualComponent
		if frameCharSet != (component.FrameCharSet{}) {
			minWidth -= 2
		}
		ret = component.NewResponsiveString(str, component.StringResponsiveConfig{
			NoSplitWords: true,
			StringConfig: component.StringConfig{
				Alignment:            component.AlignmentCentered,
				Attributes:           attr,
				BackgroundAttributes: attr,
				PaddingVertical:      2,
				MinWidth:             minWidth,
			},
		})
	} else {
		ret = component.FuncResponsive(markdown, func(int) int {
			// content won't wrap
			_, height := markdown.Dimensions()
			return height
		})
	}
	return ret
}

func writeTemplate(w io.Writer, m Manual, tmpl *template.Template) error {
	if err := tmpl.Execute(w, m); err != nil {
		return fmt.Errorf("template execute: %v", err)
	}
	return nil
}

func getSubcommandManual(cmd Manual, input []string) (
	man Manual, ok bool,
) {
	if len(input) == 0 {
		return
	}
	name := input[0]
	for _, subCmd := range cmd.Commands {
		if subCmd.Name == name {
			textSubCmd := subCmd
			subSubCmd, subSubCmdOk := getSubcommandManual(textSubCmd, input[1:])
			if subSubCmdOk {
				return subSubCmd, true
			}
			return textSubCmd, true
		}
	}
	return
}

func manualToName(man Manual) string {
	return man.Name
}
