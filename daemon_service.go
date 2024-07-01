package rxd

type DaemonService struct {
	Name    string
	Runner  ServiceRunner
	Handler ServiceHandler
}
