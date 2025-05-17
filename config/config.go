package config

import (
	"context"
	"errors"
)

// Validator is an interface meant for validating a custom daemon config
type Validator interface {
	Validate(p []byte) error
}

// Decoder is an interface meant for decoding individual service configs
type Decoder interface {
	Decode(from []byte) error
}

// Reader is an interface meant for reading a custom daemon config from a source
type Reader interface {
	Read(ctx context.Context) error
}

// Loader is an interface meant for service managers to invoke Load to retrieve
// the most recently read configuration bytes.
type Loader interface {
	Load(ctx context.Context) ([]byte, error)
}

// ReadLoader is an interface that combines the Reader and Loader interfaces.
type ReadLoader interface {
	Reader
	Loader
}

// EnvEncoder is a function that encodes the a map of environment variables into a byte array
type EnvEncoder func(vars map[string]string) ([]byte, error)

// EnvDecoder is a function that decodes a byte array into a map of environment variables
type EnvDecoder func(contents []byte) (map[string]string, error)

// DecodeFromBytes is a convenience function to decode the contents of a byte slice
// into a service config struct. It is meant to be called from inside the service Load method.
func DecodeFromBytes(from []byte, into Decoder) error {
	if len(from) == 0 {
		return errors.New("file contents were empty on read")
	}

	return into.Decode(from)
}
