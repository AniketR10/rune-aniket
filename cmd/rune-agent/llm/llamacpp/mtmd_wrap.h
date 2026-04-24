#pragma once

#include "llama.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct rune_mtmd_context rune_mtmd_context;

typedef struct rune_mtmd_file {
    const unsigned char *data;
    size_t len;
} rune_mtmd_file;

rune_mtmd_context *rune_mtmd_init(
    const char *mmproj_path,
    const struct llama_model *model,
    int use_gpu,
    int n_threads,
    int flash_attn_enabled,
    int warmup,
    int image_min_tokens,
    int image_max_tokens);

int rune_mtmd_support_vision(const rune_mtmd_context *ctx);

int rune_mtmd_count_prompt(
    rune_mtmd_context *ctx,
    const char *prompt,
    const rune_mtmd_file *files,
    size_t n_files,
    int *n_tokens,
    int *n_pos,
    char **err_out);

int rune_mtmd_eval_prompt(
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
    char **err_out);

void rune_mtmd_free(rune_mtmd_context *ctx);

#ifdef __cplusplus
}
#endif
