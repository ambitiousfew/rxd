package log

import (
	"fmt"
	"strconv"
	"strings"
)

type Handler interface {
	Handle(level Level, message string, fields []Field)
}

type Logger interface {
	Log(level Level, message string, fields ...Field)
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

type Field struct {
	Key   string
	Value string
}

func Any(key string, value any) Field {
	return Field{Key: key, Value: fmt.Sprintf("%v", value)}
}

func Error(key string, err error) Field {
	return Field{Key: key, Value: err.Error()}
}

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

func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: strconv.FormatBool(value)}
}

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
