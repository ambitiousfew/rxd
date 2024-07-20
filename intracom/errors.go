package intracom

// Error is a custom error type for the intracom package.
type Error string

const (
	ErrInvalidIntracomNil    = Error("invalid intracom, cannot be nil")
	ErrIntracomClosed        = Error("intracom is closed")
	ErrTopicNotFound         = Error("topic not found")
	ErrTopicAlreadyExists    = Error("topic already exists")
	ErrTopicDoesNotExist     = Error("topic does not exist")
	ErrInvalidTopicType      = Error("topic exists but with a different type")
	ErrTopicClosed           = Error("topic is closed")
	ErrConsumerAlreadyExists = Error("consumer already exists")
	ErrMaxTimeoutReached     = Error("max timeout reached")
)

// Action is the action that was attempted when an error occurred.
type Action string

const (
	ActionClosingIntracom      = Action("closing intracom")
	ActionClosingTopic         = Action("closing topic")
	ActionRemovingTopic        = Action("removing topic")
	ActionCreatingTopic        = Action("creating topic")
	ActionRemovingSubscription = Action("removing subscription")
	ActionCreatingSubscription = Action("creating subscription")
)

func (e Error) Error() string {
	return string(e)
}

type ErrIntracom struct {
	Action Action
	Err    error
}

func (e ErrIntracom) Error() string {
	return "error " + string(e.Action) + " reason: " + e.Err.Error()
}

type ErrSubscribe struct {
	Topic    string
	Consumer string
	Action   Action
	Err      error
}

func (e ErrSubscribe) Error() string {
	return "error " + string(e.Action) + " to topic '" + e.Topic + "' with consumer '" + e.Consumer + "' reason: " + e.Err.Error()
}

type ErrTopic struct {
	Topic  string
	Action Action
	Err    error
}

func (e ErrTopic) Error() string {
	return "error " + string(e.Action) + " removing topic '" + e.Topic + "'" + " reason: " + e.Err.Error()

}
