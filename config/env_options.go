package config

// EnvOption is a function that modifies the configuration of the environment
type EnvOption func(*configEnv)

// WithEnvEncoder is an function option that sets the encoder for the environment variable reader.
func WithEnvEncoder(encoder EnvEncoder) EnvOption {
	return func(c *configEnv) {
		c.encoder = encoder
	}
}

// WithEnvPrefix is a function option that sets the prefix for the environment variable reader.
func WithEnvPrefix(prefix string, trim bool) EnvOption {
	return func(c *configEnv) {
		c.prefix = prefix
		c.trimPrefix = trim
	}
}
