package toml

import (
	"bytes"

	"github.com/BurntSushi/toml"
	"unstable.build/go-tui/storage/encoding"
)

// Marshaler returns a TOML Marshaler.
func Marshaler() encoding.Marshaler {
	return tomlMarshaler{}
}

type tomlMarshaler struct {
}

func (j tomlMarshaler) Marshal(in interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := toml.NewEncoder(&buf).Encode(in)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (j tomlMarshaler) Unmarshal(data []byte, to interface{}) error {
	_, err := toml.NewDecoder(bytes.NewReader(data)).Decode(to)
	return err
}
