// Copyright (C) 2017-2026 Unstable Build, LLC
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

package ociregistry

import "encoding/json"

// Media types we emit or expect. Docker v2 is what ollama.com and Hugging
// Face currently return; OCI is what conformant registries produce. We
// accept both transparently via the manifest Accept header.
const (
	MediaTypeDockerManifestV2   = "application/vnd.docker.distribution.manifest.v2+json"
	MediaTypeDockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
	MediaTypeOCIManifest        = "application/vnd.oci.image.manifest.v1+json"
	MediaTypeOCIIndex           = "application/vnd.oci.image.index.v1+json"

	// MediaTypeOllamaModel is the layer type used for GGUF weight blobs by
	// both ollama.com and Hugging Face's OCI endpoint. The other Ollama
	// media types (template, params, system) are exposed for completeness.
	MediaTypeOllamaModel     = "application/vnd.ollama.image.model"
	MediaTypeOllamaProjector = "application/vnd.ollama.image.projector"
	MediaTypeOllamaTemplate  = "application/vnd.ollama.image.template"
	MediaTypeOllamaParams    = "application/vnd.ollama.image.params"
	MediaTypeOllamaSystem    = "application/vnd.ollama.image.system"
	MediaTypeOllamaAdapter   = "application/vnd.ollama.image.adapter"
	MediaTypeOllamaLicense   = "application/vnd.ollama.image.license"
)

// Descriptor describes a blob referenced by a manifest.
type Descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	URLs        []string          `json:"urls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Manifest is a Docker v2 / OCI image manifest. The two shapes are
// structurally identical for our purposes; we preserve the top-level
// MediaType verbatim so callers can tell which one a registry returned.
type Manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType,omitempty"`
	Config        Descriptor   `json:"config"`
	Layers        []Descriptor `json:"layers"`

	// Raw is the original JSON bytes, retained so callers can digest the
	// manifest exactly as served.
	Raw []byte `json:"-"`
}

// FindLayer returns the first layer with the given media type, or nil.
func (m *Manifest) FindLayer(mediaType string) *Descriptor {
	for i := range m.Layers {
		if m.Layers[i].MediaType == mediaType {
			return &m.Layers[i]
		}
	}
	return nil
}

// UnmarshalManifest decodes a manifest from its JSON form and stashes the
// original bytes on the result.
func UnmarshalManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	m.Raw = append([]byte(nil), data...)
	return &m, nil
}

// TagList is the response shape for GET /v2/{repo}/tags/list.
type TagList struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}
