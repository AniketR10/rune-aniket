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


package llamacpp

import (
	"strings"
	"testing"
)

// TestApplyChatTemplate_Gemma4Jinja reproduces the original bug: the
// legacy llama_chat_apply_template detector does not recognise Gemma 4's
// embedded Jinja template (it uses <|turn>/<turn|> tokens, not Gemma
// 1-3's <start_of_turn>/<end_of_turn>). Before wiring in the Jinja
// engine this call returned "llamacpp: chat template failed (-1)".
//
// The template below is a self-contained reduction of the real Gemma 4
// template pulled from the GGUF. It deliberately avoids hooks that need
// a model handle (bos_token / eos_token access, tool schema filters), so
// the test can run without loading a multi-GB weight blob.
func TestApplyChatTemplate_Gemma4Jinja(t *testing.T) {
	t.Parallel()
	const tmpl = `{%- for message in messages -%}
    {%- if message['role'] == 'assistant' -%}
        {%- set role = 'model' -%}
    {%- else -%}
        {%- set role = message['role'] -%}
    {%- endif -%}
    {{- '<|turn>' + role + '\n' -}}
    {{- message['content'] | trim -}}
    {{- '<turn|>\n' -}}
{%- endfor -%}
{%- if add_generation_prompt -%}
    {{- '<|turn>model\n' -}}
{%- endif -%}`

	var m *Model // no model handle needed for this template
	out, err := m.ApplyChatTemplate(tmpl, []ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "bye"},
	}, true)
	if err != nil {
		t.Fatalf("ApplyChatTemplate: %v", err)
	}
	// Exact render: each turn is '<|turn>ROLE\nCONTENT<turn|>\n', and we
	// appended a generation prompt.
	want := "<|turn>user\nhello<turn|>\n" +
		"<|turn>model\nhi there<turn|>\n" +
		"<|turn>user\nbye<turn|>\n" +
		"<|turn>model\n"
	if out != want {
		t.Fatalf("unexpected render\n got: %q\nwant: %q", out, want)
	}
}

// TestApplyChatTemplate_ChatmlJinja verifies that the Jinja path also
// handles a vanilla ChatML template — the common case. This catches
// regressions where the Jinja wrapper broke previously-working models.
func TestApplyChatTemplate_ChatmlJinja(t *testing.T) {
	t.Parallel()
	const tmpl = `{%- for message in messages -%}
{{- '<|im_start|>' + message['role'] + '\n' + message['content'] + '<|im_end|>\n' -}}
{%- endfor -%}
{%- if add_generation_prompt -%}{{- '<|im_start|>assistant\n' -}}{%- endif -%}`

	var m *Model
	out, err := m.ApplyChatTemplate(tmpl, []ChatMessage{
		{Role: "system", Content: "be terse"},
		{Role: "user", Content: "hi"},
	}, true)
	if err != nil {
		t.Fatalf("ApplyChatTemplate: %v", err)
	}
	want := "<|im_start|>system\nbe terse<|im_end|>\n" +
		"<|im_start|>user\nhi<|im_end|>\n" +
		"<|im_start|>assistant\n"
	if out != want {
		t.Fatalf("unexpected render\n got: %q\nwant: %q", out, want)
	}
}

// TestApplyChatTemplate_LegacyFallback confirms the legacy code path still
// works when explicitly requested via RUNE_LLAMACPP_FORCE_LEGACY_CHAT=1.
// Forcing legacy keeps the fallback honest: short template aliases like
// "chatml" that the Jinja parser doesn't understand still have to render.
func TestApplyChatTemplate_LegacyFallback(t *testing.T) {
	t.Setenv("RUNE_LLAMACPP_FORCE_LEGACY_CHAT", "1")

	var m *Model
	out, err := m.ApplyChatTemplate("chatml", []ChatMessage{
		{Role: "user", Content: "hi"},
	}, true)
	if err != nil {
		t.Fatalf("ApplyChatTemplate: %v", err)
	}
	// Legacy chatml emits <|im_start|>…<|im_end|>.
	if !strings.Contains(out, "<|im_start|>user") || !strings.Contains(out, "<|im_end|>") {
		t.Fatalf("legacy chatml output missing expected markers: %q", out)
	}
}
