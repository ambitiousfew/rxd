# RxDaemon
A simple (alpha) reactive services daemon

## Example Service Template

```go

// Define a service struct if you wish to use receiver methods for your Init, Idle, Run, Stop methods
// Though you only need to implement the methods you want to customize if they need to do something.
type SimpleService struct{}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *SimpleService) Run(c *rxd.ServiceContext) rxd.ServiceResponse {

	c.Log.Info("has entered the run state")

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

  // NOTE: For any stage function that has any blocking calls or has to do any waiting.
  // You will ALWAYS want to use the pattern below, 'default' may be used in place
  // of the timer.C case if you dont need an interval timer.
  // But you should ALWAYS watch for a shutdown signal so daemon/manager can shutdown 
  // the services properly and you can have the chance to gracefully stop your service
  // before its killed off or to prevent leaks.
	for {
		select {
		case <-c.ShutdownSignal():
			// ALWAYS watch for shutdown signal
			return rxd.NewResponse(nil, rxd.ExitState)

		case <-timer.C:
			// When 5 seconds has elapsed, log hello, then end the service.
			c.Log.Info("hello")
			return rxd.NewResponse(nil, rxd.StopState)
		}

	}
}

func (s *SimpleService) Init(c *rxd.ServiceContext) rxd.ServiceResponse {
	return rxd.NewResponse(nil, rxd.IdleState)
}

func (s *SimpleService) Idle(c *rxd.ServiceContext) rxd.ServiceResponse {
	return rxd.NewResponse(nil, rxd.RunState)
}

func (s *SimpleService) Stop(c *rxd.ServiceContext) rxd.ServiceResponse {
	return rxd.NewResponse(nil, rxd.ExitState)
}

// SimpleService must meet Service interface or line below errors.
var _ rxd.Service = &SimpleService{}
```

### Example Daemon Entrypoint with Custom Logger

```go
// Custom Logger use a struct that implements Debug, Info, Error to meet the Logging interface.
type MyLogger struct{}

func (ml *MyLogger) Debug(v any) {
	fmt.Printf("[DBG] %s", v)
}

func (ml *MyLogger) Info(v any) {
	fmt.Printf("[INF] %s", v)
}

func (ml *MyLogger) Error(v any) {
	fmt.Printf("[ERR] %s", v)
}

```

```go
// Example entrypoint
func main() {
	// We create an instance of our service
	simpleService := &SimpleService{}
	// We create an instance of our ServiceConfig
	svcOpts := rxd.NewServiceOpts(rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy))
	simpleRxdService := rxd.NewService("SimpleService", simpleService, svcOpts)

	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon(simpleRxdService)

	// since MyLogger meets the Logging interface we can allow the daemon to use it.
	logger := &MyLogger{}
	daemon.SetLogger(logger)

	err := daemon.Start() // Blocks main thread
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	log.Println("successfully stopped daemon")
}
```
