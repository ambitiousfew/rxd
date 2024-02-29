package rxd

type RunPolicy int

func (p RunPolicy) String() string {
	switch p {
	case RunUntilSignaled:
		return "run-until-signaled"
	case RunUntilStopped:
		return "run-until-stopped"
	case RunUntilErrors:
		return "run-until-errors"
	case RunOnce:
		return "run-once"
	default:
		return "unknown"
	}
}

const (
	// RunUntilSignaled (default) will run the service until it is signaled to stop via OS Signal or context cancellation.
	RunUntilSignaled RunPolicy = iota
	// RunUntilStopped will run the service until it is moved to a Stop state.
	RunUntilStopped
	// RunUntilErrors will run the service until any lifecycle returns an error.
	RunUntilErrors
	// RunOnce will run the service one time whether it errors or succeeds.
	RunOnce
)
