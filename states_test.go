package main

import (
	"errors"
	"testing"

	"bitbucket.org/ossystems/agent/metadata"

	"github.com/stretchr/testify/assert"
)

type EasyFotaTestController struct {
	EasyFota

	extraPoll        int
	updateAvailable  bool
	fetchUpdateError error
}

var checkUpdateCases = []struct {
	name         string
	controller   *EasyFotaTestController
	initialState State
	nextState    State
}{
	{
		"UpdateAvailable",
		&EasyFotaTestController{updateAvailable: true},
		NewUpdateCheckState(),
		&UpdateFetchState{},
	},

	{
		"UpdateNotAvailable",
		&EasyFotaTestController{updateAvailable: false},
		NewUpdateCheckState(),
		&PollState{},
	},
}

func (c *EasyFotaTestController) CheckUpdate() (*metadata.Metadata, int) {
	if c.updateAvailable {
		return &metadata.Metadata{}, c.extraPoll
	}

	return nil, c.extraPoll
}

func (c *EasyFotaTestController) FetchUpdate(updateMetadata *metadata.Metadata) error {
	return c.fetchUpdateError
}

func TestStateUpdateCheck(t *testing.T) {
	for _, tc := range checkUpdateCases {
		t.Run(tc.name, func(t *testing.T) {
			fota := &EasyFota{
				state:      tc.initialState,
				Controller: tc.controller,
			}

			next, _ := fota.state.Handle(fota)

			assert.IsType(t, tc.nextState, next)
		})
	}
}

func TestStateUpdateFetch(t *testing.T) {
	testCases := []struct {
		Name         string
		Controller   *EasyFotaTestController
		InitialState State
		NextState    State
	}{
		{
			"WithoutError",
			&EasyFotaTestController{fetchUpdateError: nil},
			NewUpdateFetchState(&metadata.Metadata{}),
			&InstallUpdateState{},
		},

		{
			"WithError",
			&EasyFotaTestController{fetchUpdateError: errors.New("fetch error")},
			NewUpdateFetchState(&metadata.Metadata{}),
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
			fota := &EasyFotaTestController{
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
