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

// Tests in this file exercise the cgo bridge against a real loaded
// model. They depend on the shared testdata service installed by
// service_test.go so the ~5 GiB GGUF is loaded only once per `go test`
// invocation; each test reuses testdataSvc.model and the live context.
//
// Tests are gated on RUNE_LLAMACPP_TESTDATA=1 via loadTestdataService.

import (
	"strings"
	"testing"

	"unstable.build/go-tui/cmd/rune-agent/llm"
)

// loadedModel returns the *Model behind the shared testdata service so
// cgo binding tests can probe it directly. Keeps the lazy-load + skip
// semantics of loadTestdataService.
func loadedModel(t *testing.T) *Model {
	t.Helper()
	svc := loadTestdataService(t)
	concrete, ok := svc.(*Service)
	if !ok {
		t.Fatalf("expected *Service, got %T", svc)
	}
	return concrete.model
}

// loadedContext returns the *Context bound to the shared testdata
// service. The context is shared across the suite: tests must not
// leave it in a state that breaks other testdata tests (clear KV at
// end if you mutate it).
func loadedContext(t *testing.T) *Context {
	t.Helper()
	svc := loadTestdataService(t)
	concrete, ok := svc.(*Service)
	if !ok {
		t.Fatalf("expected *Service, got %T", svc)
	}
	return concrete.context
}

// TestModel_Probe_RealGGUF pins the cgo accessors that pull static
// metadata out of the loaded model: NCtxTrain, NParams, Desc and the
// embedded ChatTemplate. Only sign / shape constraints are checked
// because the testdata GGUF is allowed to evolve without breaking the
// suite.
func TestModel_Probe_RealGGUF(t *testing.T) {
	m := loadedModel(t)

	if got := m.NCtxTrain(); got <= 0 {
		t.Fatalf("NCtxTrain = %d, want > 0", got)
	}
	// Gemma-4 E2B is ~2B parameters; allow a wide band so
	// re-quantisation does not destabilise the test.
	if got := m.NParams(); got < 1_000_000_000 || got > 10_000_000_000 {
		t.Fatalf("NParams = %d, want in [1e9, 10e9]", got)
	}
	desc := m.Desc()
	if desc == "" {
		t.Fatal("Desc() returned empty string")
	}
	if len(desc) > 256 {
		t.Fatalf("Desc() = %q (len=%d), want <= 256", desc, len(desc))
	}
	if got := m.ChatTemplate(""); got == "" {
		t.Fatal("ChatTemplate(\"\") = empty; testdata GGUF must ship a default template")
	}
	// A nonsense variant name returns "" (no template by that name)
	// rather than crashing.
	if got := m.ChatTemplate("definitely-not-a-real-template-name"); got != "" {
		t.Fatalf("ChatTemplate(unknown) = %q, want empty", got)
	}
}

// TestModel_TokenizeRoundTrip exercises Tokenize + TokenToPiece end to
// end. Concatenating the per-token pieces must reconstruct the input
// for ASCII text (BPE/SentencePiece special-token framing aside).
// This pins down that the buffer-grow path of Tokenize works correctly
// for inputs whose tokenisation exceeds the initial guess, and that
// TokenToPiece round-trips both regular and special tokens.
func TestModel_TokenizeRoundTrip(t *testing.T) {
	m := loadedModel(t)

	cases := []struct {
		name string
		text string
	}{
		{"short", "Hello, world!"},
		// long input: forces buf realloc inside Tokenize because the
		// initial size guess (len(text)+8) is conservative.
		{"long", strings.Repeat("the quick brown fox jumps over the lazy dog. ", 64)},
		{"unicode", "héllo — café résumé naïve"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := m.Tokenize(tc.text, false, false)
			if err != nil {
				t.Fatalf("Tokenize(%q): %v", tc.name, err)
			}
			if tc.text != "" && len(tokens) == 0 {
				t.Fatalf("Tokenize(%q) returned 0 tokens", tc.name)
			}
			// Reconstruct: piece-wise concatenation must equal the
			// original prompt for plain text. We strip an optional
			// leading space the tokenizer often inserts after BOS, but
			// we asked for no special tokens so this should not happen
			// in practice for these inputs.
			var rebuilt strings.Builder
			for _, tok := range tokens {
				rebuilt.WriteString(m.TokenToPiece(tok, false))
			}
			got := strings.TrimPrefix(rebuilt.String(), " ")
			want := strings.TrimPrefix(tc.text, " ")
			if got != want {
				t.Fatalf("round-trip mismatch:\n got  %q\n want %q", got, want)
			}
		})
	}
}

// TestModel_Tokenize_AddSpecial confirms that addSpecial=true prepends
// the model's BOS token. We don't pin the exact id (Gemma's BOS may
// shift across releases), only that the count grows by one and the
// extra token differs from anything in the no-special variant.
func TestModel_Tokenize_AddSpecial(t *testing.T) {
	m := loadedModel(t)
	plain, err := m.Tokenize("hello", false, false)
	if err != nil {
		t.Fatalf("Tokenize(plain): %v", err)
	}
	withSpecial, err := m.Tokenize("hello", true, false)
	if err != nil {
		t.Fatalf("Tokenize(addSpecial): %v", err)
	}
	if len(withSpecial) != len(plain)+1 {
		t.Fatalf("len(withSpecial)=%d, len(plain)=%d, want diff of 1 BOS",
			len(withSpecial), len(plain))
	}
	// IsEOG is a vocab-level predicate; BOS is not an EOG so the
	// prepended token must report false.
	if m.IsEOG(withSpecial[0]) {
		t.Fatalf("BOS token %d reported as EOG", withSpecial[0])
	}
}

// TestContext_KVOps exercises the KV-cache management bindings:
// CanShiftKV (capability probe), RemoveKVRange (partial removal +
// "remove from p0 to end" via p1=-1) and ShiftKVRange (must not panic
// on the live context). We always finish by clearing the cache so the
// shared service stays usable for downstream tests.
func TestContext_KVOps(t *testing.T) {
	m := loadedModel(t)
	ctx := loadedContext(t)
	t.Cleanup(ctx.ClearKV)

	// Seed the cache with a small batch so RemoveKVRange has something
	// to operate on. The shared sequence id used by the production
	// path is 0; using a different id keeps this test isolated.
	const seqID = 7
	ctx.RemoveKVRange(seqID, 0, -1) // start from a known-empty seq

	tokens, err := m.Tokenize("alpha beta gamma delta epsilon", false, false)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	if len(tokens) < 3 {
		t.Skipf("seed tokenisation produced only %d tokens; cannot exercise KV ops", len(tokens))
	}

	batch := NewBatch(len(tokens))
	defer batch.Close()
	for i, tok := range tokens {
		batch.Add(tok, i, false)
	}
	if err := ctx.Decode(batch); err != nil {
		t.Fatalf("Decode seed: %v", err)
	}

	// Capability probe: the default unified-KV memory implementation
	// supports shifts. CanShiftKV must return true so the
	// applyCacheReuse path in the service is exercised.
	if !ctx.CanShiftKV() {
		t.Fatal("CanShiftKV() = false on default backend; expected true")
	}

	// Drop the trailing two positions and re-decode them. RemoveKVRange
	// returns true on success for the unified backend.
	if !ctx.RemoveKVRange(seqID, len(tokens)-2, -1) {
		t.Fatalf("RemoveKVRange(%d, %d, -1) returned false", seqID, len(tokens)-2)
	}

	// ShiftKVRange must complete without crashing even on a delta of
	// 0 (a no-op shift). We don't pin the post-state because the
	// returned positions are an opaque implementation detail of
	// llama_memory_seq_add.
	ctx.ShiftKVRange(seqID, 0, len(tokens)-2, 0)
}

// TestNewSampler_WithChatTemplate exercises the cgo path that wires a
// resolved chat template into the sampler. This is the branch that
// pulls grammar / lazy-trigger settings from common_chat_templates_apply
// and the one production CreateCompletion uses for Gemma-4. Asserting
// only that the sampler constructs and frees cleanly avoids pinning
// upstream randomness.
func TestNewSampler_WithChatTemplate(t *testing.T) {
	m := loadedModel(t)

	handle, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: "hi"}},
		ChatTemplateOptions{AddGenerationPrompt: true, EnableThinking: false},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer handle.Close()

	s, err := NewSampler(m, DefaultSamplerParams(), handle)
	if err != nil {
		t.Fatalf("NewSampler(with template): %v", err)
	}
	s.Close()
	s.Close() // double-close must be a no-op for resource cleanup contracts.
}

// TestApplyChatTemplate_LegacyForced drives the legacy
// llama_chat_apply_template() path by forcing it explicitly via
// RUNE_LLAMACPP_FORCE_LEGACY_CHAT. The production code prefers the
// Jinja engine; the legacy branch is otherwise reachable only when
// Jinja refuses a template. Pinning the "chatml" alias guarantees a
// model-independent expected prefix.
func TestApplyChatTemplate_LegacyForced(t *testing.T) {
	m := loadedModel(t)
	t.Setenv("RUNE_LLAMACPP_FORCE_LEGACY_CHAT", "1")

	got, err := m.ApplyChatTemplate("chatml", []ChatMessage{
		{Role: "system", Content: "you are terse"},
		{Role: "user", Content: "hi"},
	}, true)
	if err != nil {
		t.Fatalf("ApplyChatTemplate(chatml legacy): %v", err)
	}
	if !strings.Contains(got, "<|im_start|>system") {
		t.Fatalf("legacy chatml output missing system marker: %q", got)
	}
	if !strings.Contains(got, "<|im_start|>user") {
		t.Fatalf("legacy chatml output missing user marker: %q", got)
	}
	if !strings.Contains(got, "<|im_start|>assistant") {
		t.Fatalf("legacy chatml output missing assistant generation prompt: %q", got)
	}
}

// TestChatStream_Feed_DrivesParser exercises the upstream PEG parser
// through the cgo layer. We feed a synthetic assistant message piece
// by piece and assert the stream emits a text-delta event whose
// concatenation reproduces the input.
func TestChatStream_Feed_DrivesParser(t *testing.T) {
	m := loadedModel(t)

	handle, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: "hi"}},
		ChatTemplateOptions{AddGenerationPrompt: true, EnableThinking: false},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer handle.Close()
	if !handle.HasParser() {
		t.Skip("template has no PEG parser; nothing to drive")
	}

	stream, err := NewChatStream(handle)
	if err != nil {
		t.Fatalf("NewChatStream: %v", err)
	}
	defer stream.Close()

	pieces := []string{"hello", " ", "world", "."}
	for i, p := range pieces {
		isPartial := i < len(pieces)-1
		if err := stream.Feed(p, isPartial); err != nil {
			t.Fatalf("Feed(%q): %v", p, err)
		}
	}
	var visible strings.Builder
	for {
		ev, ok := stream.Next()
		if !ok {
			break
		}
		if ev.Kind == ChatStreamEventTextDelta {
			visible.WriteString(ev.Text)
		}
	}
	want := strings.Join(pieces, "")
	if got := visible.String(); got != want {
		t.Fatalf("reconstructed text = %q, want %q", got, want)
	}
}

// TestChatTemplateHandle_AdditionalStops verifies the
// AdditionalStops cgo path executes against a real template. The
// upstream parser may legitimately return zero stops for a plain text
// turn, so we only pin shape: every returned stop is non-empty and the
// cgo round trip does not panic. This guards against regressions in
// the rune_chat_template_additional_stop_count + buffer-grow loop
// without coupling to upstream stop-string policy.
func TestChatTemplateHandle_AdditionalStops(t *testing.T) {
	m := loadedModel(t)

	handle, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: "hi"}},
		ChatTemplateOptions{
			AddGenerationPrompt: true,
			EnableThinking:      false,
			Tools: []Tool{{
				Name:           "get_time",
				Description:    "Return the current time.",
				ParametersJSON: `{"type":"object","properties":{}}`,
			}},
		},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer handle.Close()

	for i, s := range handle.AdditionalStops() {
		if s == "" {
			t.Fatalf("stop[%d] is empty", i)
		}
	}
}

// TestChatTemplate_PromptReproducesInput drives the Prompt() cgo round
// trip via OpenChatTemplate against the real model. The rendered
// prompt must include the user's content; this guarantees the
// rune_chat_template_prompt buffer logic returns a non-truncated,
// non-corrupt string.
func TestChatTemplate_PromptReproducesInput(t *testing.T) {
	m := loadedModel(t)

	const needle = "EARTH-CONTAINS-NEEDLE-42"
	handle, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: needle}},
		ChatTemplateOptions{AddGenerationPrompt: true, EnableThinking: false},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer handle.Close()

	prompt, err := handle.Prompt()
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if !strings.Contains(prompt, needle) {
		t.Fatalf("rendered prompt missing user content; len=%d", len(prompt))
	}
}

// TestModel_HasProjector_Real pins HasProjector against the real
// loaded model: the testdata reference includes an mmproj layer so
// the result must be true. Pairs with the nil-receiver coverage
// already in TestPublicNilSafeLifecycleMethods.
func TestModel_HasProjector_Real(t *testing.T) {
	m := loadedModel(t)
	if !m.HasProjector() {
		t.Fatal("HasProjector() = false; testdata reference includes mmproj layer")
	}
}

// TestService_Testdata_CountTokens_Empty pins the empty-prompt
// behaviour through the public surface — formatMessages still has to
// render the chat template envelope, so the count is positive. This
// pins down the empty-content branch of buildCRichChatMessages where
// Content == "" but every other field is empty.
func TestService_Testdata_CountTokens_Empty(t *testing.T) {
	svc := loadTestdataService(t)
	n, err := svc.CountTokens([]llm.Message{
		{Role: llm.RoleUser, Content: ""},
	})
	if err != nil {
		t.Fatalf("CountTokens(empty): %v", err)
	}
	if n <= 0 {
		t.Fatalf("CountTokens(empty) = %d, want > 0 (chat template still adds envelope)", n)
	}
}

// TestModel_Tokenize_RejectsInvalidUTF8 pins the cgo boundary check
// added after FuzzTokenize/e1bd2d266978352e: llama.cpp's tokenizer
// SIGSEGVs on certain non-UTF-8 byte sequences (it normalises Unicode
// before lookup, and the normaliser indexes a fixed table by
// codepoint). We refuse those inputs at the Go boundary so a caller
// passing a corrupted []byte gets a deterministic error rather than
// taking the whole process down.
func TestModel_Tokenize_RejectsInvalidUTF8(t *testing.T) {
	m := loadedModel(t)
	// 0xff 0xfe is a UTF-16 BOM: invalid as the leading byte of a
	// UTF-8 codepoint. The exact byte sequence below is the one the
	// fuzzer found.
	const bad = "\xff\xfe\x00\x01"
	_, err := m.Tokenize(bad, false, false)
	if err == nil {
		t.Fatal("Tokenize(invalid UTF-8): want error, got nil")
	}
}
