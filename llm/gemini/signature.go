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
