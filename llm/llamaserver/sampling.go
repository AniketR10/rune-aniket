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

package llamaserver

// SamplerParams configures llama-server's sampler chain. Fields left at their
// zero value inherit llama-server's own upstream defaults; the Has* flags
// distinguish "unset" from a deliberate zero for knobs whose disabling value
// is not zero.
type SamplerParams struct {
	// Seed for deterministic sampling. 0 = random.
	Seed uint32
	// Temperature. 0 disables (falls back to greedy).
	Temperature float32
	// TopK. 0 disables.
	TopK int
	// TopP. 0 or >=1 disables.
	TopP float32
	// MinP. 0 disables.
	MinP float32
	// RepeatPenalty. 1.0 disables.
	RepeatPenalty float32
	// RepeatLastN is how many recent tokens to consider for repeat penalty.
	RepeatLastN int

	// FreqPenalty (penalty_freq) — 0 disables.
	FreqPenalty float32
	// PresencePenalty (penalty_present) — 0 disables.
	PresencePenalty float32
	// TypicalP (typ_p) — 1 disables. Use HasTypicalP to override the upstream default.
	TypicalP    float32
	HasTypicalP bool
	// TopNSigma — -1 disables. Use HasTopNSigma to override.
	TopNSigma    float32
	HasTopNSigma bool
	// Mirostat — 0 disables, 1 = Mirostat v1, 2 = v2.
	Mirostat int32
	// MirostatTau / MirostatEta. Use HasMirostatTau / HasMirostatEta to override defaults.
	MirostatTau    float32
	HasMirostatTau bool
	MirostatEta    float32
	HasMirostatEta bool
	// DynaTempRange — 0 disables. DynaTempExponent only takes effect when range > 0.
	DynaTempRange    float32
	DynaTempExponent float32
	// XtcProbability — 0 disables. XtcThreshold > 0.5 disables XTC.
	XtcProbability float32
	XtcThreshold   float32
	// DryMultiplier — 0 disables. DryBase / DryAllowedLength / DryPenaltyLastN
	// require their Has* flag to override the upstream defaults.
	DryMultiplier       float32
	DryBase             float32
	HasDryBase          bool
	DryAllowedLength    int32
	HasDryAllowedLength bool
	DryPenaltyLastN     int32
	HasDryPenaltyLastN  bool
}

// DefaultSamplerParams returns the llama.cpp sampler defaults.
func DefaultSamplerParams() SamplerParams {
	return SamplerParams{
		Seed:          0xFFFFFFFF, // LLAMA_DEFAULT_SEED
		Temperature:   0.8,
		TopK:          40,
		TopP:          0.95,
		MinP:          0.05,
		RepeatPenalty: 1.0,
		RepeatLastN:   64,
	}
}
