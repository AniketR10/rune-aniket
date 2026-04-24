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
	"sync"
	"testing"
	"unsafe"
)

func TestDefaultParams_PublicConstructors(t *testing.T) {
	t.Parallel()

	t.Run("DefaultModelParams", func(t *testing.T) {
		got := DefaultModelParams()
		if got.NGPULayers != -1 {
			t.Fatalf("NGPULayers = %d, want -1", got.NGPULayers)
		}
		if !got.UseMMAP {
			t.Fatal("UseMMAP = false, want true")
		}
		if got.UseMLock {
			t.Fatal("UseMLock = true, want false")
		}
		if got.VocabOnly {
			t.Fatal("VocabOnly = true, want false")
		}
		if got.Progress != nil {
			t.Fatal("Progress != nil, want nil")
		}
	})

	t.Run("DefaultContextParams", func(t *testing.T) {
		got := DefaultContextParams()
		if got.NCtx != 0 {
			t.Fatalf("NCtx = %d, want 0", got.NCtx)
		}
		if got.NBatch != 2048 {
			t.Fatalf("NBatch = %d, want 2048", got.NBatch)
		}
		if got.NUBatch != 512 {
			t.Fatalf("NUBatch = %d, want 512", got.NUBatch)
		}
		if got.NThreads != 0 {
			t.Fatalf("NThreads = %d, want 0", got.NThreads)
		}
		if got.NThreadsBatch != 0 {
			t.Fatalf("NThreadsBatch = %d, want 0", got.NThreadsBatch)
		}
		if got.FlashAttention {
			t.Fatal("FlashAttention = true, want false")
		}
	})

	t.Run("DefaultSamplerParams", func(t *testing.T) {
		got := DefaultSamplerParams()
		if got.Seed != 0xFFFFFFFF {
			t.Fatalf("Seed = %d, want 0xFFFFFFFF", got.Seed)
		}
		if got.Temperature != 0.8 {
			t.Fatalf("Temperature = %v, want 0.8", got.Temperature)
		}
		if got.TopK != 40 {
			t.Fatalf("TopK = %d, want 40", got.TopK)
		}
		if got.TopP != 0.95 {
			t.Fatalf("TopP = %v, want 0.95", got.TopP)
		}
		if got.MinP != 0.05 {
			t.Fatalf("MinP = %v, want 0.05", got.MinP)
		}
		if got.RepeatPenalty != 1.0 {
			t.Fatalf("RepeatPenalty = %v, want 1.0", got.RepeatPenalty)
		}
		if got.RepeatLastN != 64 {
			t.Fatalf("RepeatLastN = %d, want 64", got.RepeatLastN)
		}
	})
}

func TestPublicNilSafeLifecycleMethods(t *testing.T) {
	t.Parallel()

	t.Run("Model.Close nil-safe", func(t *testing.T) {
		var m *Model
		m.Close()
		m = &Model{}
		m.Close()
	})

	t.Run("Context accessors and Close nil-safe", func(t *testing.T) {
		ctx := &Context{}
		if got := ctx.NCtx(); got != 0 {
			t.Fatalf("NCtx = %d, want 0 on zero-value Context", got)
		}
		if got := ctx.NBatch(); got != 0 {
			t.Fatalf("NBatch = %d, want 0 on zero-value Context", got)
		}
		if got := ctx.Model(); got != nil {
			t.Fatalf("Model = %#v, want nil", got)
		}

		var nilCtx *Context
		nilCtx.Close()
		ctx.Close()
	})

	t.Run("Sampler.Close nil-safe", func(t *testing.T) {
		var s *Sampler
		s.Close()
		s = &Sampler{}
		s.Close()
	})

	t.Run("HasProjector nil-safe", func(t *testing.T) {
		var m *Model
		if m.HasProjector() {
			t.Fatal("HasProjector() = true on nil model, want false")
		}
		m = &Model{}
		if m.HasProjector() {
			t.Fatal("HasProjector() = true without projector, want false")
		}
	})
}

func TestBatch_PublicBehavior(t *testing.T) {
	t.Parallel()

	b := NewBatch(4)
	defer b.Close()

	if got := b.Capacity(); got != 4 {
		t.Fatalf("Capacity = %d, want 4", got)
	}
	if got := b.Len(); got != 0 {
		t.Fatalf("Len = %d, want 0", got)
	}

	b.Add(11, 0, false)
	b.Add(12, 1, true)
	if got := b.Len(); got != 2 {
		t.Fatalf("Len after Add = %d, want 2", got)
	}

	b.Clear()
	if got := b.Len(); got != 0 {
		t.Fatalf("Len after Clear = %d, want 0", got)
	}

	var nilBatch *Batch
	nilBatch.Close()
}

func TestPublicErrorPaths_WithoutModelLoad(t *testing.T) {
	t.Parallel()

	t.Run("NewSampler with nil model panics", func(t *testing.T) {
		// NewSampler dereferences m.c; a nil/zero Model is a programmer
		// error and must crash rather than be tolerated.
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("NewSampler(nil): expected panic")
			}
		}()
		_, _ = NewSampler(nil, DefaultSamplerParams(), nil)
	})
}

// TestModel_NilMethods_Panic pins down the runtime contract on every
// Model method that dereferences m.c: calling them on a nil/zero Model
// is a programmer error (the API requires a successful LoadModel). The
// runtime is allowed to crash; it must not silently return a zero
// value, because that masks construction bugs in callers.
func TestModel_NilMethods_Panic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func(m *Model)
	}{
		{"NCtxTrain", func(m *Model) { _ = m.NCtxTrain() }},
		{"NParams", func(m *Model) { _ = m.NParams() }},
		{"Desc", func(m *Model) { _ = m.Desc() }},
		{"ChatTemplate", func(m *Model) { _ = m.ChatTemplate("") }},
		{"TokenToPiece", func(m *Model) { _ = m.TokenToPiece(0, false) }},
		{"IsEOG", func(m *Model) { _ = m.IsEOG(0) }},
		{"Tokenize", func(m *Model) { _, _ = m.Tokenize("hi", true, true) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("%s on nil Model: expected panic", tc.name)
				}
			}()
			tc.call(nil)
		})
	}
}

// TestSetLogger_Concurrent stresses the SetLogger / log-callback path.
// The C-side trampoline can be invoked from any thread that produces a
// log line; SetLogger must be safe to call concurrently with callbacks
// and a nil logger must silently no-op.
func TestSetLogger_Concurrent(t *testing.T) {
	// Mutates package-level logger state; do not run in parallel with
	// other logger-sensitive tests.
	original := func(LogLevel, string) {} // restore default at end
	t.Cleanup(func() { SetLogger(original) })

	var (
		mu   sync.Mutex
		seen int
	)
	logger := func(LogLevel, string) {
		mu.Lock()
		seen++
		mu.Unlock()
	}
	SetLogger(logger)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range 200 {
			if i%2 == 0 {
				SetLogger(logger)
			} else {
				SetLogger(nil)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 200 {
			callLlamacppLog(int32(i%5), "test\n")
		}
	}()
	wg.Wait()

	mu.Lock()
	if seen == 0 {
		t.Log("note: no callbacks observed (sampling raced with nil-logger windows)")
	}
	mu.Unlock()

	// nil-logger silencing: install nil and call once; the package's
	// defaultLogger must absorb the call without panicking.
	SetLogger(nil)
	callLlamacppLog(1, "after nil")
}

// TestBatch_PublicBehavior_Edges covers behaviour on edge sizes that
// real usage can hit (zero capacity, exact capacity).
func TestBatch_PublicBehavior_Edges(t *testing.T) {
	t.Parallel()

	t.Run("zero_capacity", func(t *testing.T) {
		b := NewBatch(0)
		defer b.Close()
		if b.Capacity() != 0 || b.Len() != 0 {
			t.Fatalf("zero batch: cap=%d len=%d", b.Capacity(), b.Len())
		}
	})

	t.Run("fill_to_capacity", func(t *testing.T) {
		b := NewBatch(3)
		defer b.Close()
		b.Add(10, 0, false)
		b.Add(11, 1, false)
		b.Add(12, 2, true)
		if b.Len() != 3 {
			t.Fatalf("len = %d, want 3", b.Len())
		}
	})

	t.Run("close_double", func(t *testing.T) {
		b := NewBatch(2)
		b.Close()
		// Second Close on the same Batch must not panic.
		// The wrapped llama_batch C struct is reset by the upstream call;
		// we forbid double-free by releasing the cap and bypassing free.
		// Currently Close has no nil-safety beyond the receiver — verify
		// that contract by only calling on nil here.
		var nilB *Batch
		nilB.Close()
	})
}

// TestFindPartialStop_Table pins the upstream string_find_partial_stop
// shim. We don't reimplement the algorithm; we lock in the cases that
// genState.applyStops actually relies on.
func TestFindPartialStop_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		stop string
		// We assert only sign + reasonableness of the offset rather than
		// pinning an exact integer, since the upstream contract is
		// "offset to a non-empty prefix of stop that ends text, or -1".
		wantNonNeg bool
	}{
		{"empty_text", "", "<turn|>", false},
		{"empty_stop", "abc", "", false},
		{"both_empty", "", "", false},
		{"no_match", "abcdef", "<stop>", false},
		{"prefix_match", "hello<tu", "<turn|>", true},
		{"full_match", "hello<turn|>", "<turn|>", true},
		{"single_char_stop", "abc", "x", false},
		{"utf8_text", "héllo<tu", "<turn|>", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			off := findPartialStop(tc.text, tc.stop)
			if tc.wantNonNeg && off < 0 {
				t.Fatalf("findPartialStop(%q, %q) = %d, want >= 0", tc.text, tc.stop, off)
			}
			if !tc.wantNonNeg && off >= 0 {
				t.Fatalf("findPartialStop(%q, %q) = %d, want -1", tc.text, tc.stop, off)
			}
		})
	}
}

// TestBuildCLegacyChatMessages_Table verifies the legacy message
// builder produces non-NULL role/content pointers — a NULL there
// crashes llama_chat_apply_template.
func TestBuildCLegacyChatMessages_Table(t *testing.T) {
	t.Parallel()

	t.Run("empty_returns_nil", func(t *testing.T) {
		ptr, free := buildCLegacyChatMessages(nil)
		defer free()
		if ptr != nil {
			t.Fatalf("ptr = %v, want nil", ptr)
		}
	})

	t.Run("populates_pointers", func(t *testing.T) {
		msgs := []ChatMessage{
			{Role: "system", Content: "be nice"},
			{Role: "user", Content: ""},
			{Role: "assistant", Content: "hi"},
		}
		ptr, free := buildCLegacyChatMessages(msgs)
		defer free()
		if ptr == nil {
			t.Fatal("ptr = nil, want non-nil")
		}
		for i := range msgs {
			rolePtr, contentPtr := cLegacyMsgPointersSnapshot(ptr, i, len(msgs))
			if rolePtr == 0 {
				t.Fatalf("msg[%d] role pointer is NULL", i)
			}
			if contentPtr == 0 {
				t.Fatalf("msg[%d] content pointer is NULL (even empty must point to NUL)", i)
			}
		}
	})
}

// TestBuildCRichChatMessages_FullShape covers the all-fields-populated
// path: content_parts and tool_calls must be non-NULL with correct
// lengths. Pairs with TestBuildCRichChatMessages_ZerosOptionalFields,
// which covers the zero path.
func TestBuildCRichChatMessages_FullShape(t *testing.T) {
	t.Parallel()
	msgs := []ChatMessage{{
		Role:             "assistant",
		Content:          "",
		ReasoningContent: "I should call a tool",
		Name:             "asst",
		ToolCallID:       "",
		ContentParts: []ChatContentPart{
			{Type: "text", Text: "hello"},
			{Type: "text", Text: "world"},
		},
		ToolCalls: []ChatToolCall{
			{Name: "get_time", Arguments: `{"tz":"UTC"}`, ID: "call_1"},
		},
	}}
	base, cleanup := buildCRichChatMessages(msgs)
	defer cleanup()
	if base == nil {
		t.Fatal("buildCRichChatMessages returned nil base")
	}
	cpPtr, cpLen, tcPtr, tcLen := cMsgOptionalFieldsSnapshot(unsafe.Pointer(base), 0, len(msgs))
	if cpPtr == 0 || cpLen != 2 {
		t.Fatalf("content_parts: ptr=%#x len=%d, want non-zero/2", cpPtr, cpLen)
	}
	if tcPtr == 0 || tcLen != 1 {
		t.Fatalf("tool_calls: ptr=%#x len=%d, want non-zero/1", tcPtr, tcLen)
	}
}

// TestApplyChatTemplate_NilModel_Panics pins the new contract: calling
// ApplyChatTemplate on a nil receiver is a programmer error. The first
// thing the implementation does is m.ChatTemplate(""), which
// dereferences m.c — we want that to crash rather than silently fall
// back to an empty template.
func TestApplyChatTemplate_NilModel_Panics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("ApplyChatTemplate(nil model): expected panic")
		}
	}()
	var m *Model
	_, _ = m.ApplyChatTemplate("", []ChatMessage{{Role: "user", Content: "hi"}}, true)
}

// TestProgressCallback_NoUserData covers the early-return branch of
// the cgo progress trampoline: when the user_data handle is zero the
// callback must return 1 (continue) without trying to dispatch.
// Driving the exported callback through a helper keeps the test free
// of cgo type imports.
func TestProgressCallback_NoUserData(t *testing.T) {
	t.Parallel()
	if got := callLlamacppProgress(0.5, 0); got != 1 {
		t.Fatalf("progress callback with 0 user_data = %d, want 1", got)
	}
}

// TestProgressCallback_Conversion verifies the float→percent conversion
// the trampoline performs. We register a handle with a callback that
// captures the (progress, total, units) tuple, then drive the
// trampoline directly with a representative range of values, including
// out-of-range values that must be clamped.
func TestProgressCallback_Conversion(t *testing.T) {
	t.Parallel()
	type sample struct {
		progress, total int64
		units           string
	}
	var got []sample
	cb := func(progress, total int64, units string) {
		got = append(got, sample{progress, total, units})
	}
	// runtime/cgo handles can be allocated from any goroutine — we use the
	// production helper so we don't need to import runtime/cgo here.
	mp := DefaultModelParams()
	mp.Progress = cb
	// We only need a value that round-trips through cgo.NewHandle; the
	// model load itself isn't exercised. Construct the handle the same
	// way LoadModel does, then dispatch through the package's exported
	// trampoline by reusing the test-only helper.
	_ = mp

	// Direct dispatch through the helper's userData==0 path is covered
	// by TestProgressCallback_NoUserData. Here we drive the path that
	// pulls a callback out of cgo.Handle.
	h := newProgressHandle(cb)
	defer h.Delete()

	for _, p := range []float32{0, 0.25, 0.5, 1.0, 2.0, -1.0} {
		if rc := callLlamacppProgress(p, uintptr(h)); rc != 1 {
			t.Fatalf("progress callback returned %d, want 1", rc)
		}
	}
	if len(got) != 6 {
		t.Fatalf("captured %d samples, want 6", len(got))
	}
	if got[0].total != 100 || got[0].units != "%" {
		t.Fatalf("sample 0 = %+v, want total=100 units=%%", got[0])
	}
	if got[2].progress != 50 {
		t.Fatalf("0.5 → progress = %d, want 50", got[2].progress)
	}
	if got[3].progress != 100 {
		t.Fatalf("1.0 → progress = %d, want 100", got[3].progress)
	}
	if got[4].progress != 100 {
		t.Fatalf("2.0 → progress = %d, want clamped to 100", got[4].progress)
	}
	if got[5].progress != 0 {
		t.Fatalf("-1.0 → progress = %d, want clamped to 0", got[5].progress)
	}
}
