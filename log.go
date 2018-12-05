package httpr

import "fmt"

type logger interface {
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type emptyLogger struct{}

func (emptyLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf("[httpr]: "+format, args...)
}

func (emptyLogger) Infof(format string, args ...interface{}) {}

var defaultLogger logger = emptyLogger{}

func SetLogger(l logger) {
	if l == nil {
		panic("logger cannot be nil")
	}
	defaultLogger = l
}
