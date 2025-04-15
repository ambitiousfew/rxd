package config

import (
	"context"
	"errors"
)

type Decoder interface {
	Decode(from []byte) error
}

type Reader interface {
	Read(ctx context.Context) error
}

type Loader interface {
	Load(ctx context.Context) ([]byte, error)
}

type ReadLoader interface {
	Reader
	Loader
}

func DecodeFromBytes(from []byte, into Decoder) error {
	if len(from) == 0 {
		return errors.New("file contents were empty on read")
	}

	if err := into.Decode(from); err != nil {
		return err
	}

	return nil
}
