package config

type Encoder interface {
	Encode() ([]byte, error)
}

type Decoder interface {
	Decode([]byte) error
}

type Codec interface {
	Encoder
	Decoder
}
