package rxd

type ServiceConfig struct {
	// TODO: this becomes a RunPolicy instead with reasonable defaults.
	RunOpts []ServiceOption
	Name    string

	opts *serviceOpts
}
