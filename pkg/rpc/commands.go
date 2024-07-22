package rpc

const (
	Unknown Command = iota
	SetLevel
)

type Command uint8

func (c Command) String() string {
	switch c {
	case SetLevel:
		return "SetLevel"
	default:
		return "Unknown"
	}
}
