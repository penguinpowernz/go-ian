package tell

import (
	"fmt"
	"log"
	"os"

	"github.com/penguinpowernz/go-ian/util/colour"
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
	// the message is already formatted, so Print rather than Printf: a value
	// containing a % must not be taken as a format verb for a second pass
	log.Print("DEBUG: " + msg)
}

// Infof logs a Infof message
func Infof(msg string, args ...interface{}) {
	if Level > INFO {
		return
	}

	msg = fmt.Sprintf(msg, args...)
	log.Print("INFO: " + msg)
}

// Warnf logs a Warnf message
func Warnf(msg string, args ...interface{}) {
	if Level > WARN {
		return
	}

	msg = fmt.Sprintf(msg, args...)
	log.Print("WARN: " + msg)
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
	log.Print("ERROR: " + msg)
}

// Fatalf logs a Fatalf message
func Fatalf(msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	// a fatal message is the last thing the process says, so it is painted
	// red when the log is going to a terminal
	log.Print(colour.For(os.Stderr).P(colour.Red, "FATAL: "+msg))
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
