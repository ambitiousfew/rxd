package rxd

type DaemonOption func(*daemon)

func UsingLogger(logger Logger) DaemonOption {
	return func(d *daemon) {
		d.log = logger
	}
}
