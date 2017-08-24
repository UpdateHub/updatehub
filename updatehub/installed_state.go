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
	uh.lastInstalledPackageUID = state.updateMetadata.PackageUID()

	uh.Settings.ProbeASAP = true
	uh.Settings.Save(uh.Store)

	return NewRebootingState(state.apiClient, state.updateMetadata), false
}

// UpdateMetadata is the ReportableState interface implementation
func (state *InstalledState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}

// NewInstalledState creates a new InstalledState
func NewInstalledState(apiClient *client.ApiClient, updateMetadata *metadata.UpdateMetadata) *InstalledState {
	state := &InstalledState{
		BaseState:      BaseState{id: UpdateHubStateInstalled},
		updateMetadata: updateMetadata,
	}

	state.apiClient = apiClient

	return state
}
