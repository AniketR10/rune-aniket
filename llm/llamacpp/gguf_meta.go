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

// Minimal pure-Go parser for GGUF header metadata. We only need a handful of
// scalar keys (most importantly `general.architecture` and
// `{arch}.context_length`) to populate the model registry, and loading the
// full model via llama.cpp just to read those would (a) require CGO, (b)
// mmap the weights, and (c) fail in VocabOnly mode — upstream llama.cpp
// does not populate n_ctx_train from a vocab-only load.
//
// Spec: https://github.com/ggerganov/ggml/blob/master/docs/gguf.md
// (mirrored in llama.cpp/ggml/include/gguf.h).

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// gguf KV value types, matching enum gguf_type in gguf.h.
const (
	ggufTypeUint8   uint32 = 0
	ggufTypeInt8    uint32 = 1
	ggufTypeUint16  uint32 = 2
	ggufTypeInt16   uint32 = 3
	ggufTypeUint32  uint32 = 4
	ggufTypeInt32   uint32 = 5
	ggufTypeFloat32 uint32 = 6
	ggufTypeBool    uint32 = 7
	ggufTypeString  uint32 = 8
	ggufTypeArray   uint32 = 9
	ggufTypeUint64  uint32 = 10
	ggufTypeInt64   uint32 = 11
	ggufTypeFloat64 uint32 = 12
)

// errGGUFKeyNotFound is returned when a requested key is absent from the
// header. Callers treat this as "fall back to defaults" rather than a hard
// error.
var errGGUFKeyNotFound = errors.New("gguf: key not found")

// readGGUFContextLength parses just enough of the GGUF header at path to
// return the model's advertised training context window, i.e. the value
// of `{general.architecture}.context_length`. It reads the file
// sequentially — no full model load, no CGO, no mmap — and stops as soon
// as both values have been observed.
func readGGUFContextLength(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return 0, fmt.Errorf("gguf: read magic: %w", err)
	}
	if string(magic[:]) != "GGUF" {
		return 0, fmt.Errorf("gguf: bad magic %q", magic[:])
	}
	var version uint32
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return 0, fmt.Errorf("gguf: read version: %w", err)
	}
	// Versions 1 and 2 are obsolete and used different integer widths for
	// the counts. llama.cpp defines GGUF_VERSION 3 and all models we
	// handle are produced by tooling that writes v3+.
	if version < 2 {
		return 0, fmt.Errorf("gguf: unsupported version %d", version)
	}

	// Tensor count + KV count. v2+ uses uint64 for both.
	var nTensors, nKV uint64
	if err := binary.Read(r, binary.LittleEndian, &nTensors); err != nil {
		return 0, fmt.Errorf("gguf: read n_tensors: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &nKV); err != nil {
		return 0, fmt.Errorf("gguf: read n_kv: %w", err)
	}

	var arch string
	var ctxLen int
	ctxLenFound := false

	for range int(nKV) {
		key, err := readGGUFString(r)
		if err != nil {
			return 0, fmt.Errorf("gguf: read key: %w", err)
		}
		val, err := readGGUFValue(r)
		if err != nil {
			return 0, fmt.Errorf("gguf: read value for %q: %w", key, err)
		}
		switch key {
		case "general.architecture":
			if s, ok := val.(string); ok {
				arch = s
			}
		default:
			// We can't know the arch-specific context-length key until
			// general.architecture has been seen, but in practice it
			// appears early in the header. Check every key that ends in
			// ".context_length" and match it against arch once known.
			if arch != "" && key == arch+".context_length" {
				n, ok := asInt(val)
				if ok {
					ctxLen = n
					ctxLenFound = true
				}
			}
		}
		if ctxLenFound && arch != "" {
			return ctxLen, nil
		}
	}
	if !ctxLenFound {
		return 0, errGGUFKeyNotFound
	}
	return ctxLen, nil
}

// readGGUFString reads a GGUF-format string: u64 length followed by that
// many bytes (no trailing NUL).
func readGGUFString(r io.Reader) (string, error) {
	var n uint64
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return "", err
	}
	if n > 1<<20 {
		// Guardrail: a single KV key or string value over 1 MiB is
		// almost certainly a corrupt file — refuse it rather than
		// allocating unbounded memory.
		return "", fmt.Errorf("gguf: string length %d exceeds 1 MiB sanity cap", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// readGGUFValue reads a single typed KV value. The value is returned in
// the narrowest Go type that fits the GGUF type, so callers can
// type-assert (string) or use asInt for any integer-like variant.
func readGGUFValue(r io.Reader) (any, error) {
	var t uint32
	if err := binary.Read(r, binary.LittleEndian, &t); err != nil {
		return nil, err
	}
	return readGGUFTypedValue(r, t)
}

func readGGUFTypedValue(r io.Reader, t uint32) (any, error) {
	switch t {
	case ggufTypeUint8:
		var v uint8
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case ggufTypeInt8:
		var v int8
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case ggufTypeUint16:
		var v uint16
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case ggufTypeInt16:
		var v int16
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case ggufTypeUint32:
		var v uint32
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case ggufTypeInt32:
		var v int32
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case ggufTypeFloat32:
		var v float32
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case ggufTypeBool:
		var v uint8
		err := binary.Read(r, binary.LittleEndian, &v)
		return v != 0, err
	case ggufTypeString:
		return readGGUFString(r)
	case ggufTypeUint64:
		var v uint64
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case ggufTypeInt64:
		var v int64
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case ggufTypeFloat64:
		var v float64
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case ggufTypeArray:
		// We don't use array-valued KVs (tokenizer vocab, etc.) for
		// registry population. Skip past them without allocating the
		// full payload: read element type, element count, then advance
		// the reader by the serialized size. For string-element arrays
		// we have to walk each element since their size is variable.
		var elemType uint32
		if err := binary.Read(r, binary.LittleEndian, &elemType); err != nil {
			return nil, err
		}
		var n uint64
		if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
			return nil, err
		}
		for range int(n) {
			if _, err := readGGUFTypedValue(r, elemType); err != nil {
				return nil, err
			}
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("gguf: unknown type %d", t)
	}
}

// asInt converts any GGUF integer-typed value into a Go int. Returns
// (0, false) for non-integer types or for values that don't fit an int.
func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case uint8:
		return int(x), true
	case int8:
		return int(x), true
	case uint16:
		return int(x), true
	case int16:
		return int(x), true
	case uint32:
		return int(x), true
	case int32:
		return int(x), true
	case uint64:
		if x > 1<<62 {
			return 0, false
		}
		return int(x), true
	case int64:
		return int(x), true
	default:
		return 0, false
	}
}
