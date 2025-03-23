package config

import (
	"context"
	"errors"
	"io"
	"sync"
)

type fileConfig struct {
	file       *file
	serializer Serializer
	fields     map[string]interface{}
	mu         sync.RWMutex
}

func (c *fileConfig) Read(ctx context.Context) error {
	reader, err := c.file.NewReader()
	if err != nil {
		return err
	}
	defer reader.Close()

	// read the contents of the entire reader
	contents, err := io.ReadAll(reader)
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

	c.mu.Lock()
	c.fields = fields
	c.mu.Unlock()

	return nil
}

// Load would be called by the service manager so the service managers would hold
// a user defined loader func to run per given service.
func (c *fileConfig) Load(ctx context.Context) map[string]any {
	// load the configuration into the provided interface
	c.mu.RLock()
	defer c.mu.RUnlock()

	fields := make(map[string]any, len(c.fields))
	for k, v := range c.fields {
		fields[k] = v
	}
	return fields
}
