/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"github.com/UpdateHub/updatehub/metadata"
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
	// UpdateHubStateUpdateProbe is set when the agent is running a
	// "probeUpdate" procedure
	UpdateHubStateUpdateProbe
	// UpdateHubStateDownloading is set when the agent is downloading
	// an update
	UpdateHubStateDownloading
	// UpdateHubStateDownloaded is set when the agent finished
	// downloading an update
	UpdateHubStateDownloaded
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
	UpdateHubStateUpdateProbe:      "update-probe",
	UpdateHubStateDownloading:      "downloading",
	UpdateHubStateDownloaded:       "downloaded",
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

// ReportableState interface describes the necessary operations for a State to be reportable
type ReportableState interface {
	UpdateMetadata() *metadata.UpdateMetadata
}
