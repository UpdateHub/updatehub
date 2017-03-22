/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"errors"
	"time"

	"github.com/UpdateHub/updatehub/metadata"
)

// UpdateHubState holds the possible states for the agent
type UpdateHubState int

const (
	// UpdateHubStatePoll is set when the agent is in the "polling" mode
	UpdateHubStatePoll = iota
	// UpdateHubStateUpdateCheck is set when the agent is running a
	// "checkUpdate" procedure
	UpdateHubStateUpdateCheck
	// UpdateHubStateUpdateFetch is set when the agent is downloading
	// an update
	UpdateHubStateUpdateFetch
	// UpdateHubStateUpdateInstall is set when the agent is installing
	// an update
	UpdateHubStateUpdateInstall
	// UpdateHubStateInstalling is set when the agent is starting an
	// update installation
	UpdateHubStateInstalling
	// UpdateHubStateInstalled is set when the agent finished
	// installing an update
	UpdateHubStateInstalled
	// UpdateHubStateWaitingForReboot is set when the agent is waiting
	// for reboot
	UpdateHubStateWaitingForReboot
	// UpdateHubStateError is set when an error occured on the agent
	UpdateHubStateError
)

var statusNames = map[UpdateHubState]string{
	UpdateHubStatePoll:             "poll",
	UpdateHubStateUpdateCheck:      "update-check",
	UpdateHubStateUpdateFetch:      "update-fetch",
	UpdateHubStateUpdateInstall:    "update-install",
	UpdateHubStateInstalling:       "installing",
	UpdateHubStateInstalled:        "installed",
	UpdateHubStateWaitingForReboot: "waiting-for-reboot",
	UpdateHubStateError:            "error",
}

// BaseState is the state from which all others must do composition
type BaseState struct {
	id UpdateHubState
}

// ID returns the state id
func (b *BaseState) ID() UpdateHubState {
	return b.id
}

// Cancel cancels a state if it is cancellable
func (b *BaseState) Cancel(ok bool) bool {
	return ok
}

// State interface describes the necessary operations for a State
type State interface {
	ID() UpdateHubState
	Handle(*UpdateHub) (State, bool) // Handle implements the behavior when the State is set
	Cancel(bool) bool
}

// StateToString converts a "UpdateHubState" to string
func StateToString(status UpdateHubState) string {
	return statusNames[status]
}

// ErrorState is the State interface implementation for the UpdateHubStateError
type ErrorState struct {
	BaseState
	cause UpdateHubErrorReporter
}

// Handle for ErrorState calls "panic" if the error is fatal or
// triggers a poll state otherwise
func (state *ErrorState) Handle(uh *UpdateHub) (State, bool) {
	if state.cause.IsFatal() {
		panic(state.cause)
	}

	return NewPollState(), false
}

// NewErrorState creates a new ErrorState from a UpdateHubErrorReporter
func NewErrorState(err UpdateHubErrorReporter) State {
	if err == nil {
		err = NewFatalError(errors.New("generic error"))
	}

	return &ErrorState{
		BaseState: BaseState{id: UpdateHubStateError},
		cause:     err,
	}
}

// ReportableState interface describes the necessary operations for a State to be reportable
type ReportableState interface {
	UpdateMetadata() *metadata.UpdateMetadata
}

// PollState is the State interface implementation for the UpdateHubStatePoll
type PollState struct {
	BaseState
	CancellableState

	elapsedTime int
	extraPoll   int
	ticksCount  int
}

// ID returns the state id
func (state *PollState) ID() UpdateHubState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *PollState) Cancel(ok bool) bool {
	return state.CancellableState.Cancel(ok)
}

// Handle for PollState encapsulates the polling logic
func (state *PollState) Handle(uh *UpdateHub) (State, bool) {
	var nextState State

	nextState = state

	if !uh.settings.PollingEnabled {
		return nextState, false
	}

	go func() {
		pollingInterval := uh.settings.PollingInterval

		if uh.settings.FirstPoll == 0 {
			uh.settings.FirstPoll = int(time.Now().Unix()) + uh.pollingIntervalSpan
			pollingInterval = 1
		}

		for {
			shouldPoll := int(time.Now().Unix()) > uh.settings.FirstPoll

			if shouldPoll && state.ticksCount > 0 && state.ticksCount%(pollingInterval+state.extraPoll) == 0 {
				state.extraPoll = 0
				nextState = NewUpdateCheckState()
				break
			}

			time.Sleep(uh.timeStep)

			state.ticksCount++
		}

		state.Cancel(true)
	}()

	state.Wait()

	return nextState, false
}

// NewPollState creates a new PollState
func NewPollState() *PollState {
	state := &PollState{
		BaseState:        BaseState{id: UpdateHubStatePoll},
		CancellableState: CancellableState{cancel: make(chan bool)},
	}

	return state
}

// UpdateCheckState is the State interface implementation for the UpdateHubStateUpdateCheck
type UpdateCheckState struct {
	BaseState
}

// ID returns the state id
func (state *UpdateCheckState) ID() UpdateHubState {
	return state.id
}

// Handle for UpdateCheckState executes a CheckUpdate procedure and
// proceed to download the update if there is one. It goes back to the
// polling state otherwise.
func (state *UpdateCheckState) Handle(uh *UpdateHub) (State, bool) {
	updateMetadata, extraPoll := uh.Controller.CheckUpdate(uh.settings.PollingRetries)

	// Reset polling retries in case of CheckUpdate success
	if extraPoll != -1 {
		uh.settings.PollingRetries = 0
	}

	if updateMetadata != nil {
		return NewUpdateFetchState(updateMetadata), false
	}

	poll := NewPollState()

	if extraPoll > 0 {
		poll.extraPoll = extraPoll

		return poll, false
	}

	// Increment the number of polling retries in case of CheckUpdate failure
	uh.settings.PollingRetries++

	return poll, false
}

// NewUpdateCheckState creates a new UpdateCheckState
func NewUpdateCheckState() *UpdateCheckState {
	state := &UpdateCheckState{
		BaseState: BaseState{id: UpdateHubStateUpdateCheck},
	}

	return state
}

// UpdateFetchState is the State interface implementation for the UpdateHubStateUpdateFetch
type UpdateFetchState struct {
	BaseState
	CancellableState
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

// ID returns the state id
func (state *UpdateFetchState) ID() UpdateHubState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *UpdateFetchState) Cancel(ok bool) bool {
	state.CancellableState.Cancel(ok)
	return ok
}

// UpdateMetadata is the ReportableState interface implementation
func (state *UpdateFetchState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}

// Handle for UpdateCheckState executes a CheckUpdate procedure and
// proceed to download the update if there is one. It goes back to the
// polling state otherwise.
func (state *UpdateFetchState) Handle(uh *UpdateHub) (State, bool) {
	var nextState State

	nextState = state

	if err := uh.Controller.FetchUpdate(state.updateMetadata, state.cancel); err == nil {
		return NewUpdateInstallState(), false
	}

	return nextState, false
}

// NewUpdateFetchState creates a new UpdateFetchState from a metadata.UpdateMetadata
func NewUpdateFetchState(updateMetadata *metadata.UpdateMetadata) *UpdateFetchState {
	state := &UpdateFetchState{
		BaseState:      BaseState{id: UpdateHubStateUpdateFetch},
		updateMetadata: updateMetadata,
	}

	return state
}

// UpdateInstallState is the State interface implementation for the UpdateHubStateUpdateInstall
type UpdateInstallState struct {
	BaseState
}

// ID returns the state id
func (state *UpdateInstallState) ID() UpdateHubState {
	return state.id
}

// Handle for UpdateInstallState setups the installation process
func (state *UpdateInstallState) Handle(uh *UpdateHub) (State, bool) {
	var nextState State

	nextState = state

	return nextState, false
}

// NewUpdateInstallState creates a new UpdateInstallState
func NewUpdateInstallState() *UpdateInstallState {
	state := &UpdateInstallState{
		BaseState: BaseState{id: UpdateHubStateUpdateInstall},
	}

	return state
}
