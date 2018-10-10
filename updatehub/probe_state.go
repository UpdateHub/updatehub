/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"net"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/pkg/errors"
	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
)

// ProbeState is the State interface implementation for the UpdateHubStateProbe
type ProbeState struct {
	BaseState
	CancellableState

	ProbeResponseReady  chan bool
	probeUpdateMetadata *metadata.UpdateMetadata
	probeExtraPoll      time.Duration
}

func (state *ProbeState) ProbeResponse() (*metadata.UpdateMetadata, time.Duration) {
	return state.probeUpdateMetadata, state.probeExtraPoll
}

// ID returns the state id
func (state *ProbeState) ID() UpdateHubState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *ProbeState) Cancel(ok bool, nextState State) bool {
	state.CancellableState.Cancel(ok, nextState)
	return ok
}

// Handle for ProbeState executes a ProbeUpdate procedure and
// proceed to download the update if there is one. It goes back to the
// polling state otherwise.
func (state *ProbeState) Handle(uh *UpdateHub) (State, bool) {
	var signature []byte
	var err error

	var nextState State
	nextState = state

	pollingRetries := uh.Settings.PollingRetries

	go func() {
	loop:
		for {
			select {
			case <-time.After(time.Second):
				state.probeUpdateMetadata, signature, state.probeExtraPoll, err = uh.Controller.ProbeUpdate(state.apiClient, pollingRetries)

				if neterr, ok := errors.Cause(err).(net.Error); ok {
					if neterr.Timeout() {
						log.Warn("timeout during download update")
					}

					nextState = NewProbeState(uh.DefaultApiClient)
				}

				break loop
			case <-state.cancel:
				break loop
			}
		}

		state.Cancel(true, nextState)
	}()

	state.Wait()

	// "non-blocking" write to channel
	select {
	case state.ProbeResponseReady <- true:
	default:
	}

	// Increment the number of polling retries
	uh.Settings.PollingRetries++

	defer uh.Settings.Save(uh.Store)

	// state cancelled
	if state.NextState() != state {
		return state.NextState(), true
	}

	// Reset polling retries and disable ASAP mode in case of extra polling
	if state.probeExtraPoll > 0 {
		uh.Settings.PollingRetries = 0
		uh.Settings.ProbeASAP = false
	}

	uh.Settings.LastPoll = time.Now()
	uh.Settings.ExtraPollingInterval = 0

	if state.probeUpdateMetadata != nil {
		// Reset polling retries and disable ASAP mode in case of ProbeUpdate success
		uh.Settings.PollingRetries = 0
		uh.Settings.ProbeASAP = false

		packageUID := state.probeUpdateMetadata.PackageUID()
		if packageUID == uh.lastInstalledPackageUID {
			return NewIdleState(), false
		}

		if !state.probeUpdateMetadata.VerifySignature(uh.PublicKey, signature) {
			err := errors.New("invalid signature")
			return NewErrorState(state.apiClient, state.probeUpdateMetadata, NewTransientError(err)), false
		}

		pendingDownload, err := uh.hasPendingDownload(state.probeUpdateMetadata)
		if !pendingDownload && err == nil {
			return NewInstallingState(state.apiClient, state.probeUpdateMetadata, &ProgressTrackerImpl{}, uh.Store), false
		}

		return NewDownloadingState(state.apiClient, state.probeUpdateMetadata, &ProgressTrackerImpl{}), false
	}

	if state.probeExtraPoll > 0 {
		now := time.Now()
		nextPoll := time.Unix(uh.Settings.FirstPoll.Unix(), 0)
		probeExtraPollTime := now.Add(state.probeExtraPoll)

		for nextPoll.Before(now) {
			nextPoll = nextPoll.Add(uh.Settings.PollingInterval)
		}

		if probeExtraPollTime.Before(nextPoll) {
			uh.Settings.ExtraPollingInterval = state.probeExtraPoll

			poll := NewPollState(uh.Settings.PollingInterval)
			poll.interval = state.probeExtraPoll

			return poll, false
		}
	}

	return NewIdleState(), false
}

// NewProbeState creates a new ProbeState
func NewProbeState(apiClient *client.ApiClient) *ProbeState {
	state := &ProbeState{
		BaseState:        BaseState{id: UpdateHubStateProbe},
		CancellableState: CancellableState{cancel: make(chan bool, 1)},
	}

	state.apiClient = apiClient
	state.ProbeResponseReady = make(chan bool, 1)

	return state
}
