package config

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"sync"
)

type Reader interface {
	Read(ctx context.Context) error
}

type Loader interface {
	Load(ctx context.Context) map[string]any
}

type ReadLoader interface {
	Reader
	Loader
}

type LoaderFn func(ctx context.Context, fields map[string]any) error

func FromReader(r io.Reader, serializer Serializer) ReadLoader {
	return &readerConfig{
		reader:     r,
		serializer: nil,
		fields:     make(map[string]any),
		mu:         sync.RWMutex{},
	}
}

func FromFile(path string, opts ...FileOption) (ReadLoader, error) {
	freader := &file{
		filepath: path,
	}

	conf := &fileConfig{
		file:       freader,
		serializer: nil,
		fields:     make(map[string]any),
		mu:         sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(conf)
	}

	if conf.serializer == nil {
		// try to pick the serializer based on the file extension
		switch filepath.Ext(path) {
		case ".json":
			conf.serializer = jsonSerializer{}
		default:
			return nil, errors.New("unsupported file extension")
		}
	}

	return conf, nil
}
