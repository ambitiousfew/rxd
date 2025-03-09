package config

import (
	"context"
	"os"
	"sync"
)

type Reader interface {
	Read(ctx context.Context) error
}

type Loader interface {
	Load(ctx context.Context, loader LoaderFn) error
}

type LoaderFn func(ctx context.Context, fields map[string]any) error

type ReadLoader interface {
	Reader
	Loader
}

func FromJSONFile(path string) ReadLoader {
	return &fileConfig{
		path:       path,
		serializer: jsonSerializer{},
		fields:     make(map[string]interface{}),
		mu:         sync.RWMutex{},
	}
}

type fileConfig struct {
	path       string
	serializer Serializer
	fields     map[string]interface{}
	mu         sync.RWMutex
}

func (f *fileConfig) Read(ctx context.Context) error {
	// read the file and populate the fields
	contents, err := os.ReadFile(f.path)
	if err != nil {
		return err
	}
	// parse the contents into the fields
	fields, err := f.serializer.Serialize(contents)
	if err != nil {
		return err
	}

	f.mu.Lock()
	f.fields = fields
	f.mu.Unlock()

	return nil
}

// Load would be called by the service manager so the service managers would hold
// a user defined loader func to run per given service.
func (f *fileConfig) Load(ctx context.Context, loader LoaderFn) error {
	// load the configuration into the provided interface
	f.mu.RLock()
	defer f.mu.RUnlock()
	return loader(ctx, f.fields)
}
