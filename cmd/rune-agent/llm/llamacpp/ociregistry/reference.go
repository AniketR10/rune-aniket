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


package ociregistry

import (
	"fmt"
	"strings"

	"oras.land/oras-go/v2/registry"
)

// DefaultTag is applied when a reference has no explicit tag.
const DefaultTag = "latest"

// Reference is a parsed OCI image reference. We keep a Go-native struct
// rather than exposing oras-go's registry.Reference so that callers don't
// couple to the transitive type and so we can relax oras-go's strict
// lowercase-only repository validator — Hugging Face repos such as
// "bartowski/Llama-3.2-1B-Instruct-GGUF" contain uppercase characters that
// fail registry.ValidateRepository.
type Reference struct {
	Host       string
	Repository string
	Tag        string
	Digest     string
}

// String renders the reference back to its canonical form.
func (r Reference) String() string {
	var b strings.Builder
	if r.Host != "" {
		b.WriteString(r.Host)
		b.WriteByte('/')
	}
	b.WriteString(r.Repository)
	if r.Digest != "" {
		b.WriteByte('@')
		b.WriteString(r.Digest)
	} else if r.Tag != "" {
		b.WriteByte(':')
		b.WriteString(r.Tag)
	}
	return b.String()
}

// Target returns the digest when set, else the tag, else DefaultTag.
// Used as the {reference} path segment in /v2/{repo}/manifests/{reference}.
func (r Reference) Target() string {
	switch {
	case r.Digest != "":
		return r.Digest
	case r.Tag != "":
		return r.Tag
	default:
		return DefaultTag
	}
}

// orasReference converts r into an oras-go registry.Reference. The conversion
// bypasses registry.ParseReference so we don't hit the strict lowercase
// validator; oras-go still validates tags and digests at request time.
func (r Reference) orasReference() registry.Reference {
	ref := registry.Reference{
		Registry:   r.Host,
		Repository: r.Repository,
	}
	if r.Digest != "" {
		ref.Reference = r.Digest
	} else if r.Tag != "" {
		ref.Reference = r.Tag
	}
	return ref
}

// ParseReference parses a reference string. Unlike registry.ParseReference,
// this accepts uppercase repository characters so Hugging Face references
// parse cleanly.
func ParseReference(s string) (Reference, error) {
	return ParseReferenceWithDefault(s, "")
}

// ParseReferenceWithDefault parses a reference, filling in defaultHost when
// the input does not include one.
func ParseReferenceWithDefault(s, defaultHost string) (Reference, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Reference{}, fmt.Errorf("ociregistry: empty reference")
	}
	// Strip scheme if present (e.g. copy-pasted https://hf.co/...).
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}

	// Split off @digest.
	var digest string
	if i := strings.LastIndex(raw, "@"); i >= 0 {
		digest = raw[i+1:]
		raw = raw[:i]
		if err := validateDigest(digest); err != nil {
			return Reference{}, err
		}
	}

	// Detect host.
	host := defaultHost
	rest := raw
	if i := strings.Index(raw, "/"); i >= 0 {
		if head := raw[:i]; looksLikeHost(head) {
			host = head
			rest = raw[i+1:]
		}
	}
	if host == "" {
		return Reference{}, fmt.Errorf("ociregistry: reference %q has no host and no default provided", s)
	}

	// Split trailing :tag. The tag can only follow the final path segment,
	// so we only look at the last colon.
	var tag string
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		tag = rest[i+1:]
		rest = rest[:i]
		if tag == "" {
			return Reference{}, fmt.Errorf("ociregistry: reference %q has empty tag", s)
		}
	}

	if rest == "" {
		return Reference{}, fmt.Errorf("ociregistry: reference %q has empty repository", s)
	}
	if strings.HasPrefix(rest, "/") || strings.HasSuffix(rest, "/") {
		return Reference{}, fmt.Errorf("ociregistry: reference %q has malformed repository", s)
	}

	return Reference{
		Host:       host,
		Repository: rest,
		Tag:        tag,
		Digest:     digest,
	}, nil
}

func looksLikeHost(s string) bool {
	if s == "localhost" || s == "hf.co" || s == "huggingface.co" {
		return true
	}
	return strings.ContainsAny(s, ".:")
}

func validateDigest(d string) error {
	i := strings.Index(d, ":")
	if i <= 0 || i == len(d)-1 {
		return fmt.Errorf("ociregistry: malformed digest %q", d)
	}
	alg := d[:i]
	hex := d[i+1:]
	switch alg {
	case "sha256":
		if len(hex) != 64 {
			return fmt.Errorf("ociregistry: sha256 digest %q has wrong length", d)
		}
	case "sha512":
		if len(hex) != 128 {
			return fmt.Errorf("ociregistry: sha512 digest %q has wrong length", d)
		}
	default:
		return fmt.Errorf("ociregistry: unsupported digest algorithm %q", alg)
	}
	for _, c := range hex {
		if !isHex(byte(c)) {
			return fmt.Errorf("ociregistry: digest %q contains non-hex characters", d)
		}
	}
	return nil
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
