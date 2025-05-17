package config

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
)

func FromReader[T Validator](r io.Reader) ReadLoader {
	var validator T
	return &configReader{
		reader:    r,
		validator: validator,
		contents:  nil,
		mu:        sync.RWMutex{},
	}
}

type file string

func (f file) NewReader() (io.ReadCloser, error) {
	return os.Open(string(f))
}

type configReader struct {
	reader    io.Reader
	validator Validator
	contents  []byte
	mu        sync.RWMutex
}

func (c *configReader) Read(_ context.Context) error {
	// read the contents of the entire reader
	contents, err := io.ReadAll(c.reader)
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

func (c *configReader) Load(_ context.Context) ([]byte, error) {
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
