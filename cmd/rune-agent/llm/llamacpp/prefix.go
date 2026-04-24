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


package llamacpp

// commonPrefixLen returns the length of the longest common prefix of a and b.
func commonPrefixLen(a, b []int32) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// resolvePrefixReuse decides how many tokens of the KV cache (populated by a
// previous turn whose prompt was `cached`) can be reused for the current
// turn's `prompt`.
//
// The returned numPast is the number of leading tokens that already live in
// the KV cache and therefore need not be re-evaluated. It is clamped to
// len(prompt)-1 so the generation step always has at least one token's worth
// of logits to sample from.
func resolvePrefixReuse(cached, prompt []int32) int {
	numPast := commonPrefixLen(cached, prompt)
	if numPast >= len(prompt) {
		// Identical prompt → leave one token for the sampler.
		numPast = len(prompt) - 1
	}
	if numPast < 0 {
		numPast = 0
	}
	return numPast
}

// cacheReuseShift records a single shift produced by planCacheReuse.
// All positions are token indices.
type cacheReuseShift struct {
	srcStart int // headC: where the matched chunk currently lives in the cache
	dstStart int // headP: where it should live to align with the new prompt
	count    int // n_match: how many tokens to shift
}

// planCacheReuse mirrors the chunk-finding loop in
// server-context.cpp:2342-2410 without touching the KV cache. Returns
// the shifts to apply (in order) and the resulting nPast.
//
// Side effect: mutates `cached` in place to mirror each shift, so the
// returned slice represents the cache state after the shifts have been
// applied. Callers that want to keep the original sequence around must
// copy first.
func planCacheReuse(
	cached, prompt []int32,
	nPast, minMatch int,
) ([]cacheReuseShift, int) {
	var shifts []cacheReuseShift
	if minMatch <= 0 {
		return nil, nPast
	}
	headC := nPast
	headP := nPast
	for headC < len(cached) && headP < len(prompt) {
		nMatch := 0
		for headC+nMatch < len(cached) &&
			headP+nMatch < len(prompt) &&
			cached[headC+nMatch] == prompt[headP+nMatch] {
			nMatch++
		}
		if nMatch >= minMatch {
			shifts = append(shifts, cacheReuseShift{
				srcStart: headC,
				dstStart: headP,
				count:    nMatch,
			})
			for i := 0; i < nMatch; i++ {
				cached[headP+i] = cached[headC+i]
			}
			nPast += nMatch
			headC += nMatch
			headP += nMatch
		} else {
			headC++
		}
	}
	return shifts, nPast
}

// applyCacheReuse mirrors server-context.cpp:2342-2410. After the LCP
// reuse picks up nPast tokens, this scan looks for further chunks of at
// least minMatch tokens that appear in `cached` and `prompt` past the
// divergence point and shifts their cached KV positions so the existing
// entries align with `prompt`. Returns the new nPast — the caller still
// removes everything past it before continuing.
//
// `cached` is the token slice that mirrors the KV state up to its end;
// the caller must keep its own slice in lockstep with the shifts (the
// model side is mutated by ShiftKVRange directly).
func applyCacheReuse(
	ctx *Context,
	seqID int,
	cached, prompt []int32,
	nPast, minMatch int,
) int {
	shifts, newNPast := planCacheReuse(cached, prompt, nPast, minMatch)
	for _, s := range shifts {
		kvShift := s.dstStart - s.srcStart
		ctx.RemoveKVRange(seqID, s.dstStart, s.srcStart)
		ctx.ShiftKVRange(seqID, s.srcStart, s.srcStart+s.count, kvShift)
	}
	return newNPast
}
