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
