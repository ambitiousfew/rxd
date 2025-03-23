package config

type FileOption func(*fileConfig)

func WithFileSerializer(s Serializer) FileOption {
	return func(c *fileConfig) {
		c.serializer = s
	}
}
