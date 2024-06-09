package rxd

import "log"

type Logger interface {
	Info(format string, kvs ...interface{})
	Debug(format string, kvs ...interface{})
	Error(format string, kvs ...interface{})
}

type DefaultLogger struct{}

func (l *DefaultLogger) Info(msg string, kvs ...interface{}) {
	log.Printf("INFO: %s %v", msg, kvs)
}

func (l *DefaultLogger) Debug(msg string, kvs ...interface{}) {
	log.Printf("DEBUG: %s %v", msg, kvs)
}

func (l *DefaultLogger) Error(msg string, kvs ...interface{}) {
	log.Printf("ERROR: %s %v", msg, kvs)
}
