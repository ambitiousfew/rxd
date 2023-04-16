# RxDaemon
A simple (alpha) reactive services daemon

## Example Service Template

```go

// Define a service struct if you wish to use receiver methods for your Init, Idle, Run, Stop methods
// Though you only need to implement the methods you want to customize if they need to do something.
type SimpleService struct{}

func NewSimpleService() *SimpleService {
	return &SimpleService{}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *SimpleService) Run(c *rxd.ServiceContext) rxd.ServiceResponse {
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
			c.LogInfo("hello")
			return rxd.NewResponse(nil, rxd.StopState)
		}

	}
}
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
	simpleService := NewSimpleService()

	// Customize our service options
	svcCtx := rxd.NewServiceOpts(
		rxd.UsingRunPolicy(rxd.RunUntilStoppedPolicy),
	)

  // Create our service with a name and customized options.
	svc := rxd.NewService("SimpleService", svcOpts)

	// We could have used a pure function or you can pass receiver function
	// as long as it meets the interface for stageFunc
	svc.UsingRunStage(simpleService.Run)

	// We can even use an inline function instead of a receiver method
	svc.UsingStopStage(func(c *rxd.ServiceContext) rxd.ServiceResponse {
		c.LogInfo("we are stopping now...")
		return rxd.NewResponse(nil, rxd.ExitState)
	})

	// We pass 1 or more potentially long-running services to NewDaemon to run.
	daemon := rxd.NewDaemon(svc)

  // If you wish to use your own logger, ensure you meet the logging interface
	logger := &MyLogger{}
	daemon.SetLogger(logger)

	err := daemon.Start() // Blocks main thread
	if err != nil {
		logger.Error(err)
		os.Exit(1)
	}

	logger.Info("successfully stopped daemon")
}

```
