package rxd

import "time"

// RestartPolicy service option type representing the restart policy of a given service
type RestartPolicy string

const (
	// Always will inform the manager to always continue run the service unless explicitly stopped.
	Always RestartPolicy = "always"
	// Once will inform themanager to stop the service after a successful run, but retry if fails.
	Once RestartPolicy = "once"
)

// ServiceOption are simply using an Option pattern to customize options
// such as restart policies, timeouts for a given service and how it should run.
type ServiceOption func(*ServiceOpts)

// ServiceOpts will allow for customizations of how a service runs and should always have
// a reasonable default to fallback if the case one isnt provided.
// This would be set by the ServiceConfig upon creation.
type ServiceOpts struct {
	RestartPolicy  RestartPolicy
	RestartTimeout time.Duration
}
