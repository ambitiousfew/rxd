package config

import (
	"context"
	"errors"
	"io"
	"sync"
)

// FromFile is a generic function requiring a type that implements the Validator interface.
// It returns a ReadLoader that reads from a file and validates the contents using the provided validator.
func FromFile[T Validator](path string) (ReadLoader, error) {
	var validator T
	conf := &configFile{
		file:      file(path),
		validator: validator,
		contents:  nil,
		mu:        sync.RWMutex{},
	}

	return conf, nil
}

type configFile struct {
	file      file
	validator Validator
	contents  []byte
	mu        sync.RWMutex
}

func (c *configFile) Read(_ context.Context) error {
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

	// validate the bytes using the provided validator struct implementation
	if err := c.validator.Validate(contents); err != nil {
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
func (c *configFile) Load(_ context.Context) ([]byte, error) {
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
