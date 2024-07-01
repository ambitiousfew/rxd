package rxd

// func TestDaemonAddService(t *testing.T) {
// 	serviceName := "valid-service"

// 	vs := &validDaemonService{}

// 	opts := []ServiceOption{
// 		// UsingRunPolicy(RunOncePolicy),
// 	}
// 	validSvc := NewService(serviceName, vs, opts...)

// 	dIface := NewDaemon("test-daemon")

// 	d, ok := dIface.(*daemon)
// 	if !ok {
// 		t.Errorf("could not cast Daemon interface to daemon struct")
// 	}

// 	if len(d.services) != 0 {
// 		t.Errorf("daemon should have no services added yet")
// 	}

// 	err := d.AddService(validSvc)
// 	if err != nil {
// 		t.Errorf("error adding service: %s", err)
// 	}

// 	if len(d.services) != 1 {
// 		t.Errorf("daemon should have one service added")
// 	}

// 	_, found := d.services[serviceName]
// 	if !found {
// 		t.Errorf("could not find the service '%s' in the daemon services map", serviceName)
// 	}
// }

// func TestDaemonStartWithNoServices(t *testing.T) {
// 	dIface := NewDaemon("test-daemon")

// 	d, ok := dIface.(*daemon)
// 	if !ok {
// 		t.Errorf("could not cast Daemon interface to daemon struct")
// 	}

// 	if len(d.services) != 0 {
// 		t.Errorf("daemon should not yet have any services")
// 	}

// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	err := d.Start(ctx)
// 	if err == nil {
// 		t.Errorf("daemon should not start without any services")
// 	}

// }

// func TestDaemonStartSingleService(t *testing.T) {
// 	serviceName := "valid-service"

// 	vs := &validService{}

// 	opts := []ServiceOption{
// 		// UsingRunPolicy(RunOncePolicy),
// 	}

// 	validSvc := NewService(serviceName, vs, opts...)

// 	d := NewDaemon("test-daemon")

// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	err := d.AddService(validSvc)
// 	if err != nil {
// 		t.Errorf("error adding service: %s", err)
// 	}

// 	// valid service has no blocking code so it will run all lifecycle stages and exit.
// 	err = d.Start(ctx)
// 	if err != nil {
// 		t.Errorf("expected: nil, got: %s", err)
// 	}

// }

// type validDaemonService struct{}

// func (s *validDaemonService) Init(ctx context.Context) error {
// 	return nil
// }

// func (s *validDaemonService) Idle(ctx context.Context) error {
// 	return nil
// }

// func (s *validDaemonService) Run(ctx context.Context) error {
// 	return nil
// }

// func (s *validDaemonService) Stop(ctx context.Context) error {
// 	return nil
// }
