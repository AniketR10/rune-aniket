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

package gemini

import (
	"encoding/base64"
	"encoding/json"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// thoughtSignatureField is the ToolCall.ProviderFields key under which the
// Gemini per-call thought_signature is carried across persistence and replay.
const thoughtSignatureField = "gemini.thought_signature"

// skipSignatureValidator is the documented sentinel that disables Gemini 3+
// thought_signature validation for a functionCall part. It is used as a
// fallback when a first-of-step call has no captured signature (e.g. history
// imported from another model or created before signatures were captured), so
// replay degrades to reduced reasoning quality instead of a hard 400.
// See https://ai.google.dev/gemini-api/docs/thought-signatures.
const skipSignatureValidator = "skip_thought_signature_validator"

// setThoughtSignature records the opaque per-call thought_signature on a tool
// call so it can be echoed back on the functionCall part in the next request.
func setThoughtSignature(tc *llmapi.ToolCall, sig []byte) {
	if len(sig) == 0 {
		return
	}
	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString(sig))
	if err != nil {
		return
	}
	if tc.ProviderFields == nil {
		tc.ProviderFields = map[string]json.RawMessage{}
	}
	tc.ProviderFields[thoughtSignatureField] = encoded
}

// thoughtSignature returns the opaque per-call thought_signature previously
// recorded on the tool call, or nil when none was captured.
func thoughtSignature(tc llmapi.ToolCall) []byte {
	raw, ok := tc.ProviderFields[thoughtSignatureField]
	if !ok {
		return nil
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil
	}
	sig, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	return sig
}
