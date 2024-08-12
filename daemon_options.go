package rxd

import (
	"os"
	"sync"

	"github.com/ambitiousfew/rxd/log"
)

type DaemonOption func(*daemon)

func WithLogWorkerCount(count int) DaemonOption {
	return func(d *daemon) {
		d.logWorkerCount = count
	}
}

func WithServiceLogger(logger log.Logger) DaemonOption {
	return func(d *daemon) {
		d.serviceLogger = logger
	}
}

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

// WithInternalLogger sets a custom logger for the daemon to use for internal logging.
// by default, the daemon will use a noop logger since this logger is used for rxd internals.
func WithInternalLogger(logger log.Logger) DaemonOption {
	return func(d *daemon) {
		d.internalLogger = logger
	}
}

// WithInternalLogging enables the internal logger to write to the filepath using the provided log level.
func WithInternalLogging(filepath string, level log.Level) DaemonOption {
	return func(d *daemon) {
		d.internalLogger = log.NewLogger(level, &daemonLogHandler{
			filepath: filepath,
			enabled:  true,
			total:    0,                // total bytes written to the log file
			limit:    10 * 1024 * 1024, // 10MB
			file:     nil,
			mu:       sync.RWMutex{},
		})
	}
}

// WithRPC enables an RPC server to run alongside the daemon.
// The RPC server will be available at the provided address and port.
// Currently the RPC server only supports a single method to change log level.
// An RPC client is provided in the pkg/rxrpc package for external use.
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
