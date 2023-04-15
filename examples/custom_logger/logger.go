package main

import "fmt"

type MyLogger struct{}

// Implement Debug, Info, Error to meet the logging interface.

func (ml *MyLogger) Debug(v any) {
	fmt.Printf("[DBG] %s", v)
}

func (ml *MyLogger) Info(v any) {
	fmt.Printf("[INF] %s", v)
}

func (ml *MyLogger) Error(v any) {
	fmt.Printf("[ERR] %s", v)
}
