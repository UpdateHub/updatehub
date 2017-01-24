package main

import "github.com/pkg/errors"

type EasyFotaErrorReporter interface {
	Cause() error
	IsFatal() bool
	error
}

type EasyFotaError struct {
	cause error
	fatal bool
}

func (e *EasyFotaError) Cause() error {
	return e.cause
}

func (e *EasyFotaError) IsFatal() bool {
	return e.fatal
}

func (e *EasyFotaError) Error() string {
	var err error

	if e.fatal {
		err = errors.Wrapf(e.cause, "fatal error")
	} else {
		err = errors.Wrapf(e.cause, "transient error")
	}

	return err.Error()
}

func NewFatalError(err error) EasyFotaErrorReporter {
	return &EasyFotaError{
		cause: err,
		fatal: true,
	}
}

func NewTransientError(err error) EasyFotaErrorReporter {
	return &EasyFotaError{
		cause: err,
		fatal: false,
	}
}
