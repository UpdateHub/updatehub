package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type StateTestController struct {
	EasyFota

	extraPoll        int
	updateAvailable  bool
	fetchUpdateError error
}

func (c *StateTestController) CheckUpdate() (bool, int) {
	return c.updateAvailable, c.extraPoll
}

func (c *StateTestController) FetchUpdate() error {
	return c.fetchUpdateError
}

func TestStateUpdateCheck(t *testing.T) {
	testCases := []struct {
		Name         string
		Controller   *StateTestController
		InitialState State
		NextState    State
	}{
		{
			"UpdateAvailable",
			&StateTestController{updateAvailable: true},
			NewUpdateCheckState(),
			&UpdateFetchState{},
		},

		{
			"UpdateNotAvailable",
			&StateTestController{updateAvailable: false},
			NewUpdateCheckState(),
			&PollState{},
		},

		{
			"ExtraPoll",
			&StateTestController{updateAvailable: false, extraPoll: 5},
			NewUpdateCheckState(),
			&PollState{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			fota := &EasyFota{
				state:      tc.InitialState,
				Controller: tc.Controller,
			}

			next, _ := fota.state.Handle(fota)

			assert.IsType(t, tc.NextState, next)
		})
	}
}

func TestStateUpdateFetch(t *testing.T) {
	testCases := []struct {
		Name         string
		Controller   *StateTestController
		InitialState State
		NextState    State
	}{
		{
			"WithoutError",
			&StateTestController{fetchUpdateError: nil},
			NewUpdateFetchState(),
			&InstallUpdateState{},
		},

		{
			"WithError",
			&StateTestController{fetchUpdateError: errors.New("fetch error")},
			NewUpdateFetchState(),
			&UpdateFetchState{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			fota := tc.Controller
			fota.EasyFota.state = tc.InitialState
			fota.Controller = tc.Controller

			next, _ := fota.state.Handle(&fota.EasyFota)

			assert.IsType(t, tc.NextState, next)
		})
	}
}

func TestPollTicks(t *testing.T) {
	testCases := []struct {
		Name         string
		PollInterval int
		ExtraPoll    int
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
		t.Run(tc.Name, func(t *testing.T) {
			fota := &StateTestController{
				updateAvailable: false,
				extraPoll:       tc.ExtraPoll,
			}

			fota.EasyFota.pollInterval = tc.PollInterval
			fota.EasyFota.state = NewUpdateCheckState()
			fota.Controller = fota

			poll, _ := fota.state.Handle(&fota.EasyFota)

			assert.IsType(t, &PollState{}, poll)

			poll.Handle(&fota.EasyFota)

			assert.Equal(t, fota.EasyFota.pollInterval+fota.extraPoll, poll.(*PollState).ticksCount)
		})
	}
}
