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

// Fuzz tests covering the cgo boundary. The goal is to catch
// SIGSEGVs / heap corruption hiding in the way Go strings are handed
// to llama.cpp: any code path that crosses the cgo boundary with a
// caller-controlled (ptr, len) pair is a place where a Go-side off-by-
// one or an unexpected NUL can take the whole process down.
//
// Each Fuzz target installs a small seed corpus (covering empty/ASCII
// /UTF-8/embedded-NUL/non-UTF-8 cases) so the corpus runs even under a
// plain `go test` and so `go test -fuzz=…` has somewhere to start.
// Targets that exercise the loaded model are gated on
// RUNE_LLAMACPP_TESTDATA=1 via loadTestdataService.
//
// IMPORTANT: model-backed fuzz targets must run with -parallel=1.
// `go test -fuzz` forks one worker process per parallel slot and each
// worker independently loads the ~5 GiB GGUF + Metal context. Loading
// 16 of those simultaneously OOMs Metal and produces non-input-related
// failures that get attributed to the fuzz seed by go-test. Pure-cgo
// targets (FuzzFindPartialStop, FuzzBuildCRichChatMessages) have no
// such constraint.

import (
	"strings"
	"testing"
)

// commonFuzzCorpus seeds Fuzz targets that take a single string. The
// values are chosen to provoke the kinds of pointer-arithmetic bugs
// fuzzing is good at finding: empty buffers, embedded NULs (which
// truncate strlen-based code paths), invalid UTF-8 (which can confuse
// length math), and very long inputs (which exercise realloc loops).
func commonFuzzCorpus() []string {
	return []string{
		"",
		"a",
		"hello world",
		"héllo — café",                 // multi-byte UTF-8
		"abc\x00def",                   // embedded NUL: cgo must use len, not strlen
		"\xff\xfe\xfd",                 // invalid UTF-8
		strings.Repeat("x", 4096),      // long, forces buffer growth
		strings.Repeat("héllo ", 1000), // long + multi-byte
		"<|im_start|>user\nhi\n<|im_end|>",
		"<turn|><|channel|>",
	}
}

// FuzzFindPartialStop fuzzes the upstream string_find_partial_stop
// shim that genState.applyStops relies on. The wrapper passes
// caller-controlled (ptr, len) pairs straight into C, so any indexing
// bug there will surface as a SIGSEGV before this fuzzer can return.
// Pure cgo: no model required.
func FuzzFindPartialStop(f *testing.F) {
	for _, text := range commonFuzzCorpus() {
		for _, stop := range commonFuzzCorpus() {
			f.Add(text, stop)
		}
	}
	f.Fuzz(func(_ *testing.T, text, stop string) {
		off := findPartialStop(text, stop)
		// The upstream contract: the returned offset is either -1 or
		// a valid byte index into text. Anything outside that band
		// indicates the cgo wrapper handed bad lengths to C.
		if off < -1 || off > len(text) {
			panic("findPartialStop returned out-of-range offset")
		}
	})
}

// FuzzBuildCRichChatMessages fuzzes the message-array builder. It
// allocates a contiguous C array containing a rune_chat_message per
// Go-side ChatMessage, then copies role/content/etc. into C-owned
// memory. Bugs there (pointer-to-Go-pointer leaks, missing zero-init,
// double-free) crash the process.
func FuzzBuildCRichChatMessages(f *testing.F) {
	for _, role := range []string{"system", "user", "assistant", "tool", ""} {
		for _, content := range commonFuzzCorpus() {
			f.Add(role, content, "", "")
		}
	}
	// Seed cases that mix every optional field so the pointer
	// arithmetic for content_parts and tool_calls is exercised.
	f.Add("assistant", "hi", "I should call a tool", `{"a":1}`)
	f.Add("tool", "result", "", "")
	f.Add("user", "abc\x00def", "rzn\x00", "")
	f.Fuzz(func(_ *testing.T, role, content, reasoning, toolArgs string) {
		msgs := []ChatMessage{{
			Role:             role,
			Content:          content,
			ReasoningContent: reasoning,
		}}
		if toolArgs != "" {
			msgs[0].ToolCalls = []ChatToolCall{{
				Name:      "fn",
				Arguments: toolArgs,
				ID:        "call_0",
			}}
		}
		// buildCRichChatMessages allocates with C.llamacpp_zalloc;
		// the cleanup func must always be safe to invoke even when
		// the input contains NULs or non-UTF-8 because we pass len-
		// erased C strings.
		base, cleanup := buildCRichChatMessages(msgs)
		defer cleanup()
		_ = base
	})
}

// FuzzTokenize fuzzes Model.Tokenize against the real loaded model.
// Tokenize hands the raw text bytes + length straight to
// llama_tokenize and then walks an int32 buffer; an off-by-one in the
// buffer-grow loop or a mishandled non-UTF-8 byte will SIGSEGV.
//
// The fuzz target is gated on the testdata model so the corpus runs
// only when the suite is opted in.
func FuzzTokenize(f *testing.F) {
	for _, s := range commonFuzzCorpus() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		m := loadedModel(t)
		tokens, err := m.Tokenize(text, false, false)
		if err != nil {
			// Errors are fine; segfaults are not.
			return
		}
		// Empty text must produce zero tokens. Anything else means
		// the cgo path read past the input.
		if text == "" && len(tokens) != 0 {
			t.Fatalf("Tokenize(\"\") returned %d tokens, want 0", len(tokens))
		}
		// Sanity: piece reconstruction must not panic for any
		// produced token. We don't assert content (the fuzzer can
		// generate non-UTF-8 input that the tokenizer normalises),
		// only that TokenToPiece survives every produced id.
		for _, tok := range tokens {
			_ = m.TokenToPiece(tok, false)
		}
	})
}

// FuzzChatStreamFeed fuzzes the streaming PEG parser via Feed. Each
// call hands a Go-managed string into rune_chat_stream_feed which then
// runs common_chat_parse on the accumulated buffer. The fuzzer is the
// only realistic way to exercise the parser against arbitrary
// byte sequences (the model normally produces well-formed text).
func FuzzChatStreamFeed(f *testing.F) {
	for _, s := range commonFuzzCorpus() {
		f.Add(s, true)
		f.Add(s, false)
	}
	f.Fuzz(func(t *testing.T, text string, isPartial bool) {
		m := loadedModel(t)
		// Open a fresh handle + stream per fuzz iteration so state
		// from prior inputs cannot mask later bugs.
		handle, err := m.OpenChatTemplate(
			[]ChatMessage{{Role: "user", Content: "hi"}},
			ChatTemplateOptions{AddGenerationPrompt: true, EnableThinking: false},
		)
		if err != nil {
			t.Skip("OpenChatTemplate failed:", err)
		}
		defer handle.Close()
		if !handle.HasParser() {
			t.Skip("template has no PEG parser; nothing to fuzz")
		}
		stream, err := NewChatStream(handle)
		if err != nil {
			t.Skip("NewChatStream failed:", err)
		}
		defer stream.Close()

		// The parser is the contract under test. Feed errors are
		// acceptable (malformed input may be rejected); a crash is
		// not. Drain Next afterwards so the C-side queue allocator
		// is exercised.
		_ = stream.Feed(text, isPartial)
		for {
			if _, ok := stream.Next(); !ok {
				break
			}
		}
	})
}

// FuzzApplyChatTemplateLegacy fuzzes the legacy
// llama_chat_apply_template path. The legacy implementation expects
// NUL-terminated C strings; our wrapper passes Go-managed buffers via
// C.CString which copies, so embedded NULs truncate but must not
// crash. The fuzzer pins down that contract.
func FuzzApplyChatTemplateLegacy(f *testing.F) {
	for _, content := range commonFuzzCorpus() {
		f.Add("user", content)
		f.Add("system", content)
		f.Add("assistant", content)
	}
	f.Fuzz(func(t *testing.T, role, content string) {
		m := loadedModel(t)
		// "chatml" is a stable legacy alias understood by every
		// model; it routes through the legacy path without going via
		// Jinja first.
		_, _ = m.applyChatTemplateLegacy("chatml",
			[]ChatMessage{{Role: role, Content: content}}, true)
	})
}
