package config

type EnvOption func(*configEnv)

func WithEnvEncoder(encoder EnvEncoder) EnvOption {
	return func(c *configEnv) {
		c.encoder = encoder
	}
}

func WithEnvPrefix(prefix string, trim bool) EnvOption {
	return func(c *configEnv) {
		c.prefix = prefix
		c.trimPrefix = trim
	}
}
