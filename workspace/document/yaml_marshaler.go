package document

import "gopkg.in/yaml.v3"

// MarshalerYAML returns a YAML Marshaler.
func MarshalerYAML() Marshaler {
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
