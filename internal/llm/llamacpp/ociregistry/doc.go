// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
