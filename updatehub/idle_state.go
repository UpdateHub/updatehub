/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"time"

	"github.com/OSSystems/pkg/log"
)

// IdleState is the State interface implementation for the UpdateHubStateIdle
type IdleState struct {
	BaseState
	CancellableState
}

// ID returns the state id
func (state *IdleState) ID() UpdateHubState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *IdleState) Cancel(ok bool, nextState State) bool {
	return state.CancellableState.Cancel(ok, nextState)
}

// Handle for IdleState
func (state *IdleState) Handle(uh *UpdateHub) (State, bool) {
	if !uh.Settings.PollingEnabled {
		state.Wait()
		return state.NextState(), false
	}

	now := time.Now()

	if uh.Settings.ExtraPollingInterval > 0 {
		extraPollTime := uh.Settings.LastPoll.Add(uh.Settings.ExtraPollingInterval)

		log.Info("ExtraPoll received: ", extraPollTime)

		if extraPollTime.Before(now) {
			return NewUpdateProbeState(), false
		}
	}

	return NewPollState(uh.Settings.PollingInterval), false
}

// NewIdleState creates a new IdleState
func NewIdleState() *IdleState {
	state := &IdleState{
		BaseState:        BaseState{id: UpdateHubStateIdle},
		CancellableState: CancellableState{cancel: make(chan bool)},
	}

	return state
}
