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

// WaitingForRebootingState is the State interface implementation for the UpdateHubStateWaitingForReboot
type WaitingForRebootingState struct {
	BaseState
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

// ID returns the state id
func (state *WaitingForRebootingState) ID() UpdateHubState {
	return state.id
}

// Handle for WaitingForRebootingState tells us that an installation has
// been made and it is waiting for a reboot
func (state *WaitingForRebootingState) Handle(uh *UpdateHub) (State, bool) {
	return NewIdleState(), false
}

// UpdateMetadata is the ReportableState interface implementation
func (state *WaitingForRebootingState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}

// NewWaitingForRebootingState creates a new WaitingForRebootingState
func NewWaitingForRebootingState(apiClient *client.ApiClient, updateMetadata *metadata.UpdateMetadata) *WaitingForRebootingState {
	state := &WaitingForRebootingState{
		BaseState:      BaseState{id: UpdateHubStateWaitingForReboot},
		updateMetadata: updateMetadata,
	}

	state.apiClient = apiClient

	return state
}
