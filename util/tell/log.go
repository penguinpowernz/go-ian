package tell

import (
	"fmt"
	"log"
	"os"
)

// log levels
const (
	DEBUG = 0
	INFO  = 1
	WARN  = 2
	ERROR = 3
	FATAL = 4
)

// Level is the current log level
var Level = 0

func init() {
	log.SetFlags(log.Ldate | log.Ltime)
}

// SetOutput sets where the log output should go
var SetOutput = log.SetOutput

// Debugf logs a Debugf message
func Debugf(msg string, args ...interface{}) {
	if Level > DEBUG {
		return
	}

	msg = fmt.Sprintf(msg, args...)
	log.Printf("DEBUG: " + msg)
}

// Infof logs a Infof message
func Infof(msg string, args ...interface{}) {
	if Level > INFO {
		return
	}

	msg = fmt.Sprintf(msg, args...)
	log.Printf("INFO: " + msg)
}

// Warnf logs a Warnf message
func Warnf(msg string, args ...interface{}) {
	if Level > WARN {
		return
	}

	msg = fmt.Sprintf(msg, args...)
	log.Printf("WARN: " + msg)
}

// IfErrorf logs an error message if there was an error
func IfErrorf(err error, msg string, args ...interface{}) {
	if err != nil {
		msg = fmt.Sprintf(msg, args...)
		Errorf(msg+": %s", err)
	}
}

// Errorf logs a Errorf message
func Errorf(msg string, args ...interface{}) {
	if Level > ERROR {
		return
	}

	msg = fmt.Sprintf(msg, args...)
	log.Printf("ERROR: " + msg)
}

// Fatalf logs a Fatalf message
func Fatalf(msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	log.Printf("FATAL: " + msg)
	os.Exit(1)
}

// IfFatalf logs a IfFatalf message
func IfFatalf(err error, msg string, args ...interface{}) {
	if err != nil {
		msg = fmt.Sprintf(msg, args...)
		Fatalf(msg+": %s", err)
	}
}

// IfEmptyFatal will log a fatal message and exit if the check is an empty string
// with the thing param describing what must not be empty
func IfEmptyFatal(check, thing string) {
	if check == "" {
		Fatalf("%s must not be empty", thing)
	}
}
