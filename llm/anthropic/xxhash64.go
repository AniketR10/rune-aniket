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

package anthropic

// xxHash64 (64-bit variant of Cyan4973's xxHash) with an explicit seed.
// Implemented inline rather than via a third-party module because the Claude
// Code billing-header signature requires a specific non-zero seed, which the
// in-tree cespare/xxhash (seed 0 only) cannot provide. The constants and
// mixing steps match the reference algorithm exactly so signatures byte-match
// the official client.

const (
	xxhPrime64_1 = 11400714785074694791
	xxhPrime64_2 = 14029467366897019727
	xxhPrime64_3 = 1609587929392839161
	xxhPrime64_4 = 9650029242287828579
	xxhPrime64_5 = 2870177450012600261
)

// xxHash64Checksum returns the 64-bit xxHash of input using the given seed.
func xxHash64Checksum(input []byte, seed uint64) uint64 {
	n := len(input)
	var h64 uint64

	if n >= 32 {
		v1 := seed + xxhPrime64_1 + xxhPrime64_2
		v2 := seed + xxhPrime64_2
		v3 := seed
		v4 := seed - xxhPrime64_1
		p := 0
		for limit := n - 32; p <= limit; p += 32 {
			sub := input[p : p+32]
			v1 = xxhRol31(v1+xxhU64(sub[0:])*xxhPrime64_2) * xxhPrime64_1
			v2 = xxhRol31(v2+xxhU64(sub[8:])*xxhPrime64_2) * xxhPrime64_1
			v3 = xxhRol31(v3+xxhU64(sub[16:])*xxhPrime64_2) * xxhPrime64_1
			v4 = xxhRol31(v4+xxhU64(sub[24:])*xxhPrime64_2) * xxhPrime64_1
		}

		h64 = xxhRol(v1, 1) + xxhRol(v2, 7) + xxhRol(v3, 12) + xxhRol(v4, 18)

		v1 *= xxhPrime64_2
		v2 *= xxhPrime64_2
		v3 *= xxhPrime64_2
		v4 *= xxhPrime64_2

		h64 = (h64^(xxhRol31(v1)*xxhPrime64_1))*xxhPrime64_1 + xxhPrime64_4
		h64 = (h64^(xxhRol31(v2)*xxhPrime64_1))*xxhPrime64_1 + xxhPrime64_4
		h64 = (h64^(xxhRol31(v3)*xxhPrime64_1))*xxhPrime64_1 + xxhPrime64_4
		h64 = (h64^(xxhRol31(v4)*xxhPrime64_1))*xxhPrime64_1 + xxhPrime64_4

		h64 += uint64(n)

		input = input[p:]
	} else {
		h64 = seed + xxhPrime64_5 + uint64(n)
	}

	p := 0
	r := len(input)
	for limit := r - 8; p <= limit; p += 8 {
		h64 ^= xxhRol31(xxhU64(input[p:p+8])*xxhPrime64_2) * xxhPrime64_1
		h64 = xxhRol(h64, 27)*xxhPrime64_1 + xxhPrime64_4
	}
	if p+4 <= r {
		h64 ^= uint64(xxhU32(input[p:p+4])) * xxhPrime64_1
		h64 = xxhRol(h64, 23)*xxhPrime64_2 + xxhPrime64_3
		p += 4
	}
	for ; p < r; p++ {
		h64 ^= uint64(input[p]) * xxhPrime64_5
		h64 = xxhRol(h64, 11) * xxhPrime64_1
	}

	h64 ^= h64 >> 33
	h64 *= xxhPrime64_2
	h64 ^= h64 >> 29
	h64 *= xxhPrime64_3
	h64 ^= h64 >> 32
	return h64
}

func xxhU64(buf []byte) uint64 {
	return uint64(buf[0]) | uint64(buf[1])<<8 | uint64(buf[2])<<16 | uint64(buf[3])<<24 |
		uint64(buf[4])<<32 | uint64(buf[5])<<40 | uint64(buf[6])<<48 | uint64(buf[7])<<56
}

func xxhU32(buf []byte) uint32 {
	return uint32(buf[0]) | uint32(buf[1])<<8 | uint32(buf[2])<<16 | uint32(buf[3])<<24
}

func xxhRol(u uint64, r uint) uint64 {
	return u<<r | u>>(64-r)
}

func xxhRol31(u uint64) uint64 {
	return u<<31 | u>>33
}
