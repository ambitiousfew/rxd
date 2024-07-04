package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ambitiousfew/rxd"
)

// HelloWorldAPIService must meet Service interface or line below errors.
var _ rxd.ServiceRunner = (*HelloWorldAPIService)(nil)

// HelloWorldAPIService create a struct for your service which requires a config field along with any other state
// your service might need to maintain throughout the life of the service.
type HelloWorldAPIService struct {
	// fields this specific server uses
	server *http.Server
}

// NewHelloWorldService just a factory helper function to help create and return a new instance of the service.
func NewHelloWorldService() *HelloWorldAPIService {
	return &HelloWorldAPIService{
		server: nil,
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *HelloWorldAPIService) Run(ctx rxd.ServiceContext) error {
	doneC := make(chan struct{})
	go func() {
		defer close(doneC)
		// DEBUG: we are trying to make the server shutdown to see if it will stop the service to check policy
		errTimeout := time.NewTimer(7 * time.Second)
		defer errTimeout.Stop()

		select {
		case <-ctx.Done():
		case <-errTimeout.C:
			ctx.LogInfo("timer has elapsed, forcing server shutdown", nil)
		}

		// NOTE: because Stop and Run cannot execute at the same time, we need to stop the server here
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.server.Shutdown(timeoutCtx); err != nil {
			ctx.LogError("error shutting down server", map[string]any{"error": err})
		} else {
			ctx.LogInfo("server shutdown", nil)
		}
	}()

	ctx.LogInfo("server starting", map[string]any{"address": s.server.Addr})
	// ListenAndServe will block forever serving requests/responses
	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return errors.New("server shutdown: " + err.Error())
	}

	<-doneC // wait for signal routine to finish...
	return nil
}

func (s *HelloWorldAPIService) Init(ctx rxd.ServiceContext) error {
	// handle initializing the primary focus of what the service will run.
	// Stop will be responsible for cleaning up any resources that were created during Init.
	if s.server != nil {
		ctx.LogError("server already initialized", nil)
		return errors.New("server already initialized")
	}

	ctx.LogInfo("entering init", nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"hello": "world"}`))
	})

	s.server = &http.Server{
		Addr:    ":8000",
		Handler: mux,
	}
	return nil
}

func (s *HelloWorldAPIService) Idle(ctx rxd.ServiceContext) error {
	ctx.LogInfo("entering idle", nil)
	return nil
}

func (s *HelloWorldAPIService) Stop(ctx rxd.ServiceContext) error {
	ctx.LogInfo("entering stop", nil)
	s.server = nil
	return nil
}

const DaemonName = "single-service"

// Example entrypoint
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// logger options

	// lopts := []rxd.LoggerOption{
	// 	rxd.UsingLogName(DaemonName),
	// 	rxd.UsingLogLevel(rxd.LogLevelDebug),
	// 	rxd.UsingLogTimeFormat("2006/01/02T15:04"),
	// }

	logger := rxd.NewDefaultLogger(rxd.LogLevelDebug)

	// daemon options
	dopts := []rxd.DaemonOption{
		rxd.UsingDaemonLogger(logger),
	}
	// Create a new daemon instance with a name and options
	daemon := rxd.NewDaemon(DaemonName, dopts...)

	// We create an instance of our service
	helloWorld := NewHelloWorldService()

	apiSvc := rxd.NewService("helloworld-api", helloWorld)

	err := daemon.AddService(apiSvc)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// Blocks main thread, runs added services in their own goroutines
	err = daemon.Start(ctx)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	logger.Info("successfully stopped daemon", nil)
}

// type myLogger struct {
// 	logger *log.Logger
// 	level  rxd.LogLevel
// }

// func (l *myLogger) log(level rxd.LogLevel, msg string, fields map[string]any) {
// 	// if the level is less than the logger level, we don't log
// 	if level < l.level {
// 		return
// 	}

// 	var b strings.Builder
// 	b.WriteString(time.Now().Format(time.RFC3339) + " ")
// 	b.WriteString("[" + level.String() + "] ")
// 	b.WriteString(msg + " ")

// 	for k, v := range fields {
// 		b.WriteString(" " + k + "=" + fmt.Sprintf("%v", v))
// 	}

// 	l.logger.Println(b.String())
// }

// func (l *myLogger) Error(msg string, fields map[string]any) {
// 	l.log(rxd.LogLevelError, msg, fields)
// }

// func (l *myLogger) Warn(msg string, fields map[string]any) {
// 	l.log(rxd.LogLevelWarning, msg, fields)
// }

// func (l *myLogger) Notice(msg string, fields map[string]any) {
// 	l.log(rxd.LogLevelNotice, msg, fields)
// }

// func (l *myLogger) Info(msg string, fields map[string]any) {
// 	l.log(rxd.LogLevelInfo, msg, fields)
// }

// func (l *myLogger) Debug(msg string, fields map[string]any) {
// 	l.log(rxd.LogLevelDebug, msg, fields)
// }

// func (l *myLogger) With(group string) rxd.Logger {
// 	return &myLogger{logger: l.logger}
// }
