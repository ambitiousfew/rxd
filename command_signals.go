package rxd

type CommandSignal uint8

const (
	CommandSignalUnknown CommandSignal = iota
	CommandSignalStop
	CommandSignalStart
	CommandSignalRestart
	CommandSignalReload
)
