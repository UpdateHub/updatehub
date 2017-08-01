/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import "time"

// UpdateProbeState is the State interface implementation for the UpdateHubStateUpdateProbe
type UpdateProbeState struct {
	BaseState
}

// ID returns the state id
func (state *UpdateProbeState) ID() UpdateHubState {
	return state.id
}

// Handle for UpdateProbeState executes a ProbeUpdate procedure and
// proceed to download the update if there is one. It goes back to the
// polling state otherwise.
func (state *UpdateProbeState) Handle(uh *UpdateHub) (State, bool) {
	updateMetadata, extraPoll := uh.Controller.ProbeUpdate(uh.Settings.PollingRetries)

	// Reset polling retries in case of ProbeUpdate success
	if extraPoll != -1 {
		uh.Settings.PollingRetries = 0
	}

	uh.Settings.LastPoll = time.Now()
	uh.Settings.ExtraPollingInterval = 0

	if updateMetadata != nil {
		packageUID := updateMetadata.PackageUID()
		if packageUID == uh.lastInstalledPackageUID {
			return NewWaitingForRebootState(updateMetadata), false
		}

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

	// Increment the number of polling retries in case of ProbeUpdate failure
	uh.Settings.PollingRetries++

	return NewIdleState(), false
}

// NewUpdateProbeState creates a new UpdateProbeState
func NewUpdateProbeState() *UpdateProbeState {
	state := &UpdateProbeState{
		BaseState: BaseState{id: UpdateHubStateUpdateProbe},
	}

	return state
}
