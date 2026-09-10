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
