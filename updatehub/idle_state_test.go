/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"testing"
	"time"

	"github.com/updatehub/updatehub/testsmocks/activeinactivemock"
	"github.com/stretchr/testify/assert"
)

func TestNewIdleState(t *testing.T) {
	state := NewIdleState()
	assert.IsType(t, &IdleState{}, state)
	assert.Equal(t, UpdateHubState(UpdateHubStateIdle), state.ID())
}

func TestStateIdle(t *testing.T) {
	testCases := []struct {
		caseName  string
		settings  *Settings
		nextState State
	}{
		{
			"PollingEnabled",
			&Settings{
				PollingSettings: PollingSettings{
					PollingEnabled: true,
				},
			},
			&PollState{},
		},

		{
			"PollingDisabled",
			&Settings{
				PollingSettings: PollingSettings{
					PollingEnabled: false,
				},
			},
			&IdleState{},
		},

		{
			"ExtraPollingBeforeNow",
			&Settings{
				PollingSettings: PollingSettings{
					PersistentPollingSettings: PersistentPollingSettings{
						LastPoll:             time.Now().Add(-10 * time.Second),
						ExtraPollingInterval: 5 * time.Second,
					},
					PollingEnabled: true,
				},
			},
			&ProbeState{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			aim := &activeinactivemock.ActiveInactiveMock{}

			uh, err := newTestUpdateHub(NewIdleState(), aim)
			assert.NoError(t, err)

			uh.Settings = tc.settings

			go func() {
				uh.Cancel(NewIdleState()) // write
			}()

			next, _ := uh.GetState().Handle(uh) // read
			assert.IsType(t, tc.nextState, next)

			aim.AssertExpectations(t)

			expectedMap := map[string]interface{}{}
			expectedMap["status"] = "idle"

			assert.Equal(t, expectedMap, uh.GetState().ToMap())
		})
	}
}

func TestStateIdleToMap(t *testing.T) {
	state := NewIdleState()

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "idle"

	assert.Equal(t, expectedMap, state.ToMap())
}
