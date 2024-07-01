package rxd

type ServiceError struct {
	Name  string
	State string
	Err   error
}

func (se ServiceError) Error() string {
	return "service error during '" + se.State + "': " + se.Name + ": " + se.Err.Error()
}
