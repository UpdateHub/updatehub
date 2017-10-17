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

// DownloadedState is the State interface implementation for the UpdateHubStateDownloaded
type DownloadedState struct {
	BaseState
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

// ID returns the state id
func (state *DownloadedState) ID() UpdateHubState {
	return state.id
}

// UpdateMetadata is the ReportableState interface implementation
func (state *DownloadedState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}

// Handle for DownloadedState just returns a new installing state
func (state *DownloadedState) Handle(uh *UpdateHub) (State, bool) {
	return NewInstallingState(state.apiClient, state.updateMetadata, &ProgressTrackerImpl{}, uh.Store), false
}

// NewDownloadedState creates a new DownloadedState
func NewDownloadedState(apiClient *client.ApiClient, updateMetadata *metadata.UpdateMetadata) *DownloadedState {
	state := &DownloadedState{
		BaseState:      BaseState{id: UpdateHubStateDownloaded},
		updateMetadata: updateMetadata,
	}

	state.apiClient = apiClient

	return state
}
