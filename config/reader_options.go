package config

type ReaderOption func(*readerConfig)

func WithReaderSerializer(s Serializer) ReaderOption {
	return func(c *readerConfig) {
		c.serializer = s
	}
}
