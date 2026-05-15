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

/*
#cgo CXXFLAGS: -std=c++17
#cgo CFLAGS:   -std=c11
#cgo CPPFLAGS: -I${SRCDIR}/llama.cpp/include
#cgo CPPFLAGS: -I${SRCDIR}/llama.cpp/ggml/include
#cgo CPPFLAGS: -I${SRCDIR}/llama.cpp/common
#cgo CPPFLAGS: -I${SRCDIR}/llama.cpp/tools/mtmd
#cgo CPPFLAGS: -I${SRCDIR}/llama.cpp/tools/server
#cgo CPPFLAGS: -I${SRCDIR}/llama.cpp/vendor
#cgo LDFLAGS:  -L${SRCDIR}/libs
// Link order matters with GNU ld (static libs): llama-common depends on
// llama, jinja, cpp-httplib. httplib is mostly dead-TU but keeping it in
// LDFLAGS keeps the link deterministic across compilers.
#cgo LDFLAGS:  -lserver-context -lllama-common -lllama-common-base -lcpp-httplib
#cgo LDFLAGS:  -lmtmd
#cgo LDFLAGS:  -lllama -lggml -lggml-base -lggml-cpu
#cgo LDFLAGS:  -lstdc++ -lm

#cgo darwin LDFLAGS:  -lggml-blas -lggml-metal
#cgo darwin LDFLAGS:  -framework Accelerate -framework Foundation -framework Metal -framework MetalKit -framework MetalPerformanceShaders

// On Linux, ggml-cpu's OpenMP-parallelised paths require linking against
// GCC's GNU OpenMP runtime (libgomp). Without this, ld fails with
// undefined references to GOMP_barrier / GOMP_parallel from ggml-cpu.c.
#cgo linux LDFLAGS: -lgomp

// On linux, BLAS comes from OpenBLAS via the GGML BLAS backend. The
// llamacpp_blas build tag opts the binary into linking it; the static
// libggml-blas.a is built only when LLAMACPP_BLAS=1 is passed to the
// llamacpp Makefile.
// libgfortran is a transitive runtime dep of OpenBLAS on Linux: openblas
// dispatches into LAPACK routines that use Fortran intrinsics like
// _gfortran_etime / _gfortran_concat_string. We must link it explicitly
// because GNU ld won't pull a shared lib's transitive deps automatically
// when the symbols are referenced through a versioned (@GFORTRAN_8) tag.
#cgo linux,llamacpp_blas LDFLAGS: -lggml-blas -lopenblas -lgfortran

#cgo llamacpp_cuda LDFLAGS: -lggml-cuda -lcudart -lcublas -lcuda

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include "llama.h"
#include "ggml.h"
#include "common_chat_wrap.h"
#include "mtmd_wrap.h"

// Thin C shims that wrap bits of the llama.h API which are awkward to call
// directly from cgo (var-length array fields, bit-flag enums, etc.).

// llamacpp_zalloc is calloc with an explicit total size, callable from
// Go without the cgo preprocessor tripping over the dual-arg calloc
// prototype. Returned memory is zeroed; callers must C.free().
static void *llamacpp_zalloc(size_t size) {
    return calloc(1, size);
}

// Append a single token to a batch previously allocated with llama_batch_init.
// seq_id 0 is used; logits flags controls whether logits will be emitted.
static void llamacpp_batch_add(
        struct llama_batch * b,
        int32_t  token,
        int32_t  pos,
        int32_t  seq_id,
        int      logits) {
    int32_t i = b->n_tokens;
    b->token[i]     = (llama_token)token;
    b->pos[i]       = (llama_pos)pos;
    b->n_seq_id[i]  = 1;
    b->seq_id[i][0] = (llama_seq_id)seq_id;
    b->logits[i]    = (int8_t)logits;
    b->n_tokens    += 1;
}

// Forward declarations so cgo can generate stubs for the Go-implemented callbacks.
extern void llamacppLogCallback(int level, char *text);
extern int  llamacppProgressCallback(float progress, uintptr_t user_data);

static void llamacpp_log_trampoline(enum ggml_log_level level, const char * text, void * user_data) {
    (void)user_data;
    if (text == NULL) {
        return;
    }
    // Cast away const — the callback only reads the string.
    llamacppLogCallback((int)level, (char *)text);
}

static void llamacpp_install_log_callback(void) {
    llama_log_set(llamacpp_log_trampoline, NULL);
}

static bool llamacpp_progress_trampoline(float progress, void * user_data) {
    if (user_data == NULL) {
        return true;
    }
    uintptr_t handle = *((uintptr_t *) user_data);
    return llamacppProgressCallback(progress, handle) != 0;
}

static void llamacpp_set_progress_callback(struct llama_model_params * params, uintptr_t user_data) {
    uintptr_t * boxed = (uintptr_t *) malloc(sizeof(uintptr_t));
    *boxed = user_data;
    params->progress_callback = llamacpp_progress_trampoline;
    params->progress_callback_user_data = (void *) boxed;
}

static void llamacpp_free_progress_callback_user_data(void * user_data) {
    free(user_data);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"runtime/cgo"
	"strings"
	"sync"
	"unicode/utf8"
	"unsafe"
)

var backendOnce sync.Once

// Init initializes the llama.cpp backend. Safe to call multiple times — the
// underlying library is only initialised on the first call. All public entry
// points that require the backend call this implicitly.
//
// The backend is not explicitly freed: llama_backend_free is only meaningful
// on process exit, at which point the OS reclaims everything. Leaking it is
// both simpler and avoids races with goroutines that might still be using
// llama.cpp state during shutdown.
func Init() {
	backendOnce.Do(func() {
		C.llamacpp_install_log_callback()
		C.llama_backend_init()
	})
}

// LogLevel mirrors ggml_log_level.
type LogLevel int

// Logger receives log messages from llama.cpp. Set via SetLogger.
type Logger func(level LogLevel, message string)

var (
	loggerMu sync.RWMutex
	logger   Logger = defaultLogger
)

func defaultLogger(LogLevel, string) {}

// SetLogger installs a Go-side logger that receives all llama.cpp log output.
// Pass nil to silence the library.
func SetLogger(l Logger) {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	if l == nil {
		logger = defaultLogger
		return
	}
	logger = l
}

//export llamacppLogCallback
func llamacppLogCallback(level C.int, text *C.char) {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()
	l(LogLevel(level), strings.TrimRight(C.GoString(text), "\n"))
}

//export llamacppProgressCallback
func llamacppProgressCallback(progress C.float, userData C.uintptr_t) C.int {
	if userData == 0 {
		return 1
	}
	h := cgo.Handle(userData)
	cb, ok := h.Value().(func(progress, total int64, units string))
	if !ok || cb == nil {
		return 1
	}
	p := int64(progress * 100)
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	cb(p, 100, "%")
	return 1
}

// ModelParams configures model loading.
type ModelParams struct {
	// NGPULayers is the number of layers to offload to GPU. 0 = CPU-only.
	// A negative value offloads everything the backend supports.
	NGPULayers int
	// UseMMAP enables memory-mapped weights when supported.
	UseMMAP bool
	// UseMLock locks weights into memory.
	UseMLock bool
	// VocabOnly loads only the vocabulary (useful for tokenization).
	VocabOnly bool
	// Progress receives model-load progress in [0,total]. total is always
	// 100 and units is "%". Intended for UI feedback during heavyweight
	// local model loads.
	Progress func(progress, total int64, units string)
}

// DefaultModelParams returns safe defaults.
func DefaultModelParams() ModelParams {
	return ModelParams{
		NGPULayers: -1,
		UseMMAP:    true,
	}
}

// Model is a loaded llama.cpp model.
type Model struct {
	c    *C.struct_llama_model
	mctx *C.rune_mtmd_context
}

// LoadModel loads a GGUF model from disk.
func LoadModel(path string, projectorPath string, p ModelParams) (*Model, error) {
	Init()

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	cparams := C.llama_model_default_params()
	cparams.n_gpu_layers = C.int32_t(p.NGPULayers)
	cparams.use_mmap = C.bool(p.UseMMAP)
	cparams.use_mlock = C.bool(p.UseMLock)
	cparams.vocab_only = C.bool(p.VocabOnly)
	var progressHandle cgo.Handle
	if p.Progress != nil {
		progressHandle = cgo.NewHandle(p.Progress)
		defer progressHandle.Delete()
		C.llamacpp_set_progress_callback(&cparams, C.uintptr_t(progressHandle))
		defer C.llamacpp_free_progress_callback_user_data(cparams.progress_callback_user_data)
	}

	m := C.llama_model_load_from_file(cPath, cparams)
	if m == nil {
		return nil, fmt.Errorf("llamacpp: failed to load model %q", path)
	}
	model := &Model{c: m}
	if projectorPath != "" {
		cProj := C.CString(projectorPath)
		defer C.free(unsafe.Pointer(cProj))
		mctx := C.rune_mtmd_init(
			cProj,
			m,
			1,
			4,
			0,
			1,
			-1,
			-1,
		)
		if mctx == nil {
			C.llama_model_free(m)
			return nil, fmt.Errorf("llamacpp: failed to load multimodal projector %q", projectorPath)
		}
		model.mctx = mctx
	}
	return model, nil
}

// Close frees the model.
func (m *Model) Close() {
	if m == nil || m.c == nil {
		return
	}
	if m.mctx != nil {
		C.rune_mtmd_free(m.mctx)
		m.mctx = nil
	}
	C.llama_model_free(m.c)
	m.c = nil
}

// NCtxTrain returns the training context length of the model.
func (m *Model) NCtxTrain() int {
	return int(C.llama_model_n_ctx_train(m.c))
}

// NParams returns the number of parameters of the model.
func (m *Model) NParams() uint64 {
	return uint64(C.llama_model_n_params(m.c))
}

// Desc returns a short human-readable description of the model.
func (m *Model) Desc() string {
	const maxLen = 256
	buf := make([]byte, maxLen)
	n := C.llama_model_desc(m.c, (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(maxLen))
	if n <= 0 {
		return ""
	}
	return string(buf[:int(n)])
}

// ChatTemplate returns the model's embedded chat template, or an empty string
// if the model does not define one. The optional name selects a variant
// (e.g. "tool_use"); pass "" to get the default template.
func (m *Model) ChatTemplate(name string) string {
	var cName *C.char
	if name != "" {
		cName = C.CString(name)
		defer C.free(unsafe.Pointer(cName))
	}
	s := C.llama_model_chat_template(m.c, cName)
	if s == nil {
		return ""
	}
	return C.GoString(s)
}

// vocab returns a handle to the model's vocabulary.
func (m *Model) vocab() *C.struct_llama_vocab {
	return C.llama_model_get_vocab(m.c)
}

// Tokenize converts text into tokens. If addSpecial is true, BOS/EOS tokens
// are added as appropriate for the model.
//
// The input must be valid UTF-8: llama.cpp's tokenizer runs Unicode
// normalisation on every byte and crashes (SIGSEGV inside unicode.cpp)
// on a handful of invalid byte sequences. We reject those at the cgo
// boundary so callers see a deterministic Go error instead of taking
// the whole process down.
func (m *Model) Tokenize(text string, addSpecial, parseSpecial bool) ([]int32, error) {
	if !utf8.ValidString(text) {
		return nil, errors.New("llamacpp: Tokenize input is not valid UTF-8")
	}
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	// First attempt: guess a reasonable upper bound and grow if needed.
	n := len(text) + 8
	if n < 16 {
		n = 16
	}
	buf := make([]C.llama_token, n)

	r := C.llama_tokenize(
		m.vocab(),
		cText,
		C.int32_t(len(text)),
		&buf[0],
		C.int32_t(n),
		C.bool(addSpecial),
		C.bool(parseSpecial),
	)
	if r < 0 {
		need := int(-r)
		buf = make([]C.llama_token, need)
		r = C.llama_tokenize(
			m.vocab(),
			cText,
			C.int32_t(len(text)),
			&buf[0],
			C.int32_t(need),
			C.bool(addSpecial),
			C.bool(parseSpecial),
		)
		if r < 0 {
			return nil, fmt.Errorf("llamacpp: tokenize failed (need=%d)", -int(r))
		}
	}

	out := make([]int32, int(r))
	for i := range out {
		out[i] = int32(buf[i])
	}
	return out, nil
}

// TokenToPiece converts a single token id into its textual piece.
func (m *Model) TokenToPiece(token int32, special bool) string {
	const initial = 32
	buf := make([]byte, initial)
	n := C.llama_token_to_piece(
		m.vocab(),
		C.llama_token(token),
		(*C.char)(unsafe.Pointer(&buf[0])),
		C.int32_t(initial),
		0,
		C.bool(special),
	)
	if n < 0 {
		need := int(-n)
		buf = make([]byte, need)
		n = C.llama_token_to_piece(
			m.vocab(),
			C.llama_token(token),
			(*C.char)(unsafe.Pointer(&buf[0])),
			C.int32_t(need),
			0,
			C.bool(special),
		)
		if n < 0 {
			return ""
		}
	}
	return string(buf[:int(n)])
}

// IsEOG reports whether the token is an end-of-generation token.
func (m *Model) IsEOG(token int32) bool {
	return bool(C.llama_vocab_is_eog(m.vocab(), C.llama_token(token)))
}

// HasProjector reports whether a multimodal projector (mmproj) is loaded
// alongside this model.
func (m *Model) HasProjector() bool {
	return m != nil && m.mctx != nil
}

// EvalMultimodalPrompt tokenizes and evaluates a multimodal prompt with
// optional image payloads, advancing the KV cache in ctx. Returns the new
// n_past (absolute position after the last evaluated chunk) and the number
// of tokens consumed. If logitsLast is true, logits will be emitted for the
// last evaluated token so the sampler can draw from them.
func (m *Model) EvalMultimodalPrompt(
	ctx *Context,
	prompt string,
	files [][]byte,
	nBatch int,
	seqID int,
	nPast int,
	logitsLast bool,
) (newNPast int, nTokens int, err error) {
	if m.mctx == nil {
		return 0, 0, errors.New("llamacpp: model has no multimodal projector")
	}
	cPrompt := C.CString(prompt)
	defer C.free(unsafe.Pointer(cPrompt))

	// cgo forbids passing a Go-allocated struct array that contains Go
	// pointers (here, each rune_mtmd_file.data). Allocate the array and
	// every data buffer in C memory so the whole thing is opaque to the Go
	// pointer checker.
	var cFiles *C.rune_mtmd_file
	var cFileBufs []unsafe.Pointer
	if len(files) > 0 {
		arrSize := C.size_t(len(files)) * C.size_t(unsafe.Sizeof(C.rune_mtmd_file{}))
		arrBase := C.malloc(arrSize)
		defer C.free(arrBase)
		arrSlice := unsafe.Slice((*C.rune_mtmd_file)(arrBase), len(files))
		// Zero the array so skipped entries (empty payloads) are null/0.
		for i := range arrSlice {
			arrSlice[i] = C.rune_mtmd_file{}
		}
		cFileBufs = make([]unsafe.Pointer, 0, len(files))
		for i, data := range files {
			if len(data) == 0 {
				continue
			}
			// C.CBytes is available via "C" but we inline the malloc+copy
			// so we stay within C memory and avoid runtime.CBytes pointer
			// classification checks.
			buf := C.malloc(C.size_t(len(data)))
			bufSlice := unsafe.Slice((*byte)(buf), len(data))
			copy(bufSlice, data)
			cFileBufs = append(cFileBufs, buf)
			arrSlice[i].data = (*C.uchar)(buf)
			arrSlice[i].len = C.size_t(len(data))
		}
		cFiles = (*C.rune_mtmd_file)(arrBase)
		defer func() {
			for _, p := range cFileBufs {
				C.free(p)
			}
		}()
	}

	var (
		newPast C.int
		nToks   C.int
		nPos    C.int
		errOut  *C.char
	)
	rc := C.rune_mtmd_eval_prompt(
		m.mctx,
		ctx.c,
		cPrompt,
		cFiles,
		C.size_t(len(files)),
		C.int(nBatch),
		C.int(seqID),
		C.int(nPast),
		boolToCInt(logitsLast),
		&newPast,
		&nToks,
		&nPos,
		&errOut,
	)
	if rc != 0 {
		msg := "rune_mtmd_eval_prompt failed"
		if errOut != nil {
			msg = C.GoString(errOut)
			C.free(unsafe.Pointer(errOut))
		}
		return 0, 0, fmt.Errorf("llamacpp: mtmd eval: %s", msg)
	}
	return int(newPast), int(nToks), nil
}

// ApplyChatTemplate formats messages using the model's embedded Jinja chat
// template (or the given override). Returns the formatted prompt string
// that should be tokenized and fed to the context.
//
// Implementation: calls into llama.cpp's common_chat_templates_* Jinja
// engine via the rune_chat_apply_jinja wrapper. The legacy
// llama_chat_apply_template() is kept as a fallback for the rare case
// where the Jinja path fails (e.g. template referring to runtime hooks we
// do not feed) or when the caller sets RUNE_LLAMACPP_FORCE_LEGACY_CHAT=1.
func (m *Model) ApplyChatTemplate(tmpl string, msgs []ChatMessage, addAssistant bool) (string, error) {
	// Resolve template once up front so diagnostics can include it, and so
	// the legacy fallback sees the same source string as the Jinja path.
	if tmpl == "" {
		tmpl = m.ChatTemplate("")
	}

	forceLegacy := os.Getenv("RUNE_LLAMACPP_FORCE_LEGACY_CHAT") == "1"

	if !forceLegacy {
		out, err := m.applyChatTemplateJinja(tmpl, msgs, addAssistant)
		if err == nil {
			return out, nil
		}
		// Fall through to legacy only when the Jinja engine itself could
		// not process the template. Other errors (nil model etc.) surface
		// directly because legacy can't fix them either.
		if !errors.Is(err, errJinjaTemplateUnsupported) {
			return "", err
		}
	}

	if tmpl == "" {
		return "", errors.New("llamacpp: no chat template available")
	}
	return m.applyChatTemplateLegacy(tmpl, msgs, addAssistant)
}

// errJinjaTemplateUnsupported is returned by applyChatTemplateJinja when
// the Jinja engine cannot render the template. ApplyChatTemplate treats
// this as a signal to try the legacy path; other errors short-circuit.
var errJinjaTemplateUnsupported = errors.New("llamacpp: jinja template unsupported")

// applyChatTemplateJinja drives the Jinja path via rune_chat_apply_jinja.
// An empty tmpl means "use the model's embedded template".
func (m *Model) applyChatTemplateJinja(tmpl string, msgs []ChatMessage, addAssistant bool) (string, error) {
	// Re-use the rune_chat_template_open / _prompt path so this code path
	// stays a thin compatibility shim over the streaming template handle.
	h, err := m.OpenChatTemplate(msgs, ChatTemplateOptions{
		TemplateOverride:    tmpl,
		AddGenerationPrompt: addAssistant,
	})
	if err != nil {
		// OpenChatTemplate wraps llama.cpp errors; treat them as the Jinja
		// engine refusing the template so ApplyChatTemplate can fall back
		// to the legacy path.
		return "", fmt.Errorf("%w: %v", errJinjaTemplateUnsupported, err)
	}
	defer h.Close()
	return h.Prompt()
}

// applyChatTemplateLegacy runs the pre-Jinja llama_chat_apply_template()
// path. Kept as a fallback because a handful of templates (and unit tests
// that use short aliases like "chatml") still rely on the legacy detector.
func (m *Model) applyChatTemplateLegacy(tmpl string, msgs []ChatMessage, addAssistant bool) (string, error) {
	cTmpl := C.CString(tmpl)
	defer C.free(unsafe.Pointer(cTmpl))

	cmsgsPtr, freeMsgs := buildCLegacyChatMessages(msgs)
	defer freeMsgs()

	// Size-first: pass NULL/0 to discover the required buffer length.
	// llama_chat_apply_template returns the required size when buflen is
	// too small (same contract as snprintf).
	r := C.llama_chat_apply_template(
		cTmpl,
		cmsgsPtr,
		C.size_t(len(msgs)),
		C.bool(addAssistant),
		nil,
		0,
	)
	if r < 0 {
		return "", fmt.Errorf("llamacpp: chat template failed (%d)", int(r))
	}
	if r == 0 {
		return "", nil
	}
	buf := make([]byte, int(r))
	r = C.llama_chat_apply_template(
		cTmpl,
		cmsgsPtr,
		C.size_t(len(msgs)),
		C.bool(addAssistant),
		(*C.char)(unsafe.Pointer(&buf[0])),
		C.int32_t(len(buf)),
	)
	if r < 0 {
		return "", fmt.Errorf("llamacpp: chat template failed (%d)", int(r))
	}
	return string(buf[:int(r)]), nil
}

// buildCChatMessages converts []ChatMessage into a contiguous C array of
// rune_chat_message and returns a cleanup func the caller must invoke.
// The returned pointer is valid until cleanup runs.
func buildCRichChatMessages(msgs []ChatMessage) (*C.struct_rune_chat_message, func()) {
	n := len(msgs)
	if n == 0 {
		return nil, func() {}
	}
	// calloc, NOT malloc: optional fields (content_parts, n_content_parts,
	// tool_calls, n_tool_calls) are only assigned when the corresponding
	// Go-side slice is non-empty. A plain C.malloc returns uninitialized
	// memory, so messages without those parts would inherit whatever
	// bytes happened to live there, and the C-side to_common_chat_msg
	// dereferences (msg.content_parts + i) using msg.n_content_parts as
	// the bound — a non-zero garbage count paired with a wild pointer
	// crashes the process. Zeroing the whole array up front guarantees
	// every untouched field is a clean nil/0.
	msgSize := C.size_t(n) * C.size_t(unsafe.Sizeof(C.struct_rune_chat_message{}))
	base := C.llamacpp_zalloc(msgSize)
	if base == nil {
		return nil, func() {}
	}
	cmsgs := unsafe.Slice((*C.struct_rune_chat_message)(base), n)
	keep := make([]unsafe.Pointer, 0, 8*n+1)
	keep = append(keep, base)
	for i, msg := range msgs {
		rp := unsafe.Pointer(C.CString(msg.Role))
		cp := unsafe.Pointer(C.CString(msg.Content))
		rcp := unsafe.Pointer(C.CString(msg.ReasoningContent))
		np := unsafe.Pointer(C.CString(msg.Name))
		tcp := unsafe.Pointer(C.CString(msg.ToolCallID))
		keep = append(keep, rp, cp, rcp, np, tcp)
		cmsgs[i].role = (*C.char)(rp)
		cmsgs[i].content = (*C.char)(cp)
		cmsgs[i].reasoning_content = (*C.char)(rcp)
		cmsgs[i].tool_name = (*C.char)(np)
		cmsgs[i].tool_call_id = (*C.char)(tcp)

		if len(msg.ContentParts) > 0 {
			// Zero-initialized: any per-part field we omit (none today,
			// but defended against future drift) must read as nil/0 on
			// the C side.
			partsSize := C.size_t(len(msg.ContentParts)) * C.size_t(unsafe.Sizeof(C.struct_rune_chat_content_part{}))
			partsBase := C.llamacpp_zalloc(partsSize)
			keep = append(keep, partsBase)
			parts := unsafe.Slice((*C.struct_rune_chat_content_part)(partsBase), len(msg.ContentParts))
			for j, part := range msg.ContentParts {
				tp := unsafe.Pointer(C.CString(part.Type))
				xp := unsafe.Pointer(C.CString(part.Text))
				keep = append(keep, tp, xp)
				parts[j].kind = (*C.char)(tp)
				parts[j].text = (*C.char)(xp)
			}
			cmsgs[i].content_parts = &parts[0]
			cmsgs[i].n_content_parts = C.size_t(len(parts))
		}

		if len(msg.ToolCalls) > 0 {
			// Zero-initialized: see comment on the message-array allocation.
			callsSize := C.size_t(len(msg.ToolCalls)) * C.size_t(unsafe.Sizeof(C.struct_rune_chat_tool_call{}))
			callsBase := C.llamacpp_zalloc(callsSize)
			keep = append(keep, callsBase)
			calls := unsafe.Slice((*C.struct_rune_chat_tool_call)(callsBase), len(msg.ToolCalls))
			for j, call := range msg.ToolCalls {
				np := unsafe.Pointer(C.CString(call.Name))
				ap := unsafe.Pointer(C.CString(call.Arguments))
				ip := unsafe.Pointer(C.CString(call.ID))
				keep = append(keep, np, ap, ip)
				calls[j].name = (*C.char)(np)
				calls[j].arguments = (*C.char)(ap)
				calls[j].id = (*C.char)(ip)
			}
			cmsgs[i].tool_calls = &calls[0]
			cmsgs[i].n_tool_calls = C.size_t(len(calls))
		}
	}
	cleanup := func() {
		for _, p := range keep {
			C.free(p)
		}
	}
	return (*C.struct_rune_chat_message)(base), cleanup
}

// buildCLegacyChatMessages converts []ChatMessage into the flat legacy
// llama_chat_message representation used by llama_chat_apply_template().
func buildCLegacyChatMessages(msgs []ChatMessage) (*C.struct_llama_chat_message, func()) {
	n := len(msgs)
	if n == 0 {
		return nil, func() {}
	}
	cmsgs := make([]C.struct_llama_chat_message, n)
	keep := make([]unsafe.Pointer, 0, 2*n)
	for i, msg := range msgs {
		rp := unsafe.Pointer(C.CString(msg.Role))
		cp := unsafe.Pointer(C.CString(msg.Content))
		keep = append(keep, rp, cp)
		cmsgs[i].role = (*C.char)(rp)
		cmsgs[i].content = (*C.char)(cp)
	}
	cleanup := func() {
		for _, p := range keep {
			C.free(p)
		}
	}
	return &cmsgs[0], cleanup
}

func boolToCInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// ChatContentPart is the Go-side subset of upstream common_chat_msg_content_part.
// For now we only preserve text parts across the Go/C bridge.
type ChatContentPart struct {
	Type string
	Text string
}

// ChatToolCall is the Go-side subset of upstream common_chat_tool_call.
type ChatToolCall struct {
	Name      string
	Arguments string
	ID        string
}

// ChatMessage is the richer message shape passed to the upstream Jinja chat
// template path. It mirrors the subset of llama.cpp/common/chat.h that our
// llmapi.Message model can express.
type ChatMessage struct {
	Role             string
	Content          string
	ContentParts     []ChatContentPart
	ToolCalls        []ChatToolCall
	ReasoningContent string
	Name             string
	ToolCallID       string
}

// ContextParams configures a Context.
type ContextParams struct {
	// NCtx is the requested context window. 0 means use the model default.
	NCtx uint32
	// NBatch is the logical max batch size submitted to Decode.
	NBatch uint32
	// NUBatch is the physical (micro) batch size.
	NUBatch uint32
	// NThreads is the thread count for generation. 0 = auto.
	NThreads int
	// NThreadsBatch is the thread count for prompt/batch processing. 0 = auto.
	NThreadsBatch int
	// FlashAttention enables flash attention (auto-detected by default).
	FlashAttention bool
}

// DefaultContextParams returns sensible defaults.
func DefaultContextParams() ContextParams {
	return ContextParams{
		NCtx:    0,
		NBatch:  2048,
		NUBatch: 512,
	}
}

// Context is an inference context bound to a Model.
type Context struct {
	c       *C.struct_llama_context
	model   *Model
	nCtx    int
	nBatch  int
	threads int
}

// NewContext allocates a fresh inference context.
func NewContext(m *Model, p ContextParams) (*Context, error) {
	Init()

	cparams := C.llama_context_default_params()
	if p.NCtx > 0 {
		cparams.n_ctx = C.uint32_t(p.NCtx)
	}
	if p.NBatch > 0 {
		cparams.n_batch = C.uint32_t(p.NBatch)
	}
	if p.NUBatch > 0 {
		cparams.n_ubatch = C.uint32_t(p.NUBatch)
	}
	if p.NThreads > 0 {
		cparams.n_threads = C.int32_t(p.NThreads)
	}
	if p.NThreadsBatch > 0 {
		cparams.n_threads_batch = C.int32_t(p.NThreadsBatch)
	}
	if p.FlashAttention {
		cparams.flash_attn_type = C.LLAMA_FLASH_ATTN_TYPE_ENABLED
	}

	c := C.llama_init_from_model(m.c, cparams)
	if c == nil {
		return nil, errors.New("llamacpp: failed to create context")
	}
	return &Context{
		c:       c,
		model:   m,
		nCtx:    int(C.llama_n_ctx(c)),
		nBatch:  int(cparams.n_batch),
		threads: int(C.llama_n_threads(c)),
	}, nil
}

// Close frees the context.
func (c *Context) Close() {
	if c == nil || c.c == nil {
		return
	}
	C.llama_free(c.c)
	c.c = nil
}

// NCtx returns the effective context window size.
func (c *Context) NCtx() int { return c.nCtx }

// NBatch returns the effective batch size.
func (c *Context) NBatch() int { return c.nBatch }

// Model returns the model bound to this context.
func (c *Context) Model() *Model { return c.model }

// ClearKV clears the KV cache for all sequences.
func (c *Context) ClearKV() {
	C.llama_memory_clear(C.llama_get_memory(c.c), C.bool(true))
}

// RemoveKVRange removes KV entries in the half-open position range [p0, p1)
// for the given sequence. Pass p1 < 0 to mean "through infinity" (drop the
// tail). Returns false when the backing cache cannot perform a partial
// removal — the caller should fall back to ClearKV in that case.
func (c *Context) RemoveKVRange(seqID, p0, p1 int) bool {
	return bool(C.llama_memory_seq_rm(
		C.llama_get_memory(c.c),
		C.llama_seq_id(seqID),
		C.llama_pos(p0),
		C.llama_pos(p1),
	))
}

// ShiftKVRange offsets every cached position in [p0, p1) for the given
// sequence by `delta`. This mirrors llama-server's n_cache_reuse path:
// after llama_memory_seq_rm clears the future range, seq_add re-bases
// the surviving chunk so the existing KV entries align with the new
// prompt positions.
func (c *Context) ShiftKVRange(seqID, p0, p1, delta int) {
	C.llama_memory_seq_add(
		C.llama_get_memory(c.c),
		C.llama_seq_id(seqID),
		C.llama_pos(p0),
		C.llama_pos(p1),
		C.llama_pos(delta),
	)
}

// CanShiftKV reports whether the backing memory implementation supports
// position shifts. Required for the n_cache_reuse optimisation.
func (c *Context) CanShiftKV() bool {
	return bool(C.llama_memory_can_shift(C.llama_get_memory(c.c)))
}

// Batch wraps a llama_batch for submission to Decode.
type Batch struct {
	c        C.struct_llama_batch
	capacity int
}

// NewBatch allocates a token batch with the given capacity.
func NewBatch(capacity int) *Batch {
	cb := C.llama_batch_init(C.int32_t(capacity), 0, 1)
	return &Batch{c: cb, capacity: capacity}
}

// Close frees the batch.
func (b *Batch) Close() {
	if b == nil {
		return
	}
	C.llama_batch_free(b.c)
}

// Clear resets the batch to hold zero tokens without reallocating.
func (b *Batch) Clear() { b.c.n_tokens = 0 }

// Len returns the number of tokens currently in the batch.
func (b *Batch) Len() int { return int(b.c.n_tokens) }

// Capacity returns the maximum number of tokens the batch can hold.
func (b *Batch) Capacity() int { return b.capacity }

// Add appends a token to the batch at position pos. If logits is true, the
// token's logits will be available after Decode.
func (b *Batch) Add(token int32, pos int, logits bool) {
	l := 0
	if logits {
		l = 1
	}
	C.llamacpp_batch_add(&b.c, C.int32_t(token), C.int32_t(pos), 0, C.int(l))
}

// ErrKVCacheFull is returned by Decode when the KV cache cannot fit the batch.
var ErrKVCacheFull = errors.New("llamacpp: kv cache full")

// Decode runs a forward pass over the batch. Callers must submit tokens in
// strictly increasing position order.
func (c *Context) Decode(b *Batch) error {
	r := C.llama_decode(c.c, b.c)
	switch {
	case r < 0:
		return fmt.Errorf("llamacpp: decode failed (%d)", int(r))
	case r == 1:
		return ErrKVCacheFull
	}
	return nil
}

// SamplerParams configures a sampler chain.
type SamplerParams struct {
	// Seed for deterministic sampling. 0 = random.
	Seed uint32
	// Temperature. 0 disables (falls back to greedy).
	Temperature float32
	// TopK. 0 disables.
	TopK int
	// TopP. 0 or >=1 disables.
	TopP float32
	// MinP. 0 disables.
	MinP float32
	// RepeatPenalty. 1.0 disables.
	RepeatPenalty float32
	// RepeatLastN is how many recent tokens to consider for repeat penalty.
	RepeatLastN int

	// Additional sampler knobs mirroring common_params_sampling. Leave
	// at zero to inherit llama.cpp's upstream default.

	// FreqPenalty (penalty_freq) — 0 disables.
	FreqPenalty float32
	// PresencePenalty (penalty_present) — 0 disables.
	PresencePenalty float32
	// TypicalP (typ_p) — 1 disables. Use HasTypicalP to override the upstream default.
	TypicalP    float32
	HasTypicalP bool
	// TopNSigma — -1 disables. Use HasTopNSigma to override.
	TopNSigma    float32
	HasTopNSigma bool
	// Mirostat — 0 disables, 1 = Mirostat v1, 2 = v2.
	Mirostat int32
	// MirostatTau / MirostatEta. Use HasMirostatTau / HasMirostatEta to override defaults.
	MirostatTau    float32
	HasMirostatTau bool
	MirostatEta    float32
	HasMirostatEta bool
	// DynaTempRange — 0 disables. DynaTempExponent only takes effect when range > 0.
	DynaTempRange    float32
	DynaTempExponent float32
	// XtcProbability — 0 disables. XtcThreshold > 0.5 disables XTC.
	XtcProbability float32
	XtcThreshold   float32
	// DryMultiplier — 0 disables. DryBase / DryAllowedLength / DryPenaltyLastN
	// require their Has* flag to override the upstream defaults.
	DryMultiplier       float32
	DryBase             float32
	HasDryBase          bool
	DryAllowedLength    int32
	HasDryAllowedLength bool
	DryPenaltyLastN     int32
	HasDryPenaltyLastN  bool
}

// DefaultSamplerParams returns the llama.cpp defaults (close to `main -ngl 99`).
func DefaultSamplerParams() SamplerParams {
	return SamplerParams{
		Seed:          0xFFFFFFFF, // LLAMA_DEFAULT_SEED
		Temperature:   0.8,
		TopK:          40,
		TopP:          0.95,
		MinP:          0.05,
		RepeatPenalty: 1.0,
		RepeatLastN:   64,
	}
}

// Sampler is a token sampler backed by llama.cpp common_sampler. It can be
// initialized with a ChatTemplateHandle so template-produced tool-call grammar
// controls are applied during decoding, matching llama-server's generation
// path.
type Sampler struct {
	c *C.rune_sampler
}

// NewSampler constructs a sampler for the given model and optional chat
// template handle. When tmpl is non-nil, the sampler receives the same
// grammar/lazy-trigger/generation-prompt controls that llama-server derives
// from common_chat_templates_apply.
func NewSampler(m *Model, p SamplerParams, tmpl *ChatTemplateHandle) (*Sampler, error) {
	sp := C.struct_rune_sampler_params{
		seed:           C.uint32_t(p.Seed),
		temperature:    C.float(p.Temperature),
		top_k:          C.int32_t(p.TopK),
		top_p:          C.float(p.TopP),
		min_p:          C.float(p.MinP),
		repeat_penalty: C.float(p.RepeatPenalty),
		repeat_last_n:  C.int32_t(p.RepeatLastN),

		freq_penalty:           C.float(p.FreqPenalty),
		presence_penalty:       C.float(p.PresencePenalty),
		typical_p:              C.float(p.TypicalP),
		top_n_sigma:            C.float(p.TopNSigma),
		mirostat:               C.int32_t(p.Mirostat),
		mirostat_tau:           C.float(p.MirostatTau),
		mirostat_eta:           C.float(p.MirostatEta),
		dynatemp_range:         C.float(p.DynaTempRange),
		dynatemp_exponent:      C.float(p.DynaTempExponent),
		xtc_probability:        C.float(p.XtcProbability),
		xtc_threshold:          C.float(p.XtcThreshold),
		dry_multiplier:         C.float(p.DryMultiplier),
		dry_base:               C.float(p.DryBase),
		dry_allowed_length:     C.int32_t(p.DryAllowedLength),
		dry_penalty_last_n:     C.int32_t(p.DryPenaltyLastN),
		has_typical_p:          boolToCInt(p.HasTypicalP),
		has_top_n_sigma:        boolToCInt(p.HasTopNSigma),
		has_mirostat_tau:       boolToCInt(p.HasMirostatTau),
		has_mirostat_eta:       boolToCInt(p.HasMirostatEta),
		has_dry_base:           boolToCInt(p.HasDryBase),
		has_dry_allowed_length: boolToCInt(p.HasDryAllowedLength),
		has_dry_penalty_last_n: boolToCInt(p.HasDryPenaltyLastN),
	}
	var cTmpl *C.rune_chat_template
	if tmpl != nil {
		cTmpl = tmpl.c
	}
	var errOut *C.char
	c := C.rune_sampler_new(m.c, &sp, cTmpl, &errOut)
	if c == nil {
		msg := "rune_sampler_new returned null"
		if errOut != nil {
			msg = C.GoString(errOut)
			C.free(unsafe.Pointer(errOut))
		}
		return nil, fmt.Errorf("llamacpp: new sampler: %s", msg)
	}
	return &Sampler{c: c}, nil
}

// Close frees the sampler chain.
func (s *Sampler) Close() {
	if s == nil || s.c == nil {
		return
	}
	C.rune_sampler_free(s.c)
	s.c = nil
}

// Sample draws a token id from the logits of the last token of the context.
// Pass idx = -1 to use the most recent logits.
func (s *Sampler) Sample(ctx *Context, idx int) int32 {
	return int32(C.rune_sampler_sample(s.c, ctx.c, C.int32_t(idx), 0))
}

// Accept records the chosen token so the sampler can update its state
// (e.g. repeat penalty, mirostat, grammar).
func (s *Sampler) Accept(token int32) {
	C.rune_sampler_accept(s.c, C.int32_t(token), 1)
}

// AcceptPrompt records a prompt token in the sampler chain without advancing
// any generation grammar. This mirrors llama-server's server_slot::init_sampler.
func (s *Sampler) AcceptPrompt(token int32) {
	C.rune_sampler_accept(s.c, C.int32_t(token), 0)
}

// Reset resets per-sequence sampler state.
func (s *Sampler) Reset() { C.rune_sampler_reset(s.c) }

// findPartialStop returns the byte offset where a non-empty prefix of
// `stop` ends `text`, or -1 when there is no such prefix. Wraps the
// upstream string_find_partial_stop primitive so Go-side stop handling
// stays in lockstep with llama-server.
func findPartialStop(text, stop string) int {
	if text == "" || stop == "" {
		return -1
	}
	cText := (*C.char)(unsafe.Pointer(unsafe.StringData(text)))
	cStop := (*C.char)(unsafe.Pointer(unsafe.StringData(stop)))
	r := C.rune_find_partial_stop(cText, C.int(len(text)), cStop, C.int(len(stop)))
	return int(r)
}
