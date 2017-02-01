package main

import (
	"errors"
	"testing"

	"bitbucket.org/ossystems/agent/metadata"

	"github.com/stretchr/testify/assert"
)

type testController struct {
	extraPoll        int
	updateAvailable  bool
	fetchUpdateError error
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

func (c *testController) CheckUpdate() (*metadata.UpdateMetadata, int) {
	if c.updateAvailable {
		return &metadata.UpdateMetadata{}, c.extraPoll
	}

	return nil, c.extraPoll
}

func (c *testController) FetchUpdate(updateMetadata *metadata.UpdateMetadata, cancel <-chan bool) error {
	return c.fetchUpdateError
}

func TestStateUpdateCheck(t *testing.T) {
	for _, tc := range checkUpdateCases {
		t.Run(tc.name, func(t *testing.T) {
			fota, err := newTestEasyFota(tc.initialState)
			assert.NoError(t, err)

			fota.Controller = tc.controller

			next, _ := fota.state.Handle(fota)

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
			&InstallUpdateState{},
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
			fota, err := newTestEasyFota(tc.initialState)
			assert.NoError(t, err)

			fota.Controller = tc.controller

			next, _ := fota.state.Handle(fota)

			assert.IsType(t, tc.nextState, next)
		})
	}
}

func TestPollTicks(t *testing.T) {
	testCases := []struct {
		name         string
		pollInterval int
		extraPoll    int
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
			fota, err := newTestEasyFota(NewUpdateCheckState())
			assert.NoError(t, err)

			c := &testController{
				updateAvailable: false,
				extraPoll:       tc.extraPoll,
			}

			fota.pollInterval = tc.pollInterval
			fota.Controller = c

			poll, _ := fota.state.Handle(fota)
			assert.IsType(t, &PollState{}, poll)

			poll.Handle(fota)
			assert.Equal(t, fota.pollInterval+c.extraPoll, poll.(*PollState).ticksCount)
		})
	}
}
