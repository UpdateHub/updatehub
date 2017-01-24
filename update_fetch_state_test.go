package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
