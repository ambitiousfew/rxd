package rpc

type Command uint8

const (
	CommandUnknown Command = iota
	CommandLogLevel
)

func CommandFromString(s string) Command {
	switch s {
	case "loglevel":
		return CommandLogLevel
	default:
		return CommandUnknown
	}
}

type CommandSignal uint8

const (
	// CommandSignalUnknown is an unknown command signal
	CommandSignalUnknown CommandSignal = iota
	// CommandSignalStop is a stop command signal
	CommandSignalStop
	// CommandSignalStart is a start command signal
	CommandSignalStart
	// CommandSignalRestart is a restart command signal
	CommandSignalRestart
	// CommandSignalReload is a reload command signal
	CommandSignalReload
)

func CommandSignalFromString(s string) CommandSignal {
	switch s {
	case "stop":
		return CommandSignalStop
	case "start":
		return CommandSignalStart
	case "restart":
		return CommandSignalRestart
	case "reload":
		return CommandSignalReload
	default:
		return CommandSignalUnknown
	}
}

type CommandSignalPayload struct {
	Service string
	Signal  CommandSignal
}
