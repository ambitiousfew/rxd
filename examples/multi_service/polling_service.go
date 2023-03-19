package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ambitiousfew/rxd"
)

// APIPollingService create a struct for your service which requires a config field along with any other state
// your service might need to maintain throughout the life of the service.
type APIPollingService struct {
	// cfg can be named anything but it MUST exist as *rxdaemon.ServiceConfig, Config() method will return it.
	cfg *rxd.ServiceConfig

	// fields this specific server uses
	client        *http.Client
	apiBase       string
	retryDuration time.Duration
	maxPollCount  int
}

// NewAPIPollingService just a factory helper function to help create and return a new instance of the service.
func NewAPIPollingService(cfg *rxd.ServiceConfig) *APIPollingService {
	return &APIPollingService{
		cfg: cfg,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		// We will check every 10s to see if we can establish a connection to the API when Idle retrying.
		retryDuration: 10 * time.Second,
		apiBase:       "http://localhost:8000",
		maxPollCount:  10,
	}
}

// Name give your service a log friendly name
func (s *APIPollingService) Name() string {
	return "APIPoller"
}

// Config should always return the ServiceConfig instance stored in the service struct.
// The rxdaemon manager needs this to access things like the service shutdown channel
func (s *APIPollingService) Config() *rxd.ServiceConfig {
	return s.cfg
}

// Init can be used to do any preparation that maybe doesnt belong in instance creation
// but sometime between instance creation and before you start your pre-checks to run.
func (s *APIPollingService) Init() rxd.ServiceResponse {
	// if all is well here, move to the next state or skip to RunState
	return rxd.NewResponse(nil, rxd.IdleState)
}

// Idle can be used for some pre-run checks or used to have run fallback to an idle retry state.
func (s *APIPollingService) Idle() rxd.ServiceResponse {
	timer := time.NewTimer(s.retryDuration)
	defer timer.Stop()

	for {
		select {
		case <-s.cfg.ShutdownC:
			return rxd.NewResponse(nil, rxd.StopState)
		case <-timer.C:
			_, err := s.client.Get(s.apiBase + "/api")
			if err != nil {
				s.cfg.LogInfo(fmt.Sprintf("could not reach the API Server, trying again in %.2f seconds", s.retryDuration.Seconds()))
				s.cfg.LogError(err.Error())
				// if we error, reset timer and try again...
				timer.Reset(s.retryDuration)
				continue
			}

			// if we make it here, we succeeded. The API is up, move to run stage.
			return rxd.NewResponse(nil, rxd.RunState)
		}
	}
}

// Run is where you want the main logic of your service to run
// when things have been initialized and are ready, this runs the heart of your service.
func (s *APIPollingService) Run() rxd.ServiceResponse {
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()

	s.cfg.LogInfo(fmt.Sprintf("%s has started to poll", s.Name()))
	var pollCount int
	for {
		select {
		case <-s.cfg.ShutdownC:
			return rxd.NewResponse(nil, rxd.StopState)
		case <-timer.C:
			if pollCount > s.maxPollCount {
				s.cfg.LogInfo(fmt.Sprintf("%s has reached its maximum poll count, stopping service", s.Name()))
				return rxd.NewResponse(nil, rxd.StopState)
			}

			resp, err := s.client.Get(s.apiBase + "/api")
			if err != nil {
				s.cfg.LogError(err.Error())
				// if we error, reset timer and try again...
				timer.Reset(s.retryDuration)
				continue
			}

			respBytes, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				s.cfg.LogError(err.Error())
				// we could return to new state: idle or stop or just continue
			}

			var respBody map[string]any
			err = json.Unmarshal(respBytes, &respBody)
			if err != nil {
				s.cfg.LogError(err.Error())
				// we could return to new state: idle or stop or just continue to keep trying.
			}

			s.cfg.LogInfo(fmt.Sprintf("%s received response from the API: %v", s.Name(), respBody))
			// Increment polling counter
			pollCount++

			// Retry every 30s after the first time.
			timer.Reset(30 * time.Second)
		}
	}
}

// Stop handles anything you might need to do to clean up before ending your service.
func (s *APIPollingService) Stop() rxd.ServiceResponse {
	// We must return a NewResponse, we use NoopState because it exits with no operation.
	// using StopState would try to recall Stop again.
	return rxd.NewResponse(nil, rxd.NoopState)
}

// This line is purely for error checking to ensure we are meeting the Service interface.
var _ rxd.Service = &APIPollingService{}
