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


#include "common_chat_wrap.h"

#include <cstdlib>
#include <cstring>
#include <deque>
#include <exception>
#include <memory>
#include <string>
#include <string_view>
#include <algorithm>
#include <vector>

#include "chat.h"  // from llama.cpp/common/
#include "common.h"
#include "sampling.h"
#include "server-common.h"  // gen_tool_call_id (upstream llama-server helper)
#include "server-task.h"    // task_result_state::update_chat_msg

// rune_find_partial_stop wraps common.h's string_find_partial_stop so the
// Go side can ask "does `text` end with a prefix of `stop`?" using the
// exact upstream primitive — same UTF-8 boundary handling, same npos
// sentinel.
extern "C" int rune_find_partial_stop(
    const char *text,
    int         text_len,
    const char *stop,
    int         stop_len) {
    if (text == nullptr || stop == nullptr || text_len <= 0 || stop_len <= 0) {
        return -1;
    }
    std::string_view t(text, static_cast<size_t>(text_len));
    std::string_view s(stop, static_cast<size_t>(stop_len));
    size_t pos = string_find_partial_stop(t, s);
    if (pos == std::string::npos) {
        return -1;
    }
    return static_cast<int>(pos);
}

namespace {

char *dup_c_str(const std::string &s) {
    char *out = static_cast<char *>(std::malloc(s.size() + 1));
    if (out == nullptr) {
        return nullptr;
    }
    std::memcpy(out, s.data(), s.size());
    out[s.size()] = '\0';
    return out;
}

common_chat_msg to_common_chat_msg(const struct rune_chat_message &msg) {
    common_chat_msg out;
    out.role              = msg.role != nullptr ? msg.role : "";
    out.content           = msg.content != nullptr ? msg.content : "";
    out.reasoning_content = msg.reasoning_content != nullptr ? msg.reasoning_content : "";
    out.tool_name         = msg.tool_name != nullptr ? msg.tool_name : "";
    out.tool_call_id      = msg.tool_call_id != nullptr ? msg.tool_call_id : "";

    out.content_parts.reserve(msg.n_content_parts);
    for (size_t i = 0; i < msg.n_content_parts; ++i) {
        common_chat_msg_content_part part;
        part.type = msg.content_parts[i].kind != nullptr ? msg.content_parts[i].kind : "";
        part.text = msg.content_parts[i].text != nullptr ? msg.content_parts[i].text : "";
        out.content_parts.push_back(std::move(part));
    }

    out.tool_calls.reserve(msg.n_tool_calls);
    for (size_t i = 0; i < msg.n_tool_calls; ++i) {
        common_chat_tool_call tc;
        tc.name      = msg.tool_calls[i].name != nullptr ? msg.tool_calls[i].name : "";
        tc.arguments = msg.tool_calls[i].arguments != nullptr ? msg.tool_calls[i].arguments : "";
        tc.id        = msg.tool_calls[i].id != nullptr ? msg.tool_calls[i].id : "";
        out.tool_calls.push_back(std::move(tc));
    }

    return out;
}

}  // namespace

// ---------------------------------------------------------------------------
// Template handle
// ---------------------------------------------------------------------------

struct rune_chat_template {
    // templates owns the compiled Jinja + caps; must outlive any stream
    // parser derived from it because the PEG arena inside params.parser
    // may reference interned strings held here.
    common_chat_templates_ptr templates;
    common_chat_params        params;
    common_reasoning_format   reasoning_format = COMMON_REASONING_FORMAT_NONE;
};

namespace {

common_reasoning_format map_reasoning(int v) {
    switch (v) {
        case RUNE_REASONING_DEEPSEEK_LEGACY:
            return COMMON_REASONING_FORMAT_DEEPSEEK_LEGACY;
        case RUNE_REASONING_DEEPSEEK:
            return COMMON_REASONING_FORMAT_DEEPSEEK;
        case RUNE_REASONING_AUTO:
            return COMMON_REASONING_FORMAT_AUTO;
        default:
            return COMMON_REASONING_FORMAT_NONE;
    }
}

common_chat_tool_choice map_tool_choice(int v) {
    switch (v) {
        case 1:
            return COMMON_CHAT_TOOL_CHOICE_REQUIRED;
        case 2:
            return COMMON_CHAT_TOOL_CHOICE_NONE;
        default:
            return COMMON_CHAT_TOOL_CHOICE_AUTO;
    }
}

void copy_sampler_params(common_params_sampling &out, const struct rune_sampler_params *in) {
    if (in == nullptr) {
        return;
    }
    out.seed           = in->seed;
    out.temp           = in->temperature;
    out.top_k          = in->top_k;
    out.top_p          = in->top_p;
    out.min_p          = in->min_p;
    out.penalty_repeat = in->repeat_penalty;
    out.penalty_last_n = in->repeat_last_n;
    // Direct numeric fields — zero is a valid disabled value upstream
    // for all of these, so we pass them through unconditionally.
    out.penalty_freq      = in->freq_penalty;
    out.penalty_present   = in->presence_penalty;
    out.mirostat          = in->mirostat;
    out.dynatemp_range    = in->dynatemp_range;
    out.xtc_probability   = in->xtc_probability;
    out.xtc_threshold     = in->xtc_threshold;
    out.dry_multiplier    = in->dry_multiplier;
    // Fields where upstream's default is non-zero (typical_p=1.0,
    // top_n_sigma=-1.0, dry_base=1.75 etc.). Only override when the
    // caller explicitly provided a value.
    if (in->has_typical_p)         out.typ_p              = in->typical_p;
    if (in->has_top_n_sigma)       out.top_n_sigma        = in->top_n_sigma;
    if (in->has_mirostat_tau)      out.mirostat_tau       = in->mirostat_tau;
    if (in->has_mirostat_eta)      out.mirostat_eta       = in->mirostat_eta;
    if (in->has_dry_base)          out.dry_base           = in->dry_base;
    if (in->has_dry_allowed_length) out.dry_allowed_length = in->dry_allowed_length;
    if (in->has_dry_penalty_last_n) out.dry_penalty_last_n = in->dry_penalty_last_n;
    // dynatemp_exponent's upstream default is 1.0; only override when
    // dynatemp is actually engaged (range > 0) so legacy zero callers
    // don't accidentally clamp the exponent to 0.
    if (in->dynatemp_range > 0.0f && in->dynatemp_exponent > 0.0f) {
        out.dynatemp_exponent = in->dynatemp_exponent;
    }
}

void copy_chat_sampler_params(
    common_params_sampling &out,
    const struct llama_model *model,
    const rune_chat_template *tmpl) {
    if (tmpl == nullptr) {
        return;
    }

    const auto &chat = tmpl->params;
    if (!chat.grammar.empty()) {
        out.grammar = {COMMON_GRAMMAR_TYPE_TOOL_CALLS, chat.grammar};
    }
    out.grammar_lazy = chat.grammar_lazy;
    out.generation_prompt = chat.generation_prompt;

    const llama_vocab *vocab = model != nullptr ? llama_model_get_vocab(model) : nullptr;
    if (vocab == nullptr) {
        out.grammar_triggers = chat.grammar_triggers;
        return;
    }

    for (const auto &text : chat.preserved_tokens) {
        auto ids = common_tokenize(vocab, text, /* add_special= */ false, /* parse_special= */ true);
        if (ids.size() == 1) {
            out.preserved_tokens.insert(ids[0]);
        }
    }

    // Mirror server-task.cpp:447-477: word triggers whose tokenisation is a
    // single token AND already in preserved_tokens become TOKEN triggers.
    // Words that tokenise to a single token but are NOT preserved are
    // rejected upstream — we keep them as WORD here to avoid breaking
    // permissive callers, but match the upstream collapse to TOKEN.
    for (const auto &trigger_in : chat.grammar_triggers) {
        if (trigger_in.type != COMMON_GRAMMAR_TRIGGER_TYPE_WORD) {
            out.grammar_triggers.push_back(trigger_in);
            continue;
        }
        const auto &word = trigger_in.value;
        auto ids = common_tokenize(vocab, word, /* add_special= */ false, /* parse_special= */ true);
        if (ids.size() == 1) {
            llama_token token = ids[0];
            if (std::find(out.preserved_tokens.begin(), out.preserved_tokens.end(), token)
                == out.preserved_tokens.end()) {
                throw std::runtime_error(
                    "Grammar trigger word should be marked as preserved token: " + word);
            }
            common_grammar_trigger trig;
            trig.type  = COMMON_GRAMMAR_TRIGGER_TYPE_TOKEN;
            trig.value = word;
            trig.token = token;
            out.grammar_triggers.push_back(std::move(trig));
        } else {
            out.grammar_triggers.push_back({COMMON_GRAMMAR_TRIGGER_TYPE_WORD, word});
        }
    }

    if (out.grammar_lazy && out.grammar_triggers.empty()) {
        throw std::runtime_error("lazy grammar has no triggers");
    }
}

}  // namespace

extern "C" rune_chat_template *rune_chat_template_open(
    const struct llama_model       *model,
    const char                     *tmpl_override,
    const struct rune_chat_message *msgs,
    size_t                          n_msgs,
    const struct rune_tool         *tools,
    size_t                          n_tools,
    const struct rune_response_format *response_format,
    int                             tool_choice,
    int                             parallel_tool_calls,
    int                             reasoning_format,
    int                             enable_thinking,
    int                             add_generation_prompt,
    char                          **err_out) {
    try {
        std::string tmpl_src = tmpl_override ? std::string(tmpl_override) : std::string();
        auto        templates = common_chat_templates_init(model, tmpl_src);
        if (!templates) {
            if (err_out != nullptr) {
                *err_out = dup_c_str("common_chat_templates_init returned null");
            }
            return nullptr;
        }

        common_chat_templates_inputs inputs;
        inputs.use_jinja             = true;
        inputs.add_generation_prompt = add_generation_prompt != 0;
        inputs.tool_choice           = map_tool_choice(tool_choice);
        inputs.parallel_tool_calls   = parallel_tool_calls != 0;
        inputs.enable_thinking       = enable_thinking != 0;
        inputs.reasoning_format      = map_reasoning(reasoning_format);
        if (response_format != nullptr && response_format->json_schema != nullptr) {
            inputs.json_schema = response_format->json_schema;
        }
        inputs.messages.reserve(n_msgs);
        for (size_t i = 0; i < n_msgs; ++i) {
            inputs.messages.push_back(to_common_chat_msg(msgs[i]));
        }
        if (tools != nullptr && n_tools > 0) {
            inputs.tools.reserve(n_tools);
            for (size_t i = 0; i < n_tools; ++i) {
                common_chat_tool t;
                t.name        = tools[i].name != nullptr ? tools[i].name : "";
                t.description = tools[i].description != nullptr ? tools[i].description : "";
                t.parameters  = tools[i].parameters_json != nullptr ? tools[i].parameters_json : "";
                inputs.tools.push_back(std::move(t));
            }
        }

        auto *h          = new rune_chat_template();
        h->templates     = std::move(templates);
        h->params        = common_chat_templates_apply(h->templates.get(), inputs);
        h->reasoning_format = inputs.reasoning_format;
        return h;
    } catch (const std::exception &e) {
        if (err_out != nullptr) {
            *err_out = dup_c_str(e.what());
        }
        return nullptr;
    } catch (...) {
        if (err_out != nullptr) {
            *err_out = dup_c_str("rune_chat_template_open: unknown C++ exception");
        }
        return nullptr;
    }
}

extern "C" int rune_chat_template_prompt(const rune_chat_template *tmpl, char *buf, int buflen) {
    if (tmpl == nullptr) {
        return -1;
    }
    const std::string &out = tmpl->params.prompt;
    if (buflen > 0 && buf != nullptr) {
        size_t copy = out.size() < static_cast<size_t>(buflen) ? out.size()
                                                               : static_cast<size_t>(buflen);
        std::memcpy(buf, out.data(), copy);
        if (copy < static_cast<size_t>(buflen)) {
            buf[copy] = '\0';
        }
    }
    return static_cast<int>(out.size());
}

extern "C" size_t rune_chat_template_additional_stop_count(const rune_chat_template *tmpl) {
    if (tmpl == nullptr) {
        return 0;
    }
    return tmpl->params.additional_stops.size();
}

extern "C" int rune_chat_template_additional_stop(
    const rune_chat_template *tmpl,
    size_t index,
    char *buf,
    int buflen) {
    if (tmpl == nullptr || index >= tmpl->params.additional_stops.size()) {
        return -1;
    }
    const std::string &out = tmpl->params.additional_stops[index];
    if (buflen > 0 && buf != nullptr) {
        size_t copy = out.size() < static_cast<size_t>(buflen) ? out.size()
                                                               : static_cast<size_t>(buflen);
        std::memcpy(buf, out.data(), copy);
        if (copy < static_cast<size_t>(buflen)) {
            buf[copy] = '\0';
        }
    }
    return static_cast<int>(out.size());
}

extern "C" int rune_chat_template_has_parser(const rune_chat_template *tmpl) {
    if (tmpl == nullptr) {
        return 0;
    }
    return tmpl->params.parser.empty() ? 0 : 1;
}

extern "C" void rune_chat_template_free(rune_chat_template *tmpl) {
    delete tmpl;
}

// ---------------------------------------------------------------------------
// Sampler
// ---------------------------------------------------------------------------

struct rune_sampler {
    common_sampler *sampler = nullptr;
};

extern "C" rune_sampler *rune_sampler_new(
    const struct llama_model         *model,
    const struct rune_sampler_params *params,
    const rune_chat_template         *tmpl,
    char                            **err_out) {
    if (model == nullptr) {
        if (err_out != nullptr) {
            *err_out = dup_c_str("rune_sampler_new: nil model");
        }
        return nullptr;
    }
    try {
        common_params_sampling sp;
        copy_sampler_params(sp, params);
        copy_chat_sampler_params(sp, model, tmpl);

        auto *out = new rune_sampler();
        out->sampler = common_sampler_init(model, sp);
        if (out->sampler == nullptr) {
            delete out;
            if (err_out != nullptr) {
                *err_out = dup_c_str("common_sampler_init returned null");
            }
            return nullptr;
        }
        return out;
    } catch (const std::exception &e) {
        if (err_out != nullptr) {
            *err_out = dup_c_str(e.what());
        }
        return nullptr;
    } catch (...) {
        if (err_out != nullptr) {
            *err_out = dup_c_str("rune_sampler_new: unknown C++ exception");
        }
        return nullptr;
    }
}

extern "C" void rune_sampler_free(rune_sampler *s) {
    if (s == nullptr) {
        return;
    }
    common_sampler_free(s->sampler);
    delete s;
}

extern "C" int32_t rune_sampler_sample(
    rune_sampler *s,
    struct llama_context *ctx,
    int32_t idx,
    int grammar_first) {
    if (s == nullptr || s->sampler == nullptr || ctx == nullptr) {
        return LLAMA_TOKEN_NULL;
    }
    return common_sampler_sample(s->sampler, ctx, idx, grammar_first != 0);
}

extern "C" void rune_sampler_accept(rune_sampler *s, int32_t token, int accept_grammar) {
    if (s == nullptr || s->sampler == nullptr) {
        return;
    }
    common_sampler_accept(s->sampler, static_cast<llama_token>(token), accept_grammar != 0);
}

extern "C" void rune_sampler_reset(rune_sampler *s) {
    if (s == nullptr || s->sampler == nullptr) {
        return;
    }
    common_sampler_reset(s->sampler);
}

// ---------------------------------------------------------------------------
// Streaming parser
// ---------------------------------------------------------------------------

namespace {

// queued_event holds the stable storage for an event that has been
// computed but not yet delivered via rune_chat_stream_next. The C-side
// string pointers returned to Go must remain valid until the next feed
// call, so we keep each event's strings here.
struct queued_event {
    int         kind = RUNE_CHAT_EVENT_NONE;
    std::string text;
    std::string tool_name;
    std::string tool_arguments;
    std::string tool_id;
    int         tool_index = -1;
};

}  // namespace

// rune_chat_stream wraps llama-server's task_result_state so that the Go
// caller gets the same incremental tool-call filtering (name+id event
// followed by argument deltas) used by the OpenAI-compatible HTTP
// streaming endpoint.
struct rune_chat_stream {
    const rune_chat_template     *tmpl  = nullptr;
    std::unique_ptr<task_result_state> state;
    std::deque<queued_event>      queue;
    // current_event backs the const char* fields returned by
    // rune_chat_stream_next. It stays alive until the next call.
    queued_event                  current_event;

    rune_chat_stream(const rune_chat_template *t, common_chat_parser_params parser_params)
        : tmpl(t), state(new task_result_state(std::move(parser_params))) {}
};

extern "C" rune_chat_stream *rune_chat_stream_new(const rune_chat_template *tmpl) {
    if (tmpl == nullptr) {
        return nullptr;
    }
    // Build common_chat_parser_params once and hand it to task_result_state.
    // Upstream rebuilds these per request from the resolved chat_params; we
    // do the same once per stream lifetime — the PEG arena load is cheap
    // compared to per-feed reparses.
    common_chat_parser_params parser_params(tmpl->params);
    parser_params.reasoning_format = tmpl->reasoning_format;
    if (!tmpl->params.parser.empty()) {
        parser_params.parser.load(tmpl->params.parser);
    }
    return new rune_chat_stream(tmpl, std::move(parser_params));
}

extern "C" void rune_chat_stream_free(rune_chat_stream *s) {
    delete s;
}

extern "C" int rune_chat_stream_feed(
    rune_chat_stream *s,
    const char       *text,
    int               text_len,
    int               is_partial,
    char            **err_out) {
    if (s == nullptr) {
        if (err_out != nullptr) {
            *err_out = dup_c_str("rune_chat_stream_feed: nil stream");
        }
        return -1;
    }
    try {
        std::string chunk;
        if (text != nullptr && text_len > 0) {
            chunk.assign(text, static_cast<size_t>(text_len));
        }

        std::vector<common_chat_msg_diff> diffs;
        try {
            // filter_tool_calls=true mirrors the OpenAI-compatible HTTP
            // streaming path: emit a single header diff with name+id when
            // a new tool call appears, then argument-only diffs.
            s->state->update_chat_msg(chunk, is_partial != 0, diffs, /*filter_tool_calls=*/true);
        } catch (const std::exception &) {
            // common_chat_parse throws on partial inputs that don't yet
            // match the grammar (e.g. mid-reasoning). update_chat_msg
            // doesn't catch these — treat as "no new deltas this round".
            // The accumulated text is already inside task_result_state.
            return 0;
        }

        for (auto &d : diffs) {
            if (!d.reasoning_content_delta.empty()) {
                queued_event ev;
                ev.kind = RUNE_CHAT_EVENT_REASONING_DELTA;
                ev.text = std::move(d.reasoning_content_delta);
                s->queue.push_back(std::move(ev));
            }
            if (!d.content_delta.empty()) {
                queued_event ev;
                ev.kind = RUNE_CHAT_EVENT_TEXT_DELTA;
                ev.text = std::move(d.content_delta);
                s->queue.push_back(std::move(ev));
            }
            if (d.tool_call_index != std::string::npos) {
                queued_event ev;
                ev.kind           = RUNE_CHAT_EVENT_TOOL_CALL_DELTA;
                ev.tool_index     = static_cast<int>(d.tool_call_index);
                ev.tool_name      = std::move(d.tool_call_delta.name);
                ev.tool_arguments = std::move(d.tool_call_delta.arguments);
                ev.tool_id        = std::move(d.tool_call_delta.id);
                s->queue.push_back(std::move(ev));
            }
        }
        return 0;
    } catch (const std::exception &e) {
        if (err_out != nullptr) {
            *err_out = dup_c_str(e.what());
        }
        return -1;
    } catch (...) {
        if (err_out != nullptr) {
            *err_out = dup_c_str("rune_chat_stream_feed: unknown C++ exception");
        }
        return -1;
    }
}

extern "C" int rune_chat_stream_next(rune_chat_stream *s, struct rune_chat_stream_event *out) {
    if (s == nullptr || out == nullptr) {
        return 1;
    }
    if (s->queue.empty()) {
        out->kind           = RUNE_CHAT_EVENT_NONE;
        out->text           = nullptr;
        out->text_len       = 0;
        out->tool_name      = nullptr;
        out->tool_arguments = nullptr;
        out->tool_id        = nullptr;
        out->tool_index     = -1;
        return 1;
    }
    s->current_event = std::move(s->queue.front());
    s->queue.pop_front();

    out->kind           = s->current_event.kind;
    out->text           = s->current_event.text.c_str();
    out->text_len       = static_cast<int>(s->current_event.text.size());
    out->tool_name      = s->current_event.tool_name.empty() ? nullptr : s->current_event.tool_name.c_str();
    out->tool_arguments = s->current_event.tool_arguments.empty() ? nullptr : s->current_event.tool_arguments.c_str();
    out->tool_id        = s->current_event.tool_id.empty() ? nullptr : s->current_event.tool_id.c_str();
    out->tool_index     = s->current_event.tool_index;
    return 0;
}
