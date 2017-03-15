package main

import (
	"errors"
	"time"

	"code.ossystems.com.br/updatehub/agent/metadata"
)

// EasyFotaState holds the possible states for the agent
type EasyFotaState int

const (
	// EasyFotaStatePoll is set when the agent is in the "polling" mode
	EasyFotaStatePoll = iota
	// EasyFotaStateUpdateCheck is set when the agent is running a
	// "checkUpdate" procedure
	EasyFotaStateUpdateCheck
	// EasyFotaStateUpdateFetch is set when the agent is downloading
	// an update
	EasyFotaStateUpdateFetch
	// EasyFotaStateUpdateInstall is set when the agent is installing
	// an update
	EasyFotaStateUpdateInstall
	// EasyFotaStateInstalling is set when the agent is starting an
	// update installation
	EasyFotaStateInstalling
	// EasyFotaStateInstalled is set when the agent finished
	// installing an update
	EasyFotaStateInstalled
	// EasyFotaStateWaitingForReboot is set when the agent is waiting
	// for reboot
	EasyFotaStateWaitingForReboot
	// EasyFotaStateError is set when an error occured on the agent
	EasyFotaStateError
)

var statusNames = map[EasyFotaState]string{
	EasyFotaStatePoll:             "poll",
	EasyFotaStateUpdateCheck:      "update-check",
	EasyFotaStateUpdateFetch:      "update-fetch",
	EasyFotaStateUpdateInstall:    "update-install",
	EasyFotaStateInstalling:       "installing",
	EasyFotaStateInstalled:        "installed",
	EasyFotaStateWaitingForReboot: "waiting-for-reboot",
	EasyFotaStateError:            "error",
}

// BaseState is the state from which all others must do composition
type BaseState struct {
	id EasyFotaState
}

// ID returns the state id
func (b *BaseState) ID() EasyFotaState {
	return b.id
}

// Cancel cancels a state if it is cancellable
func (b *BaseState) Cancel(ok bool) bool {
	return ok
}

// State interface describes the necessary operations for a State
type State interface {
	ID() EasyFotaState
	Handle(*EasyFota) (State, bool) // Handle implements the behavior when the State is set
	Cancel(bool) bool
}

// StateToString converts a "EasyFotaState" to string
func StateToString(status EasyFotaState) string {
	return statusNames[status]
}

// ErrorState is the State interface implementation for the EasyFotaStateError
type ErrorState struct {
	BaseState
	cause EasyFotaErrorReporter
}

// Handle for ErrorState calls "panic" if the error is fatal or
// triggers a poll state otherwise
func (state *ErrorState) Handle(fota *EasyFota) (State, bool) {
	if state.cause.IsFatal() {
		panic(state.cause)
	}

	return NewPollState(), false
}

// NewErrorState creates a new ErrorState from a EasyFotaErrorReporter
func NewErrorState(err EasyFotaErrorReporter) State {
	if err == nil {
		err = NewFatalError(errors.New("generic error"))
	}

	return &ErrorState{
		BaseState: BaseState{id: EasyFotaStateError},
		cause:     err,
	}
}

// ReportableState interface describes the necessary operations for a State to be reportable
type ReportableState interface {
	UpdateMetadata() *metadata.UpdateMetadata
}

// PollState is the State interface implementation for the EasyFotaStatePoll
type PollState struct {
	BaseState
	CancellableState

	elapsedTime int
	extraPoll   int
	ticksCount  int
}

// ID returns the state id
func (state *PollState) ID() EasyFotaState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *PollState) Cancel(ok bool) bool {
	return state.CancellableState.Cancel(ok)
}

// Handle for PollState encapsulates the polling logic
func (state *PollState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = state

	if !fota.settings.PollingEnabled {
		return nextState, false
	}

	go func() {
		for {
			if state.ticksCount > 0 && state.ticksCount%(fota.pollInterval+state.extraPoll) == 0 {
				state.extraPoll = 0
				nextState = NewUpdateCheckState()
				break
			}

			time.Sleep(fota.timeStep)

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
		BaseState:        BaseState{id: EasyFotaStatePoll},
		CancellableState: CancellableState{cancel: make(chan bool)},
	}

	return state
}

// UpdateCheckState is the State interface implementation for the EasyFotaStateUpdateCheck
type UpdateCheckState struct {
	BaseState
}

// ID returns the state id
func (state *UpdateCheckState) ID() EasyFotaState {
	return state.id
}

// Handle for UpdateCheckState executes a CheckUpdate procedure and
// proceed to download the update if there is one. It goes back to the
// polling state otherwise.
func (state *UpdateCheckState) Handle(fota *EasyFota) (State, bool) {
	updateMetadata, extraPoll := fota.Controller.CheckUpdate(fota.settings.PollingRetries)

	// Reset polling retries in case of CheckUpdate success
	if extraPoll != -1 {
		fota.settings.PollingRetries = 0
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
	fota.settings.PollingRetries++

	return poll, false
}

// NewUpdateCheckState creates a new UpdateCheckState
func NewUpdateCheckState() *UpdateCheckState {
	state := &UpdateCheckState{
		BaseState: BaseState{id: EasyFotaStateUpdateCheck},
	}

	return state
}

// UpdateFetchState is the State interface implementation for the EasyFotaStateUpdateFetch
type UpdateFetchState struct {
	BaseState
	CancellableState
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

// ID returns the state id
func (state *UpdateFetchState) ID() EasyFotaState {
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
func (state *UpdateFetchState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = state

	if err := fota.Controller.FetchUpdate(state.updateMetadata, state.cancel); err == nil {
		return NewUpdateInstallState(), false
	}

	return nextState, false
}

// NewUpdateFetchState creates a new UpdateFetchState from a metadata.UpdateMetadata
func NewUpdateFetchState(updateMetadata *metadata.UpdateMetadata) *UpdateFetchState {
	state := &UpdateFetchState{
		BaseState:      BaseState{id: EasyFotaStateUpdateFetch},
		updateMetadata: updateMetadata,
	}

	return state
}

// UpdateInstallState is the State interface implementation for the EasyFotaStateUpdateInstall
type UpdateInstallState struct {
	BaseState
}

// ID returns the state id
func (state *UpdateInstallState) ID() EasyFotaState {
	return state.id
}

// Handle for UpdateInstallState setups the installation process
func (state *UpdateInstallState) Handle(fota *EasyFota) (State, bool) {
	var nextState State

	nextState = state

	return nextState, false
}

// NewUpdateInstallState creates a new UpdateInstallState
func NewUpdateInstallState() *UpdateInstallState {
	state := &UpdateInstallState{
		BaseState: BaseState{id: EasyFotaStateUpdateInstall},
	}

	return state
}
