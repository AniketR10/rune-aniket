// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this
// information or reproduction of this material is strictly forbidden unless
// prior written permission is obtained from COMPANY.

// Thin C wrapper around llama.cpp's common_chat_templates_* Jinja path so
// cgo can call it without crossing a C++ ABI boundary. Exists because the
// legacy llama_chat_apply_template() only supports a fixed set of known
// template shapes via substring detection; modern models (Gemma 4 etc.)
// ship Jinja templates that require the common/jinja engine to render.

#pragma once

#include <stddef.h>
#include <stdint.h>

#include "llama.h"

#ifdef __cplusplus
extern "C" {
#endif

// rune_find_partial_stop locates the offset within `text` at which a
// prefix of `stop` begins, or -1 if no prefix of `stop` ends `text`.
// Wraps llama.cpp/common/common.h:string_find_partial_stop.
int rune_find_partial_stop(
    const char *text,
    int         text_len,
    const char *stop,
    int         stop_len);

struct rune_chat_content_part {
    const char *kind;
    const char *text;
};

struct rune_chat_tool_call {
    const char *name;
    const char *arguments;
    const char *id;
};

struct rune_chat_message {
    const char *role;
    const char *content;

    const struct rune_chat_content_part *content_parts;
    size_t                               n_content_parts;

    const struct rune_chat_tool_call    *tool_calls;
    size_t                               n_tool_calls;

    const char *reasoning_content;
    const char *tool_name;
    const char *tool_call_id;
};

// Opaque handle returned by rune_chat_template_open. Owns the rendered
// prompt, the chat format, the PEG parser arena and any other per-request
// state that streaming incremental parsers need to share with the
// template that produced the prompt.
typedef struct rune_chat_template rune_chat_template;

// rune_tool is a C view into a single tool definition. JSON is
// caller-owned and must stay alive until rune_chat_template_open returns.
struct rune_tool {
    const char *name;
    const char *description;
    const char *parameters_json;  // JSON schema serialised as a string
};

struct rune_response_format {
    const char *json_schema; // empty when no structured output is requested
};

// Reasoning format hint for stream parsing. Matches upstream's
// common_reasoning_format (see common/common.h). Use
// RUNE_REASONING_AUTO to let llama.cpp pick a reasonable default based
// on the chat format — this is what llama-server uses.
enum {
    RUNE_REASONING_NONE            = 0,
    RUNE_REASONING_AUTO            = 1,
    RUNE_REASONING_DEEPSEEK_LEGACY = 2,
    RUNE_REASONING_DEEPSEEK        = 3,
};

// rune_chat_template_open renders msgs[] into a prompt using the model's
// Jinja template (or tmpl_override) plus tools[] and returns a handle.
// The handle is later passed to rune_chat_template_prompt and
// rune_chat_stream_new. Free with rune_chat_template_free even on error.
//
// On failure returns NULL and, if err_out is non-NULL, sets *err_out to
// a heap-allocated error message the caller must free().
rune_chat_template *rune_chat_template_open(
    const struct llama_model       *model,
    const char                     *tmpl_override,
    const struct rune_chat_message *msgs,
    size_t                          n_msgs,
    const struct rune_tool         *tools,
    size_t                          n_tools,
    const struct rune_response_format *response_format,
    int                             tool_choice,  // 0=auto,1=required,2=none
    int                             parallel_tool_calls,
    int                             reasoning_format,  // RUNE_REASONING_*
    int                             enable_thinking,
    int                             add_generation_prompt,
    char                          **err_out);

// rune_chat_template_prompt copies the rendered prompt into buf. Returns
// the full prompt length (may exceed buflen — caller retries with a
// larger buffer in that case). Returns -1 if tmpl is NULL.
int rune_chat_template_prompt(const rune_chat_template *tmpl, char *buf, int buflen);

size_t rune_chat_template_additional_stop_count(const rune_chat_template *tmpl);

int rune_chat_template_additional_stop(
    const rune_chat_template *tmpl,
    size_t                    index,
    char                     *buf,
    int                       buflen);

// rune_chat_template_has_parser returns 1 when the template resolved to
// a common_chat_params with a non-empty PEG arena (Gemma 4, DeepSeek,
// etc.), and 0 when it's a plain content-only format. Callers use this
// to decide whether to route streaming output through
// common_chat_parse or fall back to their own Hermes-style parsing.
int rune_chat_template_has_parser(const rune_chat_template *tmpl);

void rune_chat_template_free(rune_chat_template *tmpl);

// Opaque sampler wrapper backed by llama.cpp common_sampler. Unlike a raw
// llama_sampler_chain, this understands the grammar/tool-call controls that
// common_chat_templates_apply stores in rune_chat_template->params.
typedef struct rune_sampler rune_sampler;

struct rune_sampler_params {
    uint32_t seed;
    float    temperature;
    int32_t  top_k;
    float    top_p;
    float    min_p;
    float    repeat_penalty;
    int32_t  repeat_last_n;
    // Additional knobs mirrored from common_params_sampling. Zero values
    // for fields explicitly opt-in (e.g. mirostat = 0 means disabled,
    // matching upstream); see copy_sampler_params for the merge rules.
    float    freq_penalty;
    float    presence_penalty;
    float    typical_p;
    float    top_n_sigma;
    int32_t  mirostat;
    float    mirostat_tau;
    float    mirostat_eta;
    float    dynatemp_range;
    float    dynatemp_exponent;
    float    xtc_probability;
    float    xtc_threshold;
    float    dry_multiplier;
    float    dry_base;
    int32_t  dry_allowed_length;
    int32_t  dry_penalty_last_n;
    // has_* flags signal whether the corresponding field was set by the
    // caller (1) or should fall back to llama.cpp's default (0). This
    // keeps the Go-side SamplerParams API additive and backward
    // compatible — old callers leaving these zero get the upstream
    // defaults.
    int32_t  has_typical_p;
    int32_t  has_top_n_sigma;
    int32_t  has_mirostat_tau;
    int32_t  has_mirostat_eta;
    int32_t  has_dry_base;
    int32_t  has_dry_allowed_length;
    int32_t  has_dry_penalty_last_n;
};

rune_sampler *rune_sampler_new(
    const struct llama_model          *model,
    const struct rune_sampler_params  *params,
    const rune_chat_template          *tmpl,
    char                             **err_out);

void rune_sampler_free(rune_sampler *s);

int32_t rune_sampler_sample(
    rune_sampler         *s,
    struct llama_context *ctx,
    int32_t               idx,
    int                   grammar_first);

void rune_sampler_accept(rune_sampler *s, int32_t token, int accept_grammar);

void rune_sampler_reset(rune_sampler *s);

// Opaque streaming parser. Owns the running accumulated text and the
// previously-parsed common_chat_msg so diffs can be computed token by
// token. Tied to the lifetime of the rune_chat_template it was opened
// from.
typedef struct rune_chat_stream rune_chat_stream;

rune_chat_stream *rune_chat_stream_new(const rune_chat_template *tmpl);

void rune_chat_stream_free(rune_chat_stream *s);

// Event kinds emitted by rune_chat_stream_next. Multiple events may be
// produced per rune_chat_stream_feed call; drain with repeated
// rune_chat_stream_next until RUNE_CHAT_EVENT_NONE.
enum {
    RUNE_CHAT_EVENT_NONE            = 0,
    RUNE_CHAT_EVENT_REASONING_DELTA = 1,
    RUNE_CHAT_EVENT_TEXT_DELTA      = 2,
    RUNE_CHAT_EVENT_TOOL_CALL_DELTA = 3,
};

// Event produced by rune_chat_stream_next. String fields are valid until
// the next rune_chat_stream_feed or rune_chat_stream_free call.
struct rune_chat_stream_event {
    int         kind;           // RUNE_CHAT_EVENT_*
    const char *text;           // TEXT_DELTA / REASONING_DELTA body
    int         text_len;
    const char *tool_name;      // TOOL_CALL_DELTA: accumulated name so far
    const char *tool_arguments; // TOOL_CALL_DELTA: accumulated args so far
    const char *tool_id;        // TOOL_CALL_DELTA: id assigned by the parser
    int         tool_index;     // TOOL_CALL_DELTA: 0-based call index
};

// rune_chat_stream_feed appends text to the running stream, reparses,
// computes diffs against the previous result, and queues them so
// rune_chat_stream_next can drain them. Returns 0 on success, -1 on
// parser failure (err_out set to a heap-allocated message).
int rune_chat_stream_feed(
    rune_chat_stream *s,
    const char       *text,
    int               text_len,
    int               is_partial,
    char            **err_out);

// rune_chat_stream_next pops the next queued event into *out. Returns 0
// when an event was written, or 1 when the queue is empty (out->kind is
// set to RUNE_CHAT_EVENT_NONE).
int rune_chat_stream_next(rune_chat_stream *s, struct rune_chat_stream_event *out);

#ifdef __cplusplus
}  // extern "C"
#endif
