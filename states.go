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
	// UpdateHubStateIdle is set when the agent is in the "idle" mode
	UpdateHubStateIdle = iota
	// UpdateHubStatePoll is set when the agent is in the "polling" mode
	UpdateHubStatePoll
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
	UpdateHubStateIdle:             "idle",
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

	return NewIdleState(), false
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

// IdleState is the State interface implementation for the UpdateHubStateIdle
type IdleState struct {
	BaseState
	CancellableState
}

// ID returns the state id
func (state *IdleState) ID() UpdateHubState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *IdleState) Cancel(ok bool) bool {
	return state.CancellableState.Cancel(ok)
}

// Handle for IdleState
func (state *IdleState) Handle(uh *UpdateHub) (State, bool) {
	if !uh.settings.PollingEnabled {
		state.Wait()
		return state, false
	}

	return uh.NewPollState(), false
}

// NewIdleState creates a new IdleState
func NewIdleState() *IdleState {
	state := &IdleState{
		BaseState:        BaseState{id: UpdateHubStateIdle},
		CancellableState: CancellableState{cancel: make(chan bool)},
	}

	return state
}

// PollState is the State interface implementation for the UpdateHubStatePoll
type PollState struct {
	BaseState
	CancellableState

	interval   int
	ticksCount int
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
		for {
			if state.ticksCount > 0 && state.ticksCount%(state.interval/int(uh.timeStep)) == 0 {
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
		interval:         int(time.Second),
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

	uh.settings.LastPoll = int(time.Now().Unix())

	if updateMetadata != nil {
		return NewUpdateFetchState(updateMetadata), false
	}

	if extraPoll > 0 {
		poll := uh.NewPollState()
		poll.interval = extraPoll

		return poll, false
	}

	// Increment the number of polling retries in case of CheckUpdate failure
	uh.settings.PollingRetries++

	return NewIdleState(), false
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
		return NewUpdateInstallState(state.updateMetadata), false
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
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

// ID returns the state id
func (state *UpdateInstallState) ID() UpdateHubState {
	return state.id
}

// Handle for UpdateInstallState setups the installation process
func (state *UpdateInstallState) Handle(uh *UpdateHub) (State, bool) {
	packageUID, err := state.updateMetadata.Checksum()
	if err != nil {
		return NewErrorState(NewTransientError(err)), false
	}

	if packageUID == uh.lastInstalledPackageUID {
		return NewWaitingForRebootState(state.updateMetadata), false
	}

	// register the packageUID at the start so it won't redo the
	// operations in case of an install error occurs
	uh.lastInstalledPackageUID = packageUID

	// FIXME: check supported hardware

	return NewInstallingState(state.updateMetadata), false
}

// NewUpdateInstallState creates a new UpdateInstallState
func NewUpdateInstallState(updateMetadata *metadata.UpdateMetadata) *UpdateInstallState {
	state := &UpdateInstallState{
		BaseState:      BaseState{id: UpdateHubStateUpdateInstall},
		updateMetadata: updateMetadata,
	}

	return state
}

// InstallingState is the State interface implementation for the UpdateHubStateInstalling
type InstallingState struct {
	BaseState
	CancellableState
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

// ID returns the state id
func (state *InstallingState) ID() UpdateHubState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *InstallingState) Cancel(ok bool) bool {
	return state.CancellableState.Cancel(ok)
}

// Handle for InstallingState implements the installation process itself
func (state *InstallingState) Handle(uh *UpdateHub) (State, bool) {
	// FIXME: not yet implemented
	return NewInstalledState(state.updateMetadata), false
}

// NewInstallingState creates a new InstallingState
func NewInstallingState(updateMetadata *metadata.UpdateMetadata) *InstallingState {
	state := &InstallingState{
		BaseState:      BaseState{id: UpdateHubStateInstalling},
		updateMetadata: updateMetadata,
	}

	return state
}

// WaitingForRebootState is the State interface implementation for the UpdateHubStateWaitingForReboot
type WaitingForRebootState struct {
	BaseState
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

// ID returns the state id
func (state *WaitingForRebootState) ID() UpdateHubState {
	return state.id
}

// Handle for WaitingForRebootState tells us that an installation has
// been made and it is waiting for a reboot
func (state *WaitingForRebootState) Handle(uh *UpdateHub) (State, bool) {
	// FIXME: not yet implemented
	return NewIdleState(), false
}

// NewWaitingForRebootState creates a new WaitingForRebootState
func NewWaitingForRebootState(updateMetadata *metadata.UpdateMetadata) *WaitingForRebootState {
	state := &WaitingForRebootState{
		BaseState:      BaseState{id: UpdateHubStateWaitingForReboot},
		updateMetadata: updateMetadata,
	}

	return state
}

// InstalledState is the State interface implementation for the UpdateHubStateInstalled
type InstalledState struct {
	BaseState
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

// ID returns the state id
func (state *InstalledState) ID() UpdateHubState {
	return state.id
}

// Handle for InstalledState implements the installation process itself
func (state *InstalledState) Handle(uh *UpdateHub) (State, bool) {
	// FIXME: not yet implemented
	return NewIdleState(), false
}

// NewInstalledState creates a new InstalledState
func NewInstalledState(updateMetadata *metadata.UpdateMetadata) *InstalledState {
	state := &InstalledState{
		BaseState:      BaseState{id: UpdateHubStateInstalled},
		updateMetadata: updateMetadata,
	}

	return state
}
