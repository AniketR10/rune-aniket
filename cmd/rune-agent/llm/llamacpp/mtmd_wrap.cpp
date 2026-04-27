#include "mtmd_wrap.h"

#include "mtmd.h"
#include "mtmd-helper.h"

#include <cstring>
#include <string>
#include <vector>

struct rune_mtmd_context {
    mtmd_context *ctx = nullptr;
};

extern "C" rune_mtmd_context *rune_mtmd_init(
    const char *mmproj_path,
    const struct llama_model *model,
    int use_gpu,
    int n_threads,
    int flash_attn_enabled,
    int warmup,
    int image_min_tokens,
    int image_max_tokens) {
    if (mmproj_path == nullptr || model == nullptr) {
        return nullptr;
    }
    mtmd_context_params params = mtmd_context_params_default();
    params.use_gpu = use_gpu != 0;
    params.print_timings = false;
    if (n_threads > 0) {
        params.n_threads = n_threads;
    }
    params.flash_attn_type = flash_attn_enabled != 0 ? LLAMA_FLASH_ATTN_TYPE_ENABLED : LLAMA_FLASH_ATTN_TYPE_AUTO;
    params.warmup = warmup != 0;
    params.image_min_tokens = image_min_tokens;
    params.image_max_tokens = image_max_tokens;

    mtmd_context *ctx = mtmd_init_from_file(mmproj_path, model, params);
    if (ctx == nullptr) {
        return nullptr;
    }
    rune_mtmd_context *out = new rune_mtmd_context();
    out->ctx = ctx;
    return out;
}

extern "C" int rune_mtmd_support_vision(const rune_mtmd_context *ctx) {
    if (ctx == nullptr || ctx->ctx == nullptr) {
        return 0;
    }
    return mtmd_support_vision(ctx->ctx) ? 1 : 0;
}

static std::vector<const mtmd_bitmap *> make_bitmaps(
    rune_mtmd_context *ctx,
    const rune_mtmd_file *files,
    size_t n_files,
    std::vector<mtmd_bitmap *> &owned,
    std::string &err) {
    std::vector<const mtmd_bitmap *> out;
    out.reserve(n_files);
    for (size_t i = 0; i < n_files; ++i) {
        mtmd_bitmap *bmp = mtmd_helper_bitmap_init_from_buf(ctx->ctx, files[i].data, files[i].len);
        if (bmp == nullptr) {
            err = "failed to decode multimodal input";
            for (mtmd_bitmap *b : owned) {
                mtmd_bitmap_free(b);
            }
            owned.clear();
            return {};
        }
        owned.push_back(bmp);
        out.push_back(bmp);
    }
    return out;
}

extern "C" int rune_mtmd_count_prompt(
    rune_mtmd_context *ctx,
    const char *prompt,
    const rune_mtmd_file *files,
    size_t n_files,
    int *n_tokens,
    int *n_pos,
    char **err_out) {
    if (ctx == nullptr || ctx->ctx == nullptr || prompt == nullptr) {
        if (err_out != nullptr) {
            *err_out = strdup("rune_mtmd_count_prompt: invalid args");
        }
        return -1;
    }
    try {
        std::vector<mtmd_bitmap *> owned;
        std::string err;
        auto bitmaps = make_bitmaps(ctx, files, n_files, owned, err);
        if (!err.empty()) {
            if (err_out != nullptr) {
                *err_out = strdup(err.c_str());
            }
            return -1;
        }

        mtmd_input_text inp_txt = {
            prompt,
            true,
            true,
        };
        mtmd_input_chunks *chunks = mtmd_input_chunks_init();
        int32_t tokenized = mtmd_tokenize(ctx->ctx, chunks, &inp_txt, bitmaps.data(), bitmaps.size());
        for (mtmd_bitmap *b : owned) {
            mtmd_bitmap_free(b);
        }
        if (tokenized != 0) {
            mtmd_input_chunks_free(chunks);
            if (err_out != nullptr) {
                *err_out = strdup("failed to tokenize multimodal prompt");
            }
            return -1;
        }
        if (n_tokens != nullptr) {
            *n_tokens = (int) mtmd_helper_get_n_tokens(chunks);
        }
        if (n_pos != nullptr) {
            *n_pos = (int) mtmd_helper_get_n_pos(chunks);
        }
        mtmd_input_chunks_free(chunks);
        return 0;
    } catch (const std::exception &e) {
        if (err_out != nullptr) {
            *err_out = strdup(e.what());
        }
        return -1;
    }
}

extern "C" int rune_mtmd_eval_prompt(
    rune_mtmd_context *ctx,
    struct llama_context *lctx,
    const char *prompt,
    const rune_mtmd_file *files,
    size_t n_files,
    int n_batch,
    int seq_id,
    int n_past,
    int logits_last,
    int *new_n_past,
    int *n_tokens,
    int *n_pos,
    char **err_out) {
    if (ctx == nullptr || ctx->ctx == nullptr || lctx == nullptr || prompt == nullptr) {
        if (err_out != nullptr) {
            *err_out = strdup("rune_mtmd_eval_prompt: invalid args");
        }
        return -1;
    }
    try {
        std::vector<mtmd_bitmap *> owned;
        std::string err;
        auto bitmaps = make_bitmaps(ctx, files, n_files, owned, err);
        if (!err.empty()) {
            if (err_out != nullptr) {
                *err_out = strdup(err.c_str());
            }
            return -1;
        }

        mtmd_input_text inp_txt = {
            prompt,
            true,
            true,
        };
        mtmd_input_chunks *chunks = mtmd_input_chunks_init();
        int32_t tokenized = mtmd_tokenize(ctx->ctx, chunks, &inp_txt, bitmaps.data(), bitmaps.size());
        for (mtmd_bitmap *b : owned) {
            mtmd_bitmap_free(b);
        }
        if (tokenized != 0) {
            mtmd_input_chunks_free(chunks);
            if (err_out != nullptr) {
                *err_out = strdup("failed to tokenize multimodal prompt");
            }
            return -1;
        }
        llama_pos np = (llama_pos) n_past;
        int32_t rc = mtmd_helper_eval_chunks(
            ctx->ctx,
            lctx,
            chunks,
            np,
            (llama_seq_id) seq_id,
            n_batch,
            logits_last != 0,
            &np);
        if (n_tokens != nullptr) {
            *n_tokens = (int) mtmd_helper_get_n_tokens(chunks);
        }
        if (n_pos != nullptr) {
            *n_pos = (int) mtmd_helper_get_n_pos(chunks);
        }
        if (new_n_past != nullptr) {
            *new_n_past = (int) np;
        }
        mtmd_input_chunks_free(chunks);
        return rc;
    } catch (const std::exception &e) {
        if (err_out != nullptr) {
            *err_out = strdup(e.what());
        }
        return -1;
    }
}

extern "C" void rune_mtmd_free(rune_mtmd_context *ctx) {
    if (ctx == nullptr) {
        return;
    }
    if (ctx->ctx != nullptr) {
        mtmd_free(ctx->ctx);
        ctx->ctx = nullptr;
    }
    delete ctx;
}
