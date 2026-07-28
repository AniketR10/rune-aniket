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

// Package ociregistry is a thin, opinionated wrapper around oras-go's
// registry client. It adds the bits the upstream library leaves to callers
// but that every model-puller needs:
//
//   - A content-addressed on-disk cache compatible with the Ollama layout
//     (blobs/sha256-<hex>, manifests/<host>/<repo>/<tag>).
//   - Streaming blob download with sha256 verification and resume-on-restart
//     via a .partial file.
//   - Progress reporting.
//   - A Hugging-Face-friendly reference parser that accepts uppercase repo
//     names (oras-go's validator is strict lowercase, which rejects HF
//     repos like "bartowski/Llama-3.2-1B-Instruct-GGUF").
//
// The underlying HTTP transport, Bearer-challenge auth, and retry policy
// come from oras-go.
package ociregistry
