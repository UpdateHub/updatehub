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
	"github.com/updatehub/updatehub/client"
	"github.com/updatehub/updatehub/metadata"
)

// ProbeState is the State interface implementation for the UpdateHubStateProbe
type ProbeState struct {
	BaseState

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

// Handle for ProbeState executes a ProbeUpdate procedure and
// proceed to download the update if there is one. It goes back to the
// polling state otherwise.
func (state *ProbeState) Handle(uh *UpdateHub) (State, bool) {
	var signature []byte
	var err error

	for {
		state.probeUpdateMetadata, signature, state.probeExtraPoll, err = uh.Controller.ProbeUpdate(state.apiClient, uh.Settings.PollingRetries)

		if neterr, ok := errors.Cause(err).(net.Error); ok {
			if neterr.Timeout() {
				log.Warn("timeout during download update")
			}

			uh.Settings.PollingRetries++

			time.Sleep(time.Second)

			continue
		}

		break
	}

	// "non-blocking" write to channel
	select {
	case state.ProbeResponseReady <- true:
	default:
	}

	// Reset polling retries and disable ASAP mode in case of ProbeUpdate success
	if state.probeExtraPoll != -1 {
		uh.Settings.PollingRetries = 0
		uh.Settings.ProbeASAP = false
	}

	uh.Settings.LastPoll = time.Now()
	uh.Settings.ExtraPollingInterval = 0
	uh.Settings.Save(uh.Store)

	if state.probeUpdateMetadata != nil {
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
			uh.Settings.Save(uh.Store)

			poll := NewPollState(uh.Settings.PollingInterval)
			poll.interval = state.probeExtraPoll

			return poll, false
		}
	}

	// Increment the number of polling retries in case of ProbeUpdate failure
	uh.Settings.PollingRetries++
	uh.Settings.Save(uh.Store)

	return NewIdleState(), false
}

// NewProbeState creates a new ProbeState
func NewProbeState(apiClient *client.ApiClient) *ProbeState {
	state := &ProbeState{
		BaseState: BaseState{id: UpdateHubStateProbe},
	}

	state.apiClient = apiClient
	state.ProbeResponseReady = make(chan bool, 1)

	return state
}
