package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
)

// EnvJSONEncoder is a function that encodes the environment variables into a JSON byte array.
func EnvJSONEncoder(vars map[string]string) ([]byte, error) {
	contents, err := json.Marshal(vars)
	if err != nil {
		return nil, err
	}

	return contents, nil
}

// EnvJSONDecoder is a function that decodes the JSON byte array into a map of environment variables.
func EnvJSONDecoder(p []byte) (map[string]string, error) {
	var vars map[string]string
	err := json.Unmarshal(p, &vars)
	if err != nil {
		return nil, err
	}

	return vars, nil
}

// FromEnvironment is a function that creates a new environment variable read loader.
// Read will read from the OS environment variables and Load will return the most recently read
// environment variables as a byte array. The environment variables are encoded using the provided
// encoder function. The prefix is used to filter the environment variables that are read.
// If the prefix is set, only environment variables that start with the prefix will be read.
// If trim is set, the prefix will be trimmed from the environment variable name before it is returned.
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

func (c *configEnv) Read(_ context.Context) error {
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
func (c *configEnv) Load(_ context.Context) ([]byte, error) {
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
