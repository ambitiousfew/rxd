package config

import "encoding/json"

type Serializer interface {
	Serialize([]byte) (map[string]any, error)
	Deserialize(map[string]any, any) error
}

type jsonSerializer struct{}

func (j jsonSerializer) Serialize(b []byte) (map[string]any, error) {
	var fields map[string]any
	err := json.Unmarshal(b, &fields)
	return fields, err
}

func (j jsonSerializer) Deserialize(fields map[string]any, iface any) error {
	b, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, iface)
}
