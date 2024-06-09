package rxd

// func TestNewServiceOpts(t *testing.T) {
// 	// defaultOpts contains default runPolicy of RunUntilStoppedPolicy
// 	defaultOpts := NewServiceOpts()
// 	want := RunUntilStoppedPolicy
// 	got := defaultOpts.runPolicy

// 	if defaultOpts.runPolicy != RunUntilStoppedPolicy {
// 		t.Errorf("NewServiceOpts did not default to %s: got %s", want, got)
// 	}

// }

// func TestUsingRunPolicy(t *testing.T) {
// 	// defaultOpts contains default runPolicy of RunUntilStoppedPolicy
// 	defaultOpts := NewServiceOpts()
// 	origin := defaultOpts.runPolicy

// 	want := RunOncePolicy
// 	// test UsingRunPolicy changes that option.
// 	applyOptionFunc := UsingRunPolicy(want)
// 	applyOptionFunc(defaultOpts)
// 	got := defaultOpts.runPolicy

// 	if defaultOpts.runPolicy != want {
// 		t.Errorf("policy was not changed from %s to %s, got: %s", origin, want, got)
// 	}

// }
