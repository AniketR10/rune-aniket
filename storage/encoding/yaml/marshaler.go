package yaml

import (
	"gopkg.in/yaml.v3"
	"unstable.build/go-tui/storage/encoding"
)

// Marshaler returns a YAML Marshaler.
func Marshaler() encoding.Marshaler {
	return yamlMarshaler{}
}

type yamlMarshaler struct {
}

func (j yamlMarshaler) Marshal(in interface{}) ([]byte, error) {
	return yaml.Marshal(in)
}

func (j yamlMarshaler) Unmarshal(data []byte, to interface{}) error {
	return yaml.Unmarshal(data, to)
}
