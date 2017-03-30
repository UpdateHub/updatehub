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

	"github.com/UpdateHub/updatehub/activeinactive"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/bouk/monkey"
	"github.com/stretchr/testify/assert"
)

type testController struct {
	extraPoll               int
	updateAvailable         bool
	fetchUpdateError        error
	reportCurrentStateError error
}

const (
	validJSONMetadata = `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      {
            "mode": "test"
          }
	    ]
	  ]
	}`

	validJSONMetadataWithActiveInactive = `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      {
            "mode": "test",
            "target": "/dev/xx1",
            "target-type": "device"
          }
	    ]
        ,
	    [
	      {
            "mode": "test",
            "target": "/dev/xx2",
            "target-type": "device"
          }
	    ]
	  ]
	}`
)

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
		&IdleState{},
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

func TestPollingRetries(t *testing.T) {
	uh, err := newTestUpdateHub(NewPollState())
	assert.NoError(t, err)

	var elapsed time.Duration

	// Simulate ticker
	defer func() *monkey.PatchGuard {
		return monkey.Patch(time.NewTicker, func(d time.Duration) *time.Ticker {
			elapsed += d

			c := make(chan time.Time, 1)
			ticker := &time.Ticker{
				C: c,
			}

			c <- time.Now().Add(elapsed * time.Second)

			return ticker
		})
	}().Unpatch()

	c := &testController{
		updateAvailable: false,
		extraPoll:       -1,
	}

	uh.Controller = c
	uh.settings.PollingInterval = int(time.Second)
	uh.settings.LastPoll = int(time.Now().Unix())

	next, _ := uh.state.Handle(uh)
	assert.IsType(t, &UpdateCheckState{}, next)

	for i := 1; i < 3; i++ {
		state, _ := next.Handle(uh)
		assert.IsType(t, &IdleState{}, state)
		next, _ = state.Handle(uh)
		assert.IsType(t, &PollState{}, next)
		next, _ = next.Handle(uh)
		assert.IsType(t, &UpdateCheckState{}, next)
		assert.Equal(t, i, uh.settings.PollingRetries)
	}

	c.updateAvailable = true
	c.extraPoll = 0

	next, _ = next.Handle(uh)
	assert.IsType(t, &UpdateFetchState{}, next)
	assert.Equal(t, 0, uh.settings.PollingRetries)
}

func TestPolling(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name                string
		pollingInterval     int
		firstPoll           int
		expectedElapsedTime time.Duration
	}{
		{
			"Now",
			10 * int(time.Second),
			int(now.Unix()),
			0,
		},

		{
			"NextRegularPoll",
			30 * int(time.Second),
			int(now.Add(-15 * time.Second).Unix()),
			15 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uh, _ := newTestUpdateHub(nil)

			var elapsed time.Duration

			// Simulate ticker
			defer func() *monkey.PatchGuard {
				return monkey.Patch(time.NewTicker, func(d time.Duration) *time.Ticker {
					elapsed += d

					c := make(chan time.Time, 1)
					ticker := &time.Ticker{
						C: c,
					}

					c <- time.Now().Add(elapsed * time.Second)

					return ticker
				})
			}().Unpatch()

			// Simulate time passage from now
			defer func() *monkey.PatchGuard {
				seconds := -1
				return monkey.Patch(time.Now, func() time.Time {
					seconds++
					return now.Add(time.Second * time.Duration(seconds))
				})
			}().Unpatch()

			uh.settings.PollingInterval = tc.pollingInterval
			uh.settings.FirstPoll = tc.firstPoll
			uh.settings.LastPoll = tc.firstPoll

			uh.StartPolling()

			poll := uh.state
			assert.IsType(t, &PollState{}, poll)

			poll.Handle(uh)
			assert.Equal(t, tc.expectedElapsedTime, elapsed)
		})
	}
}

func TestCancelPollState(t *testing.T) {
	uh, _ := newTestUpdateHub(nil)

	poll := NewPollState()
	poll.interval = int(10 * time.Second)

	go func() {
		assert.True(t, poll.Cancel(true))
	}()

	poll.Handle(uh)

	assert.Equal(t, 0, poll.ticksCount)
}

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
	}

	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			uh, err := newTestUpdateHub(NewIdleState())
			assert.NoError(t, err)

			uh.settings = tc.settings

			go func() {
				uh.state.Cancel(false)
			}()

			next, _ := uh.state.Handle(uh)
			assert.IsType(t, tc.nextState, next)
		})
	}
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
	expectedState := NewInstallingState(m, &activeinactive.DefaultImpl{})
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

type TestObject struct {
	metadata.Object
}

func TestStateInstalling(t *testing.T) {
	om := &objectmock.ObjectMock{}
	om.On("Setup").Return(nil)
	om.On("Install").Return(nil)
	om.On("Cleanup").Return(nil)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})

	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	s := NewInstallingState(m, aim)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewInstalledState(m)
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateInstallingWithActiveInactive(t *testing.T) {
	om := &objectmock.ObjectMock{}
	om.On("Setup").Return(nil)
	om.On("Install").Return(nil)
	om.On("Cleanup").Return(nil)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})

	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadataWithActiveInactive))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(1, nil)
	aim.On("SetActive", 0).Return(nil)

	s := NewInstallingState(m, aim)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewInstalledState(m)
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateInstallingWithActiveError(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})

	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadataWithActiveInactive))
	assert.NoError(t, err)

	expectedErr := fmt.Errorf("active error")

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(0, expectedErr)

	s := NewInstallingState(m, aim)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateInstallingWithSetActiveError(t *testing.T) {
	om := &objectmock.ObjectMock{}
	om.On("Setup").Return(nil)
	om.On("Install").Return(nil)
	om.On("Cleanup").Return(nil)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})

	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadataWithActiveInactive))
	assert.NoError(t, err)

	expectedErr := fmt.Errorf("set active error")

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(1, nil)
	aim.On("SetActive", 0).Return(expectedErr)

	s := NewInstallingState(m, aim)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateInstallingWithSetupError(t *testing.T) {
	expectedErr := fmt.Errorf("setup error")

	om := &objectmock.ObjectMock{}
	om.On("Setup").Return(expectedErr)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})

	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	s := NewInstallingState(m, aim)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateInstallingWithInstallError(t *testing.T) {
	expectedErr := fmt.Errorf("install error")

	om := &objectmock.ObjectMock{}
	om.On("Setup").Return(nil)
	om.On("Install").Return(expectedErr)
	om.On("Cleanup").Return(nil)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})

	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	s := NewInstallingState(m, aim)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateInstallingWithCleanupError(t *testing.T) {
	expectedErr := fmt.Errorf("cleanup error")

	om := &objectmock.ObjectMock{}
	om.On("Setup").Return(nil)
	om.On("Install").Return(nil)
	om.On("Cleanup").Return(expectedErr)

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})

	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	s := NewInstallingState(m, aim)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateInstallingWithInstallAndCleanupErrors(t *testing.T) {
	om := &objectmock.ObjectMock{}
	om.On("Setup").Return(nil)
	om.On("Install").Return(fmt.Errorf("install error"))
	om.On("Cleanup").Return(fmt.Errorf("cleanup error"))

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})

	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	s := NewInstallingState(m, aim)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(fmt.Errorf("(install error); (cleanup error)")))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestGetIndexOfObjectToBeInstalled(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})

	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadataWithActiveInactive))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(m.Objects))

	testCases := []struct {
		caseName  string
		active    int
		installTo int
	}{
		{
			"ActiveZero",
			0,
			1,
		},
		{
			"ActiveOne",
			1,
			0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			aim := &activeinactivemock.ActiveInactiveMock{}
			aim.On("Active").Return(tc.active, nil)
			index, err := GetIndexOfObjectToBeInstalled(aim, m)
			assert.NoError(t, err)
			assert.Equal(t, tc.installTo, index)
		})
	}
}

func TestGetIndexOfObjectToBeInstalledWithActiveError(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})

	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadataWithActiveInactive))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(m.Objects))

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(1, fmt.Errorf("active error"))
	index, err := GetIndexOfObjectToBeInstalled(aim, m)
	assert.EqualError(t, err, "active error")
	assert.Equal(t, 0, index)
}

func TestGetIndexOfObjectToBeInstalledWithMoreThanTwoObjects(t *testing.T) {
	activeInactiveJSONMetadataWithThreeObjects := `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      {
            "mode": "copy",
            "target": "/dev/xx1",
            "target-type": "device"
          }
	    ]
        ,
	    [
	      {
            "mode": "copy",
            "target": "/dev/xx2",
            "target-type": "device"
          }
	    ]
        ,
	    [
	      {
            "mode": "copy",
            "target": "/dev/xx3",
            "target-type": "device"
          }
	    ]
	  ]
	}`

	m, err := metadata.NewUpdateMetadata([]byte(activeInactiveJSONMetadataWithThreeObjects))
	assert.NoError(t, err)
	assert.Equal(t, 3, len(m.Objects))

	aim := &activeinactivemock.ActiveInactiveMock{}
	index, err := GetIndexOfObjectToBeInstalled(aim, m)
	assert.EqualError(t, err, "update metadata must have 1 or 2 objects. Found 3")
	assert.Equal(t, 0, index)
}

func TestGetIndexOfObjectToBeInstalledWithNoObjects(t *testing.T) {
	activeInactiveJSONMetadataWithThreeObjects := `{
	  "product-uid": "0123456789",
	  "objects": [
	  ]
	}`

	m, err := metadata.NewUpdateMetadata([]byte(activeInactiveJSONMetadataWithThreeObjects))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(m.Objects))

	aim := &activeinactivemock.ActiveInactiveMock{}
	index, err := GetIndexOfObjectToBeInstalled(aim, m)
	assert.EqualError(t, err, "update metadata must have 1 or 2 objects. Found 0")
	assert.Equal(t, 0, index)
}

func TestStateWaitingForReboot(t *testing.T) {
	m := &metadata.UpdateMetadata{}
	s := NewWaitingForRebootState(m)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewIdleState()
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
	expectedState := NewIdleState()
	// we can't assert Equal here because NewPollState() creates a
	// channel dynamically
	assert.IsType(t, expectedState, nextState)
}
