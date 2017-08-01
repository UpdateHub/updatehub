/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"errors"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/metadata"
)

// ErrorState is the State interface implementation for the UpdateHubStateError
type ErrorState struct {
	BaseState
	cause UpdateHubErrorReporter
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

// UpdateMetadata is the ReportableState interface implementation
func (state *ErrorState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}

// Handle for ErrorState calls "panic" if the error is fatal or
// triggers a poll state otherwise
func (state *ErrorState) Handle(uh *UpdateHub) (State, bool) {
	log.Warn(state.cause)

	if state.cause.IsFatal() {
		return NewExitState(1), false
	}

	return NewIdleState(), false
}

// ToMap is for the State interface implementation
func (state *ErrorState) ToMap() map[string]interface{} {
	m := state.BaseState.ToMap()
	m["error"] = state.cause.Error()
	return m
}

// NewErrorState creates a new ErrorState from a UpdateHubErrorReporter
func NewErrorState(updateMetadata *metadata.UpdateMetadata, err UpdateHubErrorReporter) State {
	if err == nil {
		err = NewFatalError(errors.New("generic error"))
	}

	return &ErrorState{
		BaseState:      BaseState{id: UpdateHubStateError},
		cause:          err,
		updateMetadata: updateMetadata,
	}
}
