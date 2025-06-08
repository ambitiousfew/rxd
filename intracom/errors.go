package intracom

// Error is a custom error type for the intracom package.
type Error string

const (
	// ErrInvalidIntracomNil represents an error when an intracom is nil.
	ErrInvalidIntracomNil = Error("invalid intracom, cannot be nil")
	// ErrIntracomClosed represents an error when an intracom is closed.
	ErrIntracomClosed = Error("intracom is closed")
	// ErrTopicNotFound represents an error when a topic is not found.
	ErrTopicNotFound = Error("topic not found")
	// ErrTopicAlreadyExists represents an error when a topic already exists.
	ErrTopicAlreadyExists = Error("topic already exists")
	// ErrTopicDoesNotExist represents an error when a topic does not exist.
	ErrTopicDoesNotExist = Error("topic does not exist")
	// ErrInvalidTopicType represents an error when a topic exists but with a different type.
	ErrInvalidTopicType = Error("topic exists but with a different type")
	// ErrTopicClosed represents an error when a topic is closed.
	ErrTopicClosed = Error("topic is closed")
	// ErrConsumerAlreadyExists represents an error when a consumer already exists for a topic.
	ErrConsumerAlreadyExists = Error("consumer already exists")
	// ErrMaxTimeoutReached represents an error when the maximum timeout is reached.
	ErrMaxTimeoutReached = Error("max timeout reached")
)

// Action is the action that was attempted when an error occurred.
type Action string

const (
	//ActionClosingIntracom represents the action of closing an intracom.
	ActionClosingIntracom = Action("closing intracom")
	// ActionClosingTopic represents the action of closing a topic.
	ActionClosingTopic = Action("closing topic")
	// ActionRemovingTopic represents the action of removing a topic.
	ActionRemovingTopic = Action("removing topic")
	// ActionCreatingTopic represents the action of creating a topic.
	ActionCreatingTopic = Action("creating topic")
	// ActionRemovingSubscription represents the action of removing a subscription.
	ActionRemovingSubscription = Action("removing subscription")
	// ActionCreatingSubscription represents the action of creating a subscription.
	ActionCreatingSubscription = Action("creating subscription")
)

func (e Error) Error() string {
	return string(e)
}

// ErrIntracom is a custom error type for the intracom package.
type ErrIntracom struct {
	Action Action
	Err    error
}

func (e ErrIntracom) Error() string {
	return "error " + string(e.Action) + " reason: " + e.Err.Error()
}

// ErrSubscribe is a custom error type for subscription errors in the intracom package.
type ErrSubscribe struct {
	Topic    string
	Consumer string
	Action   Action
	Err      error
}

func (e ErrSubscribe) Error() string {
	return "error " + string(e.Action) + " to topic '" + e.Topic + "' with consumer '" + e.Consumer + "' reason: " + e.Err.Error()
}

// ErrTopic is a custom error type for topic-related errors in the intracom package.
type ErrTopic struct {
	Topic  string
	Action Action
	Err    error
}

func (e ErrTopic) Error() string {
	return "error " + string(e.Action) + " removing topic '" + e.Topic + "'" + " reason: " + e.Err.Error()

}
