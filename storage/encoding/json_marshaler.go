package encoding

import "encoding/json"

// MarshalerJSON returns a JSON Marshaler.
func MarshalerJSON() Marshaler {
	return jsonMarshaler{}
}

type jsonMarshaler struct {
}

func (j jsonMarshaler) Marshal(in interface{}) ([]byte, error) {
	return json.Marshal(in)
}

func (j jsonMarshaler) Unmarshal(data []byte, to interface{}) error {
	return json.Unmarshal(data, to)
}
