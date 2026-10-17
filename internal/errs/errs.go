// Package errs defines the lsm error type that carries a CLI exit code.
//
// Lower layers (config, service, process) return *errs.Error values and the cmd
// layer only reads the code back out to build the process exit status. Keeping
// the codes in a leaf package avoids an import cycle between cmd and the
// platform-facing internal packages.
package errs

import (
	"errors"
	"fmt"
)

// Code is a process exit code, as documented in dev.md §9.1.
type Code int

const (
	OK             Code = 0 // success
	General        Code = 1 // unclassified error
	Usage          Code = 2 // command line usage error
	NotFound       Code = 3 // service does not exist
	AlreadyRunning Code = 4 // service is already running
	NotRunning     Code = 5 // service is not running
	ConfigRead     Code = 6 // config read / parse / validation failure
	ConfigWrite    Code = 7 // config write failure
	ProcessOp      Code = 8 // process launch / stop / liveness failure
)

// Error is an error with a user-facing message and a CLI exit code.
type Error struct {
	Code Code
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e.Msg == "" {
		if e.Err != nil {
			return e.Err.Error()
		}
		return "unknown error"
	}
	return e.Msg
}

// Unwrap exposes the wrapped cause to errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.Err }

// New builds a classified error from a format string.
func New(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// Wrap builds a classified error whose message is the formatted text followed
// by the cause, so that the detail stays visible in the CLI output.
func Wrap(code Code, err error, format string, args ...any) *Error {
	msg := fmt.Sprintf(format, args...)
	if err != nil {
		msg += ": " + err.Error()
	}
	return &Error{Code: code, Msg: msg, Err: err}
}

// ExitStatus carries an exit status that lsm did not choose, namely the status
// of the command started by `lsm run`. The CLI returns it verbatim and prints
// nothing, so a foreground run is indistinguishable from running the command
// directly.
type ExitStatus struct {
	Status int
}

func (e *ExitStatus) Error() string {
	return fmt.Sprintf("exit status %d", e.Status)
}

// Classify tags err with an exit code without altering its message.
func Classify(code Code, err error) *Error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Msg: err.Error(), Err: err}
}

// CodeOf returns the exit code carried by err, or General for unclassified
// errors and OK for nil.
func CodeOf(err error) Code {
	if err == nil {
		return OK
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return General
}
