/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"errors"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/metadata"
	"github.com/bouk/monkey"

	"github.com/stretchr/testify/assert"
)

type testController struct {
	extraPoll               int
	updateAvailable         bool
	fetchUpdateError        error
	reportCurrentStateError error
}

var checkUpdateCases = []struct {
	name         string
	controller   *testController
	initialState State
	nextState    State
}{
	{
		"UpdateAvailable",
		&testController{updateAvailable: true},
		NewUpdateCheckState(),
		&UpdateFetchState{},
	},

	{
		"UpdateNotAvailable",
		&testController{updateAvailable: false},
		NewUpdateCheckState(),
		&PollState{},
	},
}

func (c *testController) CheckUpdate(retries int) (*metadata.UpdateMetadata, int) {
	if c.updateAvailable {
		return &metadata.UpdateMetadata{}, c.extraPoll
	}

	return nil, c.extraPoll
}

func (c *testController) FetchUpdate(updateMetadata *metadata.UpdateMetadata, cancel <-chan bool) error {
	return c.fetchUpdateError
}

func (c *testController) ReportCurrentState() error {
	return c.reportCurrentStateError
}

func TestStatePoll(t *testing.T) {
	testCases := []struct {
		caseName  string
		settings  *Settings
		nextState State
	}{
		{
			"PollingEnabled",
			&Settings{
				PollingSettings: PollingSettings{
					PollingEnabled:  true,
					PollingInterval: 1,
				},
			},
			&UpdateCheckState{},
		},

		{
			"PollingDisabled",
			&Settings{
				PollingSettings: PollingSettings{
					PollingEnabled: false,
				},
			},
			&PollState{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			uh, err := newTestUpdateHub(NewPollState())
			assert.NoError(t, err)

			uh.settings = tc.settings
			uh.settings.LastPoll = int(time.Now().Unix())

			next, _ := uh.state.Handle(uh)
			assert.IsType(t, tc.nextState, next)
		})
	}
}

func TestStateUpdateCheck(t *testing.T) {
	for _, tc := range checkUpdateCases {
		t.Run(tc.name, func(t *testing.T) {
			uh, err := newTestUpdateHub(tc.initialState)
			assert.NoError(t, err)

			uh.Controller = tc.controller

			next, _ := uh.state.Handle(uh)

			assert.IsType(t, tc.nextState, next)
		})
	}
}

func TestStateUpdateFetch(t *testing.T) {
	testCases := []struct {
		name         string
		controller   *testController
		initialState State
		nextState    State
	}{
		{
			"WithoutError",
			&testController{fetchUpdateError: nil},
			NewUpdateFetchState(&metadata.UpdateMetadata{}),
			&UpdateInstallState{},
		},

		{
			"WithError",
			&testController{fetchUpdateError: errors.New("fetch error")},
			NewUpdateFetchState(&metadata.UpdateMetadata{}),
			&UpdateFetchState{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uh, err := newTestUpdateHub(tc.initialState)
			assert.NoError(t, err)

			uh.Controller = tc.controller

			next, _ := uh.state.Handle(uh)

			assert.IsType(t, tc.nextState, next)
		})
	}
}

func TestPollTicks(t *testing.T) {
	testCases := []struct {
		name            string
		pollingInterval int
		extraPoll       int
	}{
		{
			"PollWithoutExtraPoll",
			10,
			0,
		},

		{
			"PollWithExtraPoll",
			13,
			88,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uh, err := newTestUpdateHub(NewUpdateCheckState())
			assert.NoError(t, err)

			c := &testController{
				updateAvailable: false,
				extraPoll:       tc.extraPoll,
			}

			uh.settings.FirstPoll = int(time.Now().Add(-1 * time.Second).Unix())
			uh.settings.LastPoll = int(time.Now().Add(-2 * time.Second).Unix())
			uh.settings.PollingInterval = tc.pollingInterval
			uh.Controller = c

			poll, _ := uh.state.Handle(uh)
			assert.IsType(t, &PollState{}, poll)

			poll.Handle(uh)
			assert.Equal(t, uh.settings.PollingInterval+c.extraPoll, poll.(*PollState).ticksCount)
		})
	}
}

func TestFirstPoll(t *testing.T) {
	uh, err := newTestUpdateHub(NewUpdateCheckState())
	assert.NoError(t, err)

	now := time.Now()
	count := -1
	guard := monkey.Patch(time.Now, func() time.Time {
		count++
		if count == 0 {
			return now
		}
		return now.Add(time.Second * time.Duration(count))
	})
	defer guard.Unpatch()

	c := &testController{
		updateAvailable: false,
		extraPoll:       0,
	}

	uh.pollingIntervalSpan = 5
	uh.settings.FirstPoll = 0
	uh.settings.PollingInterval = 10

	uh.Controller = c

	poll, _ := uh.state.Handle(uh)
	assert.IsType(t, &PollState{}, poll)

	state, _ := poll.Handle(uh)
	assert.Equal(t, int(now.Unix())+uh.pollingIntervalSpan, uh.settings.FirstPoll)
	assert.Equal(t, uh.pollingIntervalSpan, poll.(*PollState).ticksCount)
	assert.IsType(t, &UpdateCheckState{}, state)
}

func TestDelayedPolling(t *testing.T) {
	uh, err := newTestUpdateHub(NewUpdateCheckState())
	assert.NoError(t, err)

	now := time.Now()

	c := &testController{
		updateAvailable: false,
		extraPoll:       0,
	}

	uh.settings.FirstPoll = int(now.Add(-1 * time.Second).Unix())
	uh.settings.LastPoll = 0

	uh.Controller = c

	poll, _ := uh.state.Handle(uh)
	assert.IsType(t, &PollState{}, poll)

	state, _ := poll.Handle(uh)
	assert.Equal(t, 0, poll.(*PollState).ticksCount)
	assert.IsType(t, &UpdateCheckState{}, state)
}

func TestPollingRetries(t *testing.T) {
	uh, err := newTestUpdateHub(NewPollState())
	assert.NoError(t, err)

	c := &testController{
		updateAvailable: false,
		extraPoll:       -1,
	}

	uh.Controller = c
	uh.settings.LastPoll = int(time.Now().Unix())

	next, _ := uh.state.Handle(uh)
	assert.IsType(t, &UpdateCheckState{}, next)

	for i := 1; i < 3; i++ {
		state, _ := next.Handle(uh)
		assert.IsType(t, &PollState{}, state)
		next, _ = state.Handle(uh)
		assert.IsType(t, &UpdateCheckState{}, next)
		assert.Equal(t, i, uh.settings.PollingRetries)
	}

	c.updateAvailable = true
	c.extraPoll = 0

	next, _ = next.Handle(uh)
	assert.IsType(t, &UpdateFetchState{}, next)
	assert.Equal(t, 0, uh.settings.PollingRetries)
}

type testReportableState struct {
	BaseState
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

func (state *testReportableState) Handle(uh *UpdateHub) (State, bool) {
	return nil, true
}

func (state *testReportableState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}
