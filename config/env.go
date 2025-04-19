package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
)

type EnvEncoder func(vars map[string]string) ([]byte, error)
type EnvDecoder func(contents []byte) (map[string]string, error)

func FromEnvironment(opts ...EnvOption) (ReadLoader, error) {
	conf := &configEnv{
		encoder:  EnvJSONEncoder,
		prefix:   "",
		contents: nil,
		mu:       sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(conf)
	}

	return conf, nil
}

type configEnv struct {
	prefix     string
	trimPrefix bool
	encoder    EnvEncoder
	contents   []byte
	mu         sync.RWMutex
}

func (c *configEnv) Read(ctx context.Context) error {
	envVars := os.Environ()

	vars := make(map[string]string)

	for _, envVar := range envVars {
		kvs := strings.Split(envVar, "=")
		if len(kvs) != 2 {
			continue
		}

		// if prefix is set, check if the environment variable starts with the prefix
		if c.prefix != "" && !strings.HasPrefix(kvs[0], c.prefix) {
			// skip this environment variable if it doesn't start with the prefix
			continue
		}

		envVar = kvs[0]
		// if trimPrefix is set, trim the prefix from the environment variable
		if c.trimPrefix {
			envVar = strings.TrimPrefix(envVar, c.prefix)
		}

		vars[envVar] = kvs[1]
	}

	contents, err := c.encoder(vars)
	if err != nil {
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
func (c *configEnv) Load(ctx context.Context) ([]byte, error) {
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

func EnvJSONEncoder(vars map[string]string) ([]byte, error) {
	contents, err := json.Marshal(vars)
	if err != nil {
		return nil, err
	}

	return contents, nil
}

func EnvJSONDecoder(vars map[string]string) ([]byte, error) {
	contents, err := json.Marshal(vars)
	if err != nil {
		return nil, err
	}

	return contents, nil
}
