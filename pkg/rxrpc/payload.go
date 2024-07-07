package rxrpc

type CommandResponse struct {
	Command Command
	Success bool
	Body    []byte
}

type CommandPayload struct {
	Service  string          // Service name
	Command  Command         // Command to send
	Response CommandResponse // Response from the RPC server
}
