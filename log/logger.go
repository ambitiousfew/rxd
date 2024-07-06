package log

type Logger interface {
	Log(level Level, message string, args ...any)
	SetLevel(level Level)
}

const (
	// LevelEmergency (0) Rarely used by user applications but import for critical services
	// examples include: when the system is unusable, system-wide outaged, situations that require immediate attention and human intervention
	LevelEmergency Level = iota // 0
	// LevelAlert (1) less commonly used but used in applications where immediate attention is required
	// examples include: security applications (breach detected), loss of connectivity, or failure of a key component thats leads to downtime.
	LevelAlert
	// LevelCritical (2) failure is severe enough to potentially stop the application or require immediate attention
	// examples include: unhandled exceptions that lead to application crashes, dependency failures, or data corruption issues.
	LevelCritical
	// ErrorLevel (3) common to most user applications to indicate significant issues that prevent normal operation but do not stop the entire application
	// examples include: failed db queries, invalid user input causing operation to fail, or network timeouts.
	LevelError
	// LevelWarning (4) indicate something unexpected happened or indicative of some problem in the near future (low disk space)
	// examples include: network request failed but will be retried, disk space is low, or a deprecated feature is being used.
	LevelWarning
	// LevelNotice (5) significant events but not indicative of a problem. More important than info but not critical.
	// examples include: user authentication, system startup, or significant configuration changes that should be logged for monitoring.
	LevelNotice
	// LevelInfo (6) general information about the application's operations
	// examples include: successful operations like starting up, shutting down, periodic health checks, or maintenance tasks.
	LevelInfo
	// LevelDebug (7) detailed information for debugging purposes, contain internal information about the application's state.
	// examples include: capturing execution paths, variable values, or other internal state information.
	LevelDebug
)

type Level uint8

func (l Level) String() string {
	switch l {
	case LevelError:
		return "ERROR"
	case LevelWarning:
		return "WARNING"
	case LevelNotice:
		return "NOTICE"
	case LevelInfo:
		return "INFO"
	case LevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}
