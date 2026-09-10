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

// Package llamacpp maintains the on-disk catalog of locally-cached GGUF
// models. It downloads models from OCI/HuggingFace registries (see the
// ociregistry subpackage), tracks them under a cache directory, and exposes
// them as llmapi.ModelEntry values keyed by the LLMProvider identifier.
//
// Inference is no longer performed in-process: the router hands these entries
// to the llamaserver backend, which runs the OpenAI-compatible `llama-server`
// binary as a managed subprocess. This package is therefore
// inference-agnostic and free of any CGo/llama.cpp linkage.
package llamacpp
