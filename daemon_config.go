package rxd

import (
	"os"

	"golang.org/x/exp/slog"
)

type DaemonConfig struct {
	Name               string       // name of the daemon will be used in logging
	Signals            []os.Signal  // OS signals you want your daemon to listen for
	LogHandler         slog.Handler // log handler for daemon/manager/services
	IntracomLogHandler slog.Handler // log handler for embedded intracom instances (debugging)
}
