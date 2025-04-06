package config

import (
	"context"
	"io"
	"sync"
)

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

type LoaderFn func(ctx context.Context, from []byte) error

type DecoderFn[T any] func([]byte, *T) error

func FromReader(r io.Reader, decoder Decoder) ReadLoader {
	return &readerConfig{
		reader:  r,
		decoder: decoder,
		p:       nil,
		mu:      sync.RWMutex{},
	}
}

func FromFile(path string, decoder Decoder) (ReadLoader, error) {
	freader := &file{
		filepath: path,
	}

	conf := &fileConfig{
		file:    freader,
		decoder: decoder,
		p:       nil,
		mu:      sync.RWMutex{},
	}

	return conf, nil
}

func DecodeFromBytes(b []byte, decoder Decoder) error {
	if err := decoder.Decode(b); err != nil {
		return err
	}
	return nil
}
