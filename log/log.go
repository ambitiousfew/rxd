// Package log provides a simple logging interface with different levels of logging.
// It allows for structured logging with fields and supports different log levels.
// The package defines a Logger interface for logging messages with various levels and fields.
// It also defines a Handler interface for handling log messages.
package log

import (
	"fmt"
	"strconv"
	"strings"
)

// Handler is an interface that defines the behavior for handling log messages.
type Handler interface {
	Handle(level Level, message string, fields []Field)
}

// Logger is an interface that defines the behavior for logging messages.
type Logger interface {
	Log(level Level, message string, fields ...Field)
	With(fields ...Field) Logger
	SetLevel(level Level)
}

const (
	// LevelEmergency (0) Rarely used by user applications but import for critical services
	// examples include: when the system is unusable, system-wide outaged, situations that require immediate attention and human intervention
	LevelEmergency = iota // 0
	// LevelAlert (1) less commonly used but used in applications where immediate attention is required
	// examples include: security applications (breach detected), loss of connectivity, or failure of a key component thats leads to downtime.
	LevelAlert
	// LevelCritical (2) failure is severe enough to potentially stop the application or require immediate attention
	// examples include: unhandled exceptions that lead to application crashes, dependency failures, or data corruption issues.
	LevelCritical
	// LevelError (3) common to most user applications to indicate significant issues that prevent normal operation but do not stop the entire application
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

// Level represents the log level of a message.
// It is a uint8 type that defines different levels of logging.
type Level uint8

func (l Level) String() string {
	switch l {
	case LevelEmergency:
		return "EMERGENCY"
	case LevelAlert:
		return "ALERT"
	case LevelCritical:
		return "CRITICAL"
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
		return "INFO"
	}
}

// LevelFromString converts a string representation of a log level to a Level type.
// It supports case-insensitive matching and returns LevelInfo if the string does not match any known level.
func LevelFromString(level string) Level {
	switch strings.ToUpper(level) {
	case "EMERGENCY":
		return LevelEmergency
	case "ALERT":
		return LevelAlert
	case "CRITICAL":
		return LevelCritical
	case "ERROR":
		return LevelError
	case "WARNING":
		return LevelWarning
	case "NOTICE":
		return LevelNotice
	case "INFO":
		return LevelInfo
	case "DEBUG":
		return LevelDebug
	default:
		return LevelInfo
	}
}

// Field represents a key-value pair for structured logging.
// It is used to add additional context to log messages.
// Fields can be used to provide more information about the log message, such as error details, user IDs, or any other relevant data.
type Field struct {
	Key   string
	Value string
}

// Any creates a Field with a key and a value of any type.
// The value is converted to a string using fmt.Sprintf, which allows for flexible logging of different types.
func Any(key string, value any) Field {
	return Field{Key: key, Value: fmt.Sprintf("%v", value)}
}

// Error creates a Field with a key and an error message.
// It extracts the error message from the error object and uses it as the value of the field.
func Error(key string, err error) Field {
	return Field{Key: key, Value: err.Error()}
}

// Int creates a Field with a key and an integer value.
// It supports various integer types (int, uint, int8, uint8, int16, uint16, int32, uint32, int64, uint64)
func Int(key string, value any) Field {
	switch t := value.(type) {
	case int:
		return Field{Key: key, Value: strconv.Itoa(t)}
	case uint:
		return Field{Key: key, Value: strconv.FormatUint(uint64(t), 10)}
	case int8:
		return Field{Key: key, Value: strconv.Itoa(int(t))}
	case uint8:
		return Field{Key: key, Value: strconv.FormatUint(uint64(t), 10)}
	case int16:
		return Field{Key: key, Value: strconv.Itoa(int(t))}
	case uint16:
		return Field{Key: key, Value: strconv.FormatUint(uint64(t), 10)}
	case int32:
		return Field{Key: key, Value: strconv.Itoa(int(t))}
	case uint32:
		return Field{Key: key, Value: strconv.FormatUint(uint64(t), 10)}
	case int64:
		return Field{Key: key, Value: strconv.FormatInt(t, 10)}
	case uint64:
		return Field{Key: key, Value: strconv.FormatUint(t, 10)}
	default:
		return Field{Key: key, Value: "<unknown value type for int field>"}
	}
}

// String creates a Field with a key and a string value.
// It is used to log string values in a structured way.
// This is useful for logging messages, identifiers, or any other string data.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a Field with a key and a boolean value.
// It converts the boolean value to a string using strconv.FormatBool, which returns "true" or "false".
// This is useful for logging boolean flags or states in a structured way.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: strconv.FormatBool(value)}
}

// Float creates a Field with a key and a float value.
// It supports both float32 and float64 types, converting them to strings using strconv.FormatFloat.
func Float(key string, value any) Field {
	switch t := value.(type) {
	case float32:
		return Field{Key: key, Value: strconv.FormatFloat(float64(t), 'f', -1, 32)}
	case float64:
		return Field{Key: key, Value: strconv.FormatFloat(t, 'f', -1, 64)}
	default:
		return Field{Key: key, Value: "<unknown value type for float field>"}
	}
}
