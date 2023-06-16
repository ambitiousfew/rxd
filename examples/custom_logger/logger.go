package main

import "fmt"

type MyLogger struct{}

// Implement Debug, Info, Error to meet the logging interface.

func (ml *MyLogger) Debug(v ...any) {
	fmt.Printf("[DBG] %s", v...)
}

func (ml *MyLogger) Debugf(format string, v ...any) {
	fmt.Printf("[DBG] %s", fmt.Sprintf(format, v...))
}

func (ml *MyLogger) Info(v ...any) {
	fmt.Printf("[INF] %s", v...)
}

func (ml *MyLogger) Infof(format string, v ...any) {
	fmt.Printf("[INF] %s", fmt.Sprintf(format, v...))
}

func (ml *MyLogger) Warn(v ...any) {
	fmt.Printf("[WRN] %s", v...)
}

func (ml *MyLogger) Warnf(format string, v ...any) {
	fmt.Printf("[WRN] %s", v...)
}

func (ml *MyLogger) Error(v ...any) {
	fmt.Printf("[ERR] %s", v...)
}

func (ml *MyLogger) Errorf(format string, v ...any) {
	fmt.Printf("[ERR] %s", fmt.Sprintf(format, v...))
}
