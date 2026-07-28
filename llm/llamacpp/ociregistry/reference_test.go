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

package ociregistry_test

import (
	"strings"
	"testing"

	"unstable.build/go-tui/llm/llamacpp/ociregistry"
)

func TestParseReference(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  ociregistry.Reference
	}{
		{
			name:  "hf with user-repo and tag",
			input: "hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q4_K_M",
			want: ociregistry.Reference{
				Host:       "hf.co",
				Repository: "bartowski/Llama-3.2-1B-Instruct-GGUF",
				Tag:        "Q4_K_M",
			},
		},
		{
			name:  "hf with huggingface.co",
			input: "huggingface.co/bartowski/repo",
			want: ociregistry.Reference{
				Host:       "huggingface.co",
				Repository: "bartowski/repo",
			},
		},
		{
			name:  "scheme stripped",
			input: "https://hf.co/bartowski/repo:q4",
			want: ociregistry.Reference{
				Host:       "hf.co",
				Repository: "bartowski/repo",
				Tag:        "q4",
			},
		},
		{
			name:  "digest-only reference",
			input: "registry.ollama.ai/library/llama3@sha256:" + strings.Repeat("a", 64),
			want: ociregistry.Reference{
				Host:       "registry.ollama.ai",
				Repository: "library/llama3",
				Digest:     "sha256:" + strings.Repeat("a", 64),
			},
		},
		{
			name:  "both tag and digest",
			input: "hf.co/u/r:v@sha256:" + strings.Repeat("b", 64),
			want: ociregistry.Reference{
				Host:       "hf.co",
				Repository: "u/r",
				Tag:        "v",
				Digest:     "sha256:" + strings.Repeat("b", 64),
			},
		},
		{
			name:  "localhost with port is a host",
			input: "localhost:5000/foo/bar:v1",
			want: ociregistry.Reference{
				Host:       "localhost:5000",
				Repository: "foo/bar",
				Tag:        "v1",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ociregistry.ParseReference(tc.input)
			if err != nil {
				t.Fatalf("ParseReference(%q): %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("ParseReference(%q):\n got  %+v\n want %+v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseReference_DefaultHost(t *testing.T) {
	ref, err := ociregistry.ParseReferenceWithDefault("library/llama3:8b", "registry.ollama.ai")
	if err != nil {
		t.Fatalf("ParseReferenceWithDefault: %v", err)
	}
	if ref.Host != "registry.ollama.ai" || ref.Repository != "library/llama3" || ref.Tag != "8b" {
		t.Fatalf("unexpected: %+v", ref)
	}
}

// TestParseReference_HFDefault verifies that bare `owner/repo` input is
// parsed against a huggingface.co default — the case the `download`
// command relies on.
func TestParseReference_HFDefault(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  ociregistry.Reference
	}{
		{
			name:  "bare owner/repo",
			input: "unsloth/gemma-3n-E2B-it-GGUF",
			want: ociregistry.Reference{
				Host:       "huggingface.co",
				Repository: "unsloth/gemma-3n-E2B-it-GGUF",
			},
		},
		{
			name:  "bare with tag",
			input: "unsloth/gemma-3n-E2B-it-GGUF:Q4_K_M",
			want: ociregistry.Reference{
				Host:       "huggingface.co",
				Repository: "unsloth/gemma-3n-E2B-it-GGUF",
				Tag:        "Q4_K_M",
			},
		},
		{
			name:  "explicit host wins over default",
			input: "docker.io/library/alpine:3",
			want: ociregistry.Reference{
				Host:       "docker.io",
				Repository: "library/alpine",
				Tag:        "3",
			},
		},
		{
			name:  "scheme + hostless (not a real form but should not crash)",
			input: "https://foo/bar",
			want: ociregistry.Reference{
				// `foo` is the first segment and looks like a host only
				// if it contains `.` or `:`; here it does not, so the
				// default host applies.
				Host:       "huggingface.co",
				Repository: "foo/bar",
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ociregistry.ParseReferenceWithDefault(tc.input, "huggingface.co")
			if err != nil {
				t.Fatalf("ParseReferenceWithDefault(%q): %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("\n got  %+v\n want %+v", got, tc.want)
			}
		})
	}
}

func TestParseReference_Invalid(t *testing.T) {
	cases := []string{
		"",
		"bareword",
		"hf.co/:tag",
		"hf.co/repo:",
		"hf.co/repo@sha256:tooshort",
		"hf.co/repo@unknown:abc",
	}
	for _, c := range cases {
		c := c
		t.Run(c, func(t *testing.T) {
			if _, err := ociregistry.ParseReference(c); err == nil {
				t.Fatalf("expected error parsing %q", c)
			}
		})
	}
}

func TestReferenceString_RoundTrip(t *testing.T) {
	in := "hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF:Q4_K_M"
	ref, err := ociregistry.ParseReference(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := ref.String(); got != in {
		t.Fatalf("round trip:\n got  %q\n want %q", got, in)
	}
}

func TestReferenceTarget(t *testing.T) {
	digest := "sha256:" + strings.Repeat("c", 64)
	r, _ := ociregistry.ParseReference("hf.co/u/r@" + digest)
	if r.Target() != digest {
		t.Fatalf("digest target: got %q want %q", r.Target(), digest)
	}
	r, _ = ociregistry.ParseReference("hf.co/u/r:v")
	if r.Target() != "v" {
		t.Fatalf("tag target: got %q want %q", r.Target(), "v")
	}
	r, _ = ociregistry.ParseReference("hf.co/u/r")
	if r.Target() != ociregistry.DefaultTag {
		t.Fatalf("default target: got %q want %q", r.Target(), ociregistry.DefaultTag)
	}
}
