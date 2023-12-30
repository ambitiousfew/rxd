package rxd

type rxdSignal int

const (
	prefix                 string = "_rxd"
	internalServiceStates  string = prefix + ".states"
	internalSignals        string = prefix + ".signals"
	internalSignalsManager string = prefix + ".signals.manager"
	// helper consts to build prefixes for internal consumer names of internal states
	internalServiceEnterStates string = prefix + ".enter.states"
	internalServiceExitStates  string = prefix + ".exit.states"
	internalServiceAllStates   string = prefix + ".all.states"
)

// constants used by daemon to signal to manager
// TODO: we can potentially use custom signals to signal to manager
// to trigger other behavior on the services like reload, restart, etc.
// without having to shutdown the daemon/manager entirely.
const (
	signalStart rxdSignal = iota
	signalReload
	signalStop // currently the only signal being used
	signalRestart
)
