package rxd

const (
	prefix                string = "_rxd"
	internalServiceStates string = prefix + ".states"

	// helper consts to build prefixes for internal consumer names of internal states
	internalServiceEnterStates string = prefix + ".enter.states"
	internalServiceExitStates  string = prefix + ".exit.states"
	internalServiceAllStates   string = prefix + ".all.states"
)
