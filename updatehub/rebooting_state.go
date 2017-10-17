/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"github.com/updatehub/updatehub/client"
	"github.com/updatehub/updatehub/metadata"
)

// RebootingState is the State interface implementation for the UpdateHubStateRebooting
type RebootingState struct {
	BaseState
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

// ID returns the state id
func (state *RebootingState) ID() UpdateHubState {
	return state.id
}

// UpdateMetadata is the ReportableState interface implementation
func (state *RebootingState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}

// Handle for RebootingState implements the installation process itself
func (state *RebootingState) Handle(uh *UpdateHub) (State, bool) {
	err := uh.Reboot()
	if err != nil {
		return NewErrorState(state.apiClient, nil, NewTransientError(err)), false
	}

	return NewIdleState(), false
}

// NewRebootingState creates a new RebootingState
func NewRebootingState(apiClient *client.ApiClient, updateMetadata *metadata.UpdateMetadata) *RebootingState {
	state := &RebootingState{
		BaseState:      BaseState{id: UpdateHubStateRebooting},
		updateMetadata: updateMetadata,
	}

	state.apiClient = apiClient

	return state
}
