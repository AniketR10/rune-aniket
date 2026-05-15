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
