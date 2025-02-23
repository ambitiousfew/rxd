package rxd

const (
	prefix string = "_rxd"
	// helper consts to build prefixes for internal consumer names of internal states
	internalServiceStates  string = prefix + ".states"
	internalSignals        string = prefix + ".signals"
	internalSignalsManager string = prefix + ".signals.manager"
	internalCommandSignals string = prefix + ".command.signals"
)
