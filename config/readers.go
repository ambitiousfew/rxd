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
	reader  io.Reader
	decoder Decoder
	p       []byte
	mu      sync.RWMutex
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

	// decode the contents into the provided interface.
	// this just validates the contents are in-fact valid.
	if err := c.decoder.Decode(contents); err != nil {
		return err
	}

	// store the contents of the last read
	c.mu.Lock()
	c.p = contents
	c.mu.Unlock()
	return nil
}

func (c *readerConfig) Load(ctx context.Context) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.p) == 0 {
		return nil, errors.New("file contents were empty on read")
	}

	// return a copy contents of the last read
	b := make([]byte, len(c.p))
	copy(b, c.p)
	return b, nil
}
