/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package updatehub

import (
	"fmt"
	"time"

	"github.com/OSSystems/pkg/log"
)

// PollState is the State interface implementation for the UpdateHubStatePoll
type PollState struct {
	BaseState
	CancellableState

	interval   time.Duration
	ticksCount int64
}

// ID returns the state id
func (state *PollState) ID() UpdateHubState {
	return state.id
}

// Cancel cancels a state if it is cancellable
func (state *PollState) Cancel(ok bool, nextState State) bool {
	return state.CancellableState.Cancel(ok, nextState)
}

// Handle for PollState encapsulates the polling logic
func (state *PollState) Handle(uh *UpdateHub) (State, bool) {
	if !uh.Settings.PollingEnabled && !uh.Settings.ProbeASAP {
		state.Wait()
		return state.NextState(), false
	}

	var nextState State

	nextState = state

	if state.interval < uh.TimeStep {
		finalErr := fmt.Errorf("polling interval (%s) must be greater than '%s', using %s instead", state.interval, uh.TimeStep, uh.TimeStep)
		log.Warn(finalErr)
		state.interval = uh.TimeStep
	}

	go func() {
		ticks := state.ticksCount

	polling:
		for {
			ticker := time.NewTicker(uh.TimeStep)

			defer ticker.Stop()

			select {
			case <-ticker.C:
				ticks++

				if ticks > 0 && ticks%int64(state.interval.Seconds()/uh.TimeStep.Seconds()) == 0 {
					nextState = NewProbeState(uh.DefaultApiClient)
					break polling
				}
			case <-state.cancel:
				break
			}
		}

		state.Cancel(true, nextState)

		state.ticksCount = ticks
	}()

	state.Wait()

	// state cancelled
	if state.NextState() != nil {
		return state.NextState(), true
	}

	return nextState, false
}

// NewPollState creates a new PollState
func NewPollState(pollingInterval time.Duration) *PollState {
	state := &PollState{
		BaseState:        BaseState{id: UpdateHubStatePoll},
		CancellableState: CancellableState{cancel: make(chan bool, 1)},
	}

	state.interval = pollingInterval

	return state
}
