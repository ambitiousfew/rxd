package config

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
)

func (f *file) NewReader() (io.ReadCloser, error) {
	return os.Open(f.filepath)
}

type file struct {
	filepath string
}

type readerConfig struct {
	reader     io.Reader
	serializer Serializer
	fields     map[string]interface{}
	mu         sync.RWMutex
}

func (c *readerConfig) Read(ctx context.Context) error {
	// read the contents of the entire reader
	contents, err := io.ReadAll(c.reader)
	if err != nil {
		return err
	}

	if len(contents) == 0 {
		return errors.New("file contents were empty on read")
	}

	// parse the contents into the fields
	fields, err := c.serializer.Serialize(contents)
	if err != nil {
		return err
	}

	c.fields = fields

	return nil
}

func (c *readerConfig) Load(ctx context.Context) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fields := make(map[string]any, len(c.fields))
	for k, v := range c.fields {
		fields[k] = v
	}
	return fields
}
