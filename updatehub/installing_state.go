/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"sync"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/spf13/afero"
)

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
		return NewWaitingForRebootState(state.apiClient, state.updateMetadata), false
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
		return NewErrorState(state.apiClient, state.updateMetadata, NewTransientError(err)), false
	}

	return NewInstalledState(state.apiClient, state.updateMetadata), false
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
	apiClient *client.ApiClient,
	updateMetadata *metadata.UpdateMetadata,
	pti ProgressTracker,
	fsb afero.Fs) *InstallingState {
	state := &InstallingState{
		BaseState:         BaseState{id: UpdateHubStateInstalling},
		updateMetadata:    updateMetadata,
		FileSystemBackend: fsb,
		ProgressTracker:   pti,
	}

	state.apiClient = apiClient

	return state
}
