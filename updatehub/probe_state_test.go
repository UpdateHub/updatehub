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

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/controllermock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/stretchr/testify/assert"
)

var probeUpdateCases = []struct {
	name         string
	controller   *testController
	settings     *Settings
	initialState State
	nextState    State
	subTest      func(t *testing.T, uh *UpdateHub, state State)
}{
	{
		"UpdateAvailable",
		&testController{updateAvailable: true},
		&Settings{},
		NewProbeState(client.NewApiClient("address")),
		&DownloadingState{},
		func(t *testing.T, uh *UpdateHub, state State) {},
	},

	{
		"UpdateNotAvailable",
		&testController{updateAvailable: false},
		&Settings{},
		NewProbeState(client.NewApiClient("address")),
		&IdleState{},
		func(t *testing.T, uh *UpdateHub, state State) {},
	},

	{
		"ExtraPoll",
		&testController{updateAvailable: false, extraPoll: 5 * time.Second},
		&Settings{
			PollingSettings: PollingSettings{
				PersistentPollingSettings: PersistentPollingSettings{
					FirstPoll: time.Now().Add(-5 * time.Second),
				},
				PollingInterval: 15 * time.Second,
			},
		},
		NewProbeState(client.NewApiClient("address")),
		&PollState{},
		func(t *testing.T, uh *UpdateHub, state State) {
			poll := state.(*PollState)
			assert.Equal(t, 5*time.Second, poll.interval)
			assert.Equal(t, 5*time.Second, uh.Settings.ExtraPollingInterval)
		},
	},
}

func TestStateProbe(t *testing.T) {
	for _, tc := range probeUpdateCases {
		t.Run(tc.name, func(t *testing.T) {
			aim := &activeinactivemock.ActiveInactiveMock{}

			uh, err := newTestUpdateHub(tc.initialState, aim)
			assert.NoError(t, err)

			uh.Controller = tc.controller
			uh.Settings = tc.settings

			next, _ := uh.GetState().Handle(uh)

			assert.IsType(t, tc.nextState, next)

			tc.subTest(t, uh, next)

			aim.AssertExpectations(t)
		})
	}
}

func TestStateProbeWithUpdateAvailableButAlreadyInstalled(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &controllermock.ControllerMock{}

	apiClient := client.NewApiClient("address")

	uh, err := newTestUpdateHub(NewProbeState(apiClient), aim)
	assert.NoError(t, err)

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	uh.lastInstalledPackageUID = m.PackageUID()

	cm.On("ProbeUpdate", apiClient, 0).Return(m, time.Duration(0))

	uh.Controller = cm
	uh.Settings = &Settings{}

	next, _ := uh.GetState().Handle(uh)

	assert.IsType(t, &WaitingForRebootState{}, next)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateProbeToMap(t *testing.T) {
	state := NewProbeState(client.NewApiClient("address"))

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "probe"

	assert.Equal(t, expectedMap, state.ToMap())
}
