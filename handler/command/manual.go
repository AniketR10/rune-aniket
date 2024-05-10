package command

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

const manualTemplate = `USAGE
{{ .Name }} {{ .Synopsis }}

DESCRIPTION{{ with .AliasOf }}{{ $length := len . }} {{if eq $length 1 }}
Alias of {{ index . 0 }}{{else}}
Alias of the following sequence of commands:

{{ range . }}- {{ . }}
{{ end }}{{ end }}{{end}}
{{ .Summary }}{{ with .Commands }}

SUB-COMMANDS
{{ range . }}- {{ .Name }}{{ end }}
{{ end }}`

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.New("test").Parse(manualTemplate))
}

func makeManualComponent(
	man text.CommandManual,
	frameCharSet component.FrameCharSet,
	attr term.Attributes,
) component.Responsive {
	var builder strings.Builder
	var str string
	err := writeTemplate(&builder, man)
	if err != nil {
		str = fmt.Sprintf("ERROR: build manual: %v", err)
	} else {
		str = builder.String()
	}

	minWidth := minWidthManualComponent
	if frameCharSet != (component.FrameCharSet{}) {
		minWidth -= 2
	}

	ret := component.NewResponsiveString(str, component.StringResponsiveConfig{
		NoSplitWords: true,
		StringConfig: component.StringConfig{
			Alignment:            component.SpanAlignmentCentered,
			Attributes:           attr,
			BackgroundAttributes: attr,
			PaddingVertical:      2,
			PaddingHorizontal:    2,
			MinWidth:             minWidth,
		},
	})
	return ret
}

func writeTemplate(w io.Writer, m text.CommandManual) error {
	if err := tmpl.Execute(w, m); err != nil {
		return fmt.Errorf("template execute: %v", err)
	}
	return nil
}

func getSubcommandManual(cmd text.CommandManual, input []string) (
	man text.CommandManual, ok bool,
) {
	if len(input) == 0 {
		return
	}
	name := input[0]
	for _, subCmd := range cmd.Commands {
		if subCmd.Name == name {
			textSubCmd := toTextManual(subCmd)
			subSubCmd, subSubCmdOk := getSubcommandManual(textSubCmd, input[1:])
			if subSubCmdOk {
				return subSubCmd, true
			}
			return textSubCmd, true
		}
	}
	return
}

func toTextManual(api textapi.CommandManual) text.CommandManual {
	return text.CommandManual{
		CommandManual: textapi.CommandManual{
			Name:     api.Name,
			Synopsis: api.Synopsis,
			Summary:  api.Summary,
			Commands: api.Commands,
		},
	}
}

func manualToName(man textapi.CommandManual) string {
	return man.Name
}
