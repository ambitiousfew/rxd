package rxd

import "os"

type DaemonOption func(*daemon)

// WithReportAlive sets the interval in seconds for when the daemon should report that it is still alive
// to the service manager. If the value is set to 0, the daemon will not interact with the service manager.
func WithReportAlive(timeoutSecs uint64) DaemonOption {
	return func(d *daemon) {
		d.reportAliveSecs = timeoutSecs
	}
}

// WithSignals sets the OS signals that the daemon should listen for. If no signals are provided, the daemon
// will listen for SIGINT and SIGTERM by default.
func WithSignals(signals ...os.Signal) DaemonOption {
	return func(d *daemon) {
		d.signals = signals
	}
}

func WithRPC(cfg RPCConfig) DaemonOption {
	return func(d *daemon) {
		d.rpcEnabled = true

		addr := cfg.Addr
		port := cfg.Port

		if addr == "" {
			addr = "127.0.0.1"
		}

		if port == 0 {
			port = 1337
		}

		d.rpcConfig = RPCConfig{
			Addr: addr,
			Port: port,
		}
	}
}
