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
	"fmt"
	"reflect"
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

func TestStateUpdateInstall(t *testing.T) {
	m := &metadata.UpdateMetadata{}
	s := NewUpdateInstallState(m)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewInstallingState(m)
	assert.Equal(t, expectedState, nextState)
}

func TestStateUpdateInstallWithChecksumError(t *testing.T) {
	expectedErr := fmt.Errorf("checksum error")

	m := &metadata.UpdateMetadata{}

	guard := monkey.PatchInstanceMethod(reflect.TypeOf(m), "Checksum", func(*metadata.UpdateMetadata) (string, error) {
		return "", expectedErr
	})
	defer guard.Unpatch()

	s := NewUpdateInstallState(m)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)
}

func TestStateUpdateInstallWithUpdateMetadataAlreadyInstalled(t *testing.T) {
	m := &metadata.UpdateMetadata{}
	s := NewUpdateInstallState(m)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	uh.lastInstalledPackageUID, _ = m.Checksum()

	nextState, _ := s.Handle(uh)
	expectedState := NewWaitingForRebootState(m)
	assert.Equal(t, expectedState, nextState)
}

func TestStateInstalling(t *testing.T) {
	m := &metadata.UpdateMetadata{}
	s := NewInstallingState(m)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewInstalledState(m)
	assert.Equal(t, expectedState, nextState)
}

func TestStateWaitingForReboot(t *testing.T) {
	m := &metadata.UpdateMetadata{}
	s := NewWaitingForRebootState(m)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewPollState()
	// we can't assert Equal here because NewPollState() creates a
	// channel dynamically
	assert.IsType(t, expectedState, nextState)
}

func TestStateInstalled(t *testing.T) {
	m := &metadata.UpdateMetadata{}
	s := NewInstalledState(m)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewPollState()
	// we can't assert Equal here because NewPollState() creates a
	// channel dynamically
	assert.IsType(t, expectedState, nextState)
}
