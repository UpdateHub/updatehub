package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			&IdleState{},
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
