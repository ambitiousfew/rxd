package rxd

import (
	"os"
	"time"
)

type DaemonConfig struct {
	Name            string        // name of the daemon will be used in logging
	Signals         []os.Signal   // OS signals you want your daemon to listen for
	WatchdogTimeout time.Duration // watchdog timeout in seconds
}
