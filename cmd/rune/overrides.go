// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

// Bootstrap config-format choices.
const (
	configFormatYAML = "yaml"
	configFormatStar = "star"
)

// Bootstrap BYOE editor preset keys.
const (
	byoePresetVim   = "vim"
	byoePresetNvim  = "nvim"
	byoePresetHelix = "helix"
	byoePresetKak   = "kak"
	byoePresetEmacs = "emacs"
)

// byoePreset describes how to launch a third-party TUI editor and how to
// drive it to a specific cursor location once it is open. Both fields are
// substituted into the BYOE override templates as {{.Command}} and
// {{.Goto}}.
type byoePreset struct {
	Command string
	Goto    string
	Quit    string
}

// byoePresets lists the editor command and goto sequence for each built-in
// BYOE preset. Keys match the byoePreset* constants.
var byoePresets = map[string]byoePreset{
	byoePresetVim: {
		Command: `vim "+call cursor({line}, {col})" {file}`,
		Goto:    `<esc>:{line}<enter>{col}|`,
		Quit:    `<esc>:qa!<enter>`,
	},
	byoePresetNvim: {
		Command: `nvim "+call cursor({line}, {col})" {file}`,
		Goto:    `<esc>:{line}<enter>{col}|`,
		Quit:    `<esc>:qa!<enter>`,
	},
	byoePresetHelix: {
		Command: `hx {file}:{line}:{col}`,
		Goto:    `<esc>:goto<space>{line}<enter>`,
		Quit:    `<esc>:q!<enter>`,
	},
	byoePresetKak: {
		Command: `kak {file} +{line}:{col}`,
		Goto:    `<esc>:edit<space>-existing<space>{file}<space>{line}<space>{col}<enter>`,
		Quit:    `<esc>:q!<enter>`,
	},
	byoePresetEmacs: {
		Command: `emacs -nw +{line}:{col} {file}`,
		Goto:    `<a-x>goto-line<enter>{line}<enter>`,
		Quit:    `<a-x>kill-emacs<enter>`,
	},
}

//go:embed override_modal.star
var overrideModalStar string

//go:embed override_modal.yaml
var overrideModalYAML string

//go:embed override_modeless.star
var overrideModelessStar string

//go:embed override_modeless.yaml
var overrideModelessYAML string

//go:embed override_byoe_modal.star
var overrideBYOEModalStar string

//go:embed override_byoe_modal.yaml
var overrideBYOEModalYAML string

//go:embed override_byoe_modeless.star
var overrideBYOEModelessStar string

//go:embed override_byoe_modeless.yaml
var overrideBYOEModelessYAML string

// renderOverride returns the rendered override-config file body for the
// given editor choice and config format. For BYOE choices the named preset
// is substituted into the embedded template; for non-BYOE choices the body
// is returned verbatim and preset may be empty.
func renderOverride(editor, format, preset string) (string, error) {
	body, isByoe, err := overrideTemplate(editor, format)
	if err != nil {
		return "", err
	}
	if !isByoe {
		return body, nil
	}
	p, ok := byoePresets[preset]
	if !ok {
		return "", fmt.Errorf("unknown BYOE preset: %q", preset)
	}
	// Use non-default delimiters so the commented Go text/template
	// snippets that appear in the embedded BYOE override files (e.g.
	// `{{ .Status | fg "white" }}` inside an example `status_bar.layout`)
	// pass through verbatim. Only `<<.Command>>` and `<<.Goto>>` in the
	// active block get substituted.
	tmpl, err := template.New("override").Delims("<<", ">>").Parse(body)
	if err != nil {
		return "", fmt.Errorf("parse override template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return "", fmt.Errorf("execute override template: %w", err)
	}
	return buf.String(), nil
}

// overrideTemplate returns the raw override body for the given editor and
// format pair, along with whether the body needs BYOE template rendering.
func overrideTemplate(editor, format string) (body string, isByoe bool, err error) {
	switch editor {
	case editorModal:
		switch format {
		case configFormatYAML:
			return overrideModalYAML, false, nil
		case configFormatStar:
			return overrideModalStar, false, nil
		}
	case editorModeless:
		switch format {
		case configFormatYAML:
			return overrideModelessYAML, false, nil
		case configFormatStar:
			return overrideModelessStar, false, nil
		}
	case editorBYOEModal:
		switch format {
		case configFormatYAML:
			return overrideBYOEModalYAML, true, nil
		case configFormatStar:
			return overrideBYOEModalStar, true, nil
		}
	case editorBYOEModeless:
		switch format {
		case configFormatYAML:
			return overrideBYOEModelessYAML, true, nil
		case configFormatStar:
			return overrideBYOEModelessStar, true, nil
		}
	}
	return "", false, fmt.Errorf("unknown editor/format pair: %q/%q", editor, format)
}

// configFilenameForFormat maps a bootstrap config format to the file name
// (within the data directory) used to persist the user's selection.
func configFilenameForFormat(format string) string {
	switch format {
	case configFormatStar:
		return configStarFilename
	default:
		return configFilename
	}
}
