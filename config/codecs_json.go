package config

import "encoding/json"

type JSONDecoder[T any] struct{}

func (j JSONDecoder[T]) Decode(b []byte, into *T) error {
	return json.Unmarshal(b, into)
}

type JSONEncoder[T any] struct{}

func (j JSONEncoder[T]) Encode(from T) ([]byte, error) {
	return json.Marshal(from)
}

type JSONCodec[T any] struct{}

func (j JSONCodec[T]) Encode(from T) ([]byte, error) {
	return json.Marshal(from)
}

func (j JSONCodec[T]) Decode(b []byte, into *T) error {
	return json.Unmarshal(b, into)
}
