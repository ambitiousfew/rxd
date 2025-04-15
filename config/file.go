package config

import (
	"context"
	"errors"
	"io"
	"sync"
)

func FromFile(path string, validator Decoder) (ReadLoader, error) {
	conf := &configFile{
		file:     file(path),
		decoder:  validator,
		contents: nil,
		mu:       sync.RWMutex{},
	}

	return conf, nil
}

type configFile struct {
	file     file
	decoder  Decoder
	contents []byte
	mu       sync.RWMutex
}

func (c *configFile) Read(ctx context.Context) error {
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

	// decode the contents into the provided interface.
	// this just validates the contents are in-fact valid.
	if err := c.decoder.Decode(contents); err != nil {
		return err
	}

	// store the contents of the last read
	c.mu.Lock()
	c.contents = contents
	c.mu.Unlock()

	return nil
}

// Load would be called by the service manager so the service managers would hold
// a user defined loader func to run per given service.
func (c *configFile) Load(ctx context.Context) ([]byte, error) {
	// load the configuration into the provided interface
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.contents) == 0 {
		return nil, errors.New("file contents were empty on read")
	}

	// return a copy contents of the last read
	b := make([]byte, len(c.contents))
	copy(b, c.contents)
	return b, nil
}
