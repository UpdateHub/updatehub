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
	"fmt"
	"sync"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/spf13/afero"
)

// UpdateHubState holds the possible states for the agent
type UpdateHubState int

const (
	// UpdateHubDummyState is a dummy state
	UpdateHubDummyState = iota
	// UpdateHubStateIdle is set when the agent is in the "idle" mode
	UpdateHubStateIdle
	// UpdateHubStatePoll is set when the agent is in the "polling" mode
	UpdateHubStatePoll
	// UpdateHubStateUpdateCheck is set when the agent is running a
	// "checkUpdate" procedure
	UpdateHubStateUpdateCheck
	// UpdateHubStateDownloading is set when the agent is downloading
	// an update
	UpdateHubStateDownloading
	// UpdateHubStateInstalling is set when the agent is starting an
	// update installation
	UpdateHubStateInstalling
	// UpdateHubStateInstalled is set when the agent finished
	// installing an update
	UpdateHubStateInstalled
	// UpdateHubStateWaitingForReboot is set when the agent is waiting
	// for reboot
	UpdateHubStateWaitingForReboot
	// UpdateHubStateExit is set when the daemon is about to quit
	UpdateHubStateExit
	// UpdateHubStateError is set when an error occured on the agent
	UpdateHubStateError
)

var statusNames = map[UpdateHubState]string{
	UpdateHubStateIdle:             "idle",
	UpdateHubStatePoll:             "poll",
	UpdateHubStateUpdateCheck:      "update-check",
	UpdateHubStateDownloading:      "downloading",
	UpdateHubStateInstalling:       "installing",
	UpdateHubStateInstalled:        "installed",
	UpdateHubStateWaitingForReboot: "waiting-for-reboot",
	UpdateHubStateExit:             "exit",
	UpdateHubStateError:            "error",
}

// ProgressTracker will define which way the progress is kept
type ProgressTracker interface {
	SetProgress(progress int)
	GetProgress() int
}

// ProgressTrackerImpl is for the ProgressTracker interface implementation
type ProgressTrackerImpl struct {
	progress int
}

// SetProgress is for the ProgressTracker interface implementation
func (pti *ProgressTrackerImpl) SetProgress(progress int) {
	pti.progress = progress
}

// GetProgress is for the ProgressTracker interface implementation
func (pti *ProgressTrackerImpl) GetProgress() int {
	return pti.progress
}

// BaseState is the state from which all others must do composition
type BaseState struct {
	id UpdateHubState
}

// ToMap is for the State interface implementation
func (state *BaseState) ToMap() map[string]interface{} {
	m := map[string]interface{}{}
	m["status"] = StateToString(state.ID())
	return m
}

// ID returns the state id
func (b *BaseState) ID() UpdateHubState {
	return b.id
}

// Cancel cancels a state if it is cancellable
func (b *BaseState) Cancel(ok bool, nextState State) bool {
	return ok
}

// State interface describes the necessary operations for a State
type State interface {
	ID() UpdateHubState
	Handle(*UpdateHub) (State, bool) // Handle implements the behavior when the State is set
	Cancel(bool, State) bool
	ToMap() map[string]interface{}
}

// StateToString converts a "UpdateHubState" to string
func StateToString(status UpdateHubState) string {
	return statusNames[status]
}

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

// ReportableState interface describes the necessary operations for a State to be reportable
type ReportableState interface {
	UpdateMetadata() *metadata.UpdateMetadata
}

// IdleState is the State interface implementation for the UpdateHubStateIdle
type IdleState struct {
	BaseState
	CancellableState
	ReportableState
}

// ID returns the state id
func (state *IdleState) ID() UpdateHubState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *IdleState) Cancel(ok bool, nextState State) bool {
	return state.CancellableState.Cancel(ok, nextState)
}

// Handle for IdleState
func (state *IdleState) Handle(uh *UpdateHub) (State, bool) {
	if !uh.Settings.PollingEnabled {
		state.Wait()
		return state.NextState(), false
	}

	now := time.Now()

	if uh.Settings.ExtraPollingInterval > 0 {
		extraPollTime := uh.Settings.LastPoll.Add(uh.Settings.ExtraPollingInterval)

		if extraPollTime.Before(now) {
			return NewUpdateCheckState(), false
		}
	}

	return NewPollState(uh.Settings.PollingInterval), false
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

	interval   time.Duration
	ticksCount int64
}

// ID returns the state id
func (state *PollState) ID() UpdateHubState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *PollState) Cancel(ok bool, nextState State) bool {
	return state.CancellableState.Cancel(ok, nextState)
}

// Handle for PollState encapsulates the polling logic
func (state *PollState) Handle(uh *UpdateHub) (State, bool) {
	var nextState State

	nextState = state

	if state.interval <= 0 {
		err := fmt.Errorf("Can't handle polling with invalid interval. It must be greater than zero")
		return NewErrorState(nil, NewTransientError(err)), false
	}

	go func() {
		ticks := state.ticksCount

	polling:
		for {
			ticker := time.NewTicker(uh.TimeStep)

			defer ticker.Stop()

			select {
			case <-ticker.C:
				ticks++

				if ticks > 0 && ticks%int64(state.interval/uh.TimeStep) == 0 {
					nextState = NewUpdateCheckState()
					break polling
				}
			case <-state.cancel:
				break
			}
		}

		state.Cancel(true, nextState)

		state.ticksCount = ticks
	}()

	state.Wait()

	// state cancelled
	if state.NextState() != nil {
		return state.NextState(), true
	}

	return nextState, false
}

// NewPollState creates a new PollState
func NewPollState(pollingInterval time.Duration) *PollState {
	state := &PollState{
		BaseState:        BaseState{id: UpdateHubStatePoll},
		CancellableState: CancellableState{cancel: make(chan bool)},
	}

	state.interval = pollingInterval

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
	updateMetadata, extraPoll := uh.Controller.CheckUpdate(uh.Settings.PollingRetries)

	// Reset polling retries in case of CheckUpdate success
	if extraPoll != -1 {
		uh.Settings.PollingRetries = 0
	}

	uh.Settings.LastPoll = time.Now()
	uh.Settings.ExtraPollingInterval = 0

	if updateMetadata != nil {
		return NewDownloadingState(updateMetadata, &ProgressTrackerImpl{}), false
	}

	if extraPoll > 0 {
		now := time.Now()
		nextPoll := time.Unix(uh.Settings.FirstPoll.Unix(), 0)
		extraPollTime := now.Add(extraPoll)

		for nextPoll.Before(now) {
			nextPoll = nextPoll.Add(uh.Settings.PollingInterval)
		}

		if extraPollTime.Before(nextPoll) {
			uh.Settings.ExtraPollingInterval = extraPoll

			poll := NewPollState(uh.Settings.PollingInterval)
			poll.interval = extraPoll

			return poll, false
		}
	}

	// Increment the number of polling retries in case of CheckUpdate failure
	uh.Settings.PollingRetries++

	return NewIdleState(), false
}

// NewUpdateCheckState creates a new UpdateCheckState
func NewUpdateCheckState() *UpdateCheckState {
	state := &UpdateCheckState{
		BaseState: BaseState{id: UpdateHubStateUpdateCheck},
	}

	return state
}

// DownloadingState is the State interface implementation for the UpdateHubStateDownloading
type DownloadingState struct {
	BaseState
	CancellableState
	ReportableState
	ProgressTracker

	updateMetadata *metadata.UpdateMetadata
}

// ID returns the state id
func (state *DownloadingState) ID() UpdateHubState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *DownloadingState) Cancel(ok bool, nextState State) bool {
	state.CancellableState.Cancel(ok, nextState)
	return ok
}

// UpdateMetadata is the ReportableState interface implementation
func (state *DownloadingState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}

// Handle for DownloadingState starts the objects downloads. It goes
// to the installing state if successfull. It goes back to the error
// state otherwise.
func (state *DownloadingState) Handle(uh *UpdateHub) (State, bool) {
	var err error
	var nextState State

	nextState = state

	progressChan := make(chan int, 10)

	m := sync.Mutex{}
	m.Lock()

	go func() {
		m.Lock()
		defer m.Unlock()

		err = uh.Controller.FetchUpdate(state.updateMetadata, state.cancel, progressChan)
		close(progressChan)
	}()

	m.Unlock()
	for p := range progressChan {
		state.ProgressTracker.SetProgress(p)
	}

	if err != nil {
		nextState = NewErrorState(state.updateMetadata, NewTransientError(err))
	} else {
		nextState = NewInstallingState(state.updateMetadata, &ProgressTrackerImpl{}, uh.Store)
	}

	// state cancelled
	if state.NextState() != nil {
		return state.NextState(), true
	}

	return nextState, false
}

// ToMap is for the State interface implementation
func (state *DownloadingState) ToMap() map[string]interface{} {
	m := state.BaseState.ToMap()
	m["progress"] = state.ProgressTracker.GetProgress()
	return m
}

// NewDownloadingState creates a new DownloadingState from a metadata.UpdateMetadata
func NewDownloadingState(updateMetadata *metadata.UpdateMetadata, pti ProgressTracker) *DownloadingState {
	state := &DownloadingState{
		BaseState:       BaseState{id: UpdateHubStateDownloading},
		updateMetadata:  updateMetadata,
		ProgressTracker: pti,
	}

	return state
}

// InstallingState is the State interface implementation for the UpdateHubStateInstalling
type InstallingState struct {
	BaseState
	ReportableState
	ProgressTracker
	FileSystemBackend afero.Fs
	updateMetadata    *metadata.UpdateMetadata
}

// ID returns the state id
func (state *InstallingState) ID() UpdateHubState {
	return state.id
}

// Handle for InstallingState implements the installation process itself
func (state *InstallingState) Handle(uh *UpdateHub) (State, bool) {
	packageUID := state.updateMetadata.PackageUID()
	if packageUID == uh.lastInstalledPackageUID {
		return NewWaitingForRebootState(state.updateMetadata), false
	}

	// register the packageUID at the start so it won't redo the
	// operations in case of an install error occurs
	uh.lastInstalledPackageUID = packageUID

	var err error

	progressChan := make(chan int, 10)

	m := sync.Mutex{}
	m.Lock()

	go func() {
		m.Lock()
		defer m.Unlock()

		err = uh.Controller.InstallUpdate(state.updateMetadata, progressChan)
		close(progressChan)
	}()

	m.Unlock()
	for p := range progressChan {
		state.ProgressTracker.SetProgress(p)
	}

	if err != nil {
		return NewErrorState(state.updateMetadata, NewTransientError(err)), false
	}

	return NewInstalledState(state.updateMetadata), false
}

// ToMap is for the State interface implementation
func (state *InstallingState) ToMap() map[string]interface{} {
	m := state.BaseState.ToMap()
	m["progress"] = state.ProgressTracker.GetProgress()
	return m
}

// UpdateMetadata is the ReportableState interface implementation
func (state *InstallingState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}

// NewInstallingState creates a new InstallingState
func NewInstallingState(
	updateMetadata *metadata.UpdateMetadata,
	pti ProgressTracker,
	fsb afero.Fs) *InstallingState {
	state := &InstallingState{
		BaseState:         BaseState{id: UpdateHubStateInstalling},
		updateMetadata:    updateMetadata,
		FileSystemBackend: fsb,
		ProgressTracker:   pti,
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

// ExitState is the final state of the state machine
type ExitState struct {
	BaseState

	exitCode int
}

// NewExitState creates a new ExitState
func NewExitState(exitCode int) *ExitState {
	return &ExitState{
		BaseState: BaseState{id: UpdateHubStateExit},
		exitCode:  exitCode,
	}
}

// Handle for ExitState
func (state *ExitState) Handle(uh *UpdateHub) (State, bool) {
	panic("ExitState handler should not be called")
}
