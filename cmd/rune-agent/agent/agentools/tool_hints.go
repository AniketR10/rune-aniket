// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agentools

import "regexp"

// bashToolHint inspects a shell command string and returns a hint suggesting
// a dedicated built-in tool when the command could be replaced by one.
// Returns empty string when no hint applies.
func bashToolHint(command string) string {
	for _, h := range toolHintRules {
		if h.pattern.MatchString(command) {
			return h.hint
		}
	}
	return ""
}

type toolHintRule struct {
	pattern *regexp.Regexp
	hint    string
}

var toolHintRules = []toolHintRule{
	{
		pattern: regexp.MustCompile(`\b(grep|rg|ag|ack)\b`),
		hint:    "Use the search_content or find_files tool instead of grep/rg for text search, or search_symbols/find_definition for symbol search.",
	},
	{
		pattern: regexp.MustCompile(`\bfind\s`),
		hint:    "Use the find_files tool instead of the find command.",
	},
	{
		pattern: regexp.MustCompile(`\b(cat|head|tail|less|more)\b`),
		hint:    "Use the read_file tool instead of cat/head/tail to read file contents.",
	},
	{
		pattern: regexp.MustCompile(`\b(gofmt|goimports|go\s+fmt)\b`),
		hint:    "Use the format_file tool instead of running formatters via shell.",
	},
	{
		pattern: regexp.MustCompile(`\bsed\b`),
		hint:    "Use apply_patch or rename_symbol instead of sed for file modifications.",
	},
	{
		pattern: regexp.MustCompile(`\bwc\s+-l\b`),
		hint:    "Use read_file or outline_file to inspect file contents instead of wc.",
	},
}
