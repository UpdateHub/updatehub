/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
)

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

// UpdateMetadata is the ReportableState interface implementation
func (state *WaitingForRebootState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}

// NewWaitingForRebootState creates a new WaitingForRebootState
func NewWaitingForRebootState(apiClient *client.ApiClient, updateMetadata *metadata.UpdateMetadata) *WaitingForRebootState {
	state := &WaitingForRebootState{
		BaseState:      BaseState{id: UpdateHubStateWaitingForReboot},
		updateMetadata: updateMetadata,
	}

	state.apiClient = apiClient

	return state
}
