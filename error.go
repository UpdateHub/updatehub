/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import "github.com/pkg/errors"

type UpdateHubErrorReporter interface {
	Cause() error
	IsFatal() bool
	error
}

type UpdateHubError struct {
	cause error
	fatal bool
}

func (e *UpdateHubError) Cause() error {
	return e.cause
}

func (e *UpdateHubError) IsFatal() bool {
	return e.fatal
}

func (e *UpdateHubError) Error() string {
	var err error

	if e.fatal {
		err = errors.Wrapf(e.cause, "fatal error")
	} else {
		err = errors.Wrapf(e.cause, "transient error")
	}

	return err.Error()
}

func NewFatalError(err error) UpdateHubErrorReporter {
	return &UpdateHubError{
		cause: err,
		fatal: true,
	}
}

func NewTransientError(err error) UpdateHubErrorReporter {
	return &UpdateHubError{
		cause: err,
		fatal: false,
	}
}
