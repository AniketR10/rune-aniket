package command

import (
	"fmt"
	"io"
	"text/template"

	textapi "unstable.build/go-tui/api/text"
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
		Name:     api.Name,
		Synopsis: api.Synopsis,
		Summary:  api.Summary,
		Commands: api.Commands,
	}
}

func manualToName(man textapi.CommandManual) string {
	return man.Name
}
