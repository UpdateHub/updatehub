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
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/activeinactive"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/copymock"
	"github.com/UpdateHub/updatehub/testsmocks/filemock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/UpdateHub/updatehub/testsmocks/statesmock"
	"github.com/UpdateHub/updatehub/utils"
	"github.com/bouk/monkey"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type testController struct {
	extraPoll               time.Duration
	pollingInterval         time.Duration
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
            "mode": "test",
            "sha256sum": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
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
            "sha256sum": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
            "target": "/dev/xx1",
            "target-type": "device"
          }
	    ]
        ,
	    [
	      {
            "mode": "test",
            "sha256sum": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
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
	settings     *Settings
	initialState State
	nextState    State
	subTest      func(t *testing.T, uh *UpdateHub, state State)
}{
	{
		"UpdateAvailable",
		&testController{updateAvailable: true},
		&Settings{},
		NewUpdateCheckState(),
		&UpdateFetchState{},
		func(t *testing.T, uh *UpdateHub, state State) {},
	},

	{
		"UpdateNotAvailable",
		&testController{updateAvailable: false},
		&Settings{},
		NewUpdateCheckState(),
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
		NewUpdateCheckState(),
		&PollState{},
		func(t *testing.T, uh *UpdateHub, state State) {
			poll := state.(*PollState)
			assert.Equal(t, 5*time.Second, poll.interval)
			assert.Equal(t, 5*time.Second, uh.settings.ExtraPollingInterval)
		},
	},
}

func (c *testController) CheckUpdate(retries int) (*metadata.UpdateMetadata, time.Duration) {
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

func TestCheckDownloadedObjectSha256sum(t *testing.T) {
	memFs := afero.NewMemMapFs()
	testPath, err := afero.TempDir(memFs, "", "states-test")

	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	err = afero.WriteFile(memFs, path.Join(testPath, expectedSha256sum), []byte("test"), 0666)
	assert.NoError(t, err)

	sci := &Sha256CheckerImpl{&utils.ExtendedIO{}}
	err = sci.CheckDownloadedObjectSha256sum(memFs, testPath, expectedSha256sum)
	assert.NoError(t, err)
}

func TestCheckDownloadedObjectSha256sumWithOpenError(t *testing.T) {
	dummyPath := "/dummy"
	dummySha256sum := "dummy_hash"

	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Open", path.Join(dummyPath, dummySha256sum)).Return(&filemock.FileMock{}, fmt.Errorf("open error"))

	sci := &Sha256CheckerImpl{&utils.ExtendedIO{}}
	err := sci.CheckDownloadedObjectSha256sum(fsm, dummyPath, dummySha256sum)
	assert.EqualError(t, err, "open error")

	fsm.AssertExpectations(t)
}

func TestCheckDownloadedObjectSha256sumWithCopyError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	testPath, err := afero.TempDir(memFs, "", "states-test")

	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	err = afero.WriteFile(memFs, path.Join(testPath, expectedSha256sum), []byte("test"), 0666)
	assert.NoError(t, err)

	cm := &copymock.CopierMock{}
	cm.On("Copy", mock.AnythingOfType("*sha256.digest"), mock.AnythingOfType("*mem.File"), time.Minute, mock.AnythingOfType("<-chan bool"), utils.ChunkSize, 0, -1, false).Return(false, fmt.Errorf("copy error"))

	sci := &Sha256CheckerImpl{cm}
	err = sci.CheckDownloadedObjectSha256sum(memFs, testPath, expectedSha256sum)
	assert.EqualError(t, err, "copy error")

	cm.AssertExpectations(t)
}

func TestCheckDownloadedObjectSha256sumWithSumsDontMatching(t *testing.T) {
	memFs := afero.NewMemMapFs()
	testPath, err := afero.TempDir(memFs, "", "states-test")

	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	err = afero.WriteFile(memFs, path.Join(testPath, expectedSha256sum), []byte("another"), 0666)
	assert.NoError(t, err)

	sci := &Sha256CheckerImpl{&utils.ExtendedIO{}}
	err = sci.CheckDownloadedObjectSha256sum(memFs, testPath, expectedSha256sum)
	assert.EqualError(t, err, "sha256sum's don't match. Expected: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08 / Calculated: ae448ac86c4e8e4dec645729708ef41873ae79c6dff84eff73360989487f08e5")
}

func TestStateUpdateCheck(t *testing.T) {
	for _, tc := range checkUpdateCases {
		t.Run(tc.name, func(t *testing.T) {
			uh, err := newTestUpdateHub(tc.initialState)
			assert.NoError(t, err)

			uh.Controller = tc.controller
			uh.settings = tc.settings

			next, _ := uh.state.Handle(uh)

			assert.IsType(t, tc.nextState, next)

			tc.subTest(t, uh, next)
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

			c <- time.Now().Add(elapsed)

			return ticker
		})
	}().Unpatch()

	c := &testController{
		updateAvailable: false,
		extraPoll:       -1,
	}

	uh.Controller = c
	uh.settings.PollingInterval = time.Second
	uh.settings.LastPoll = time.Now()

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
		pollingInterval     time.Duration
		firstPoll           time.Time
		expectedElapsedTime time.Duration
	}{
		{
			"NextRegularPollFromNow",
			10 * time.Second,
			now,
			10 * time.Second,
		},

		{
			"NextRegularPollFromPast",
			30 * time.Second,
			now.Add(-15 * time.Second),
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

					c <- now.Add(elapsed)

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
	poll.interval = 10 * time.Second

	go func() {
		assert.True(t, poll.Cancel(true))
	}()

	poll.Handle(uh)

	assert.Equal(t, int64(0), poll.ticksCount)
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
			&UpdateCheckState{},
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
	memFs := afero.NewOsFs()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		HardwareRevision: "",
		Version:          "version-value",
	}

	m := &metadata.UpdateMetadata{}
	s := NewUpdateInstallState(m, fm)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewInstallingState(m, &activeinactive.DefaultImpl{}, &Sha256CheckerImpl{&utils.ExtendedIO{}}, memFs)
	assert.Equal(t, expectedState, nextState)
}

func TestStateUpdateInstallWithCheckSupportedHardwareError(t *testing.T) {
	expectedErr := fmt.Errorf("this hardware doesn't match the hardware supported by the update")

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "hardware-value",
		HardwareRevision: "hardware-revision-value",
		Version:          "version-value",
	}

	m := &metadata.UpdateMetadata{}
	s := NewUpdateInstallState(m, fm)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)
}

func TestStateUpdateInstallWithChecksumError(t *testing.T) {
	expectedErr := fmt.Errorf("checksum error")

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "hardware-value",
		HardwareRevision: "hardware-revision-value",
		Version:          "version-value",
	}

	m := &metadata.UpdateMetadata{}

	guard := monkey.PatchInstanceMethod(reflect.TypeOf(m), "Checksum", func(*metadata.UpdateMetadata) (string, error) {
		return "", expectedErr
	})
	defer guard.Unpatch()

	s := NewUpdateInstallState(m, fm)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)
}

func TestStateUpdateInstallWithUpdateMetadataAlreadyInstalled(t *testing.T) {
	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "hardware-value",
		HardwareRevision: "hardware-revision-value",
		Version:          "version-value",
	}

	m := &metadata.UpdateMetadata{}
	s := NewUpdateInstallState(m, fm)

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
	memFs := afero.NewMemMapFs()

	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	scm := &statesmock.Sha256CheckerMock{}

	s := NewInstallingState(m, aim, scm, memFs)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	om.On("Setup").Return(nil)
	om.On("Install", uh.settings.DownloadDir).Return(nil)
	om.On("Cleanup").Return(nil)

	// "expectedSha256sum" got from "validJSONMetadata" content
	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	scm.On("CheckDownloadedObjectSha256sum", memFs, uh.settings.DownloadDir, expectedSha256sum).Return(nil)

	nextState, _ := s.Handle(uh)
	expectedState := NewInstalledState(m)
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
}

func TestStateInstallingWithActiveInactive(t *testing.T) {
	memFs := afero.NewMemMapFs()

	om := &objectmock.ObjectMock{}

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

	scm := &statesmock.Sha256CheckerMock{}

	s := NewInstallingState(m, aim, scm, memFs)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	om.On("Setup").Return(nil)
	om.On("Install", uh.settings.DownloadDir).Return(nil)
	om.On("Cleanup").Return(nil)

	// "expectedSha256sum" got from "validJSONMetadataWithActiveInactive" content
	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	scm.On("CheckDownloadedObjectSha256sum", memFs, uh.settings.DownloadDir, expectedSha256sum).Return(nil)

	nextState, _ := s.Handle(uh)
	expectedState := NewInstalledState(m)
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
}

func TestStateInstallingWithActiveError(t *testing.T) {
	memFs := afero.NewMemMapFs()

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

	scm := &statesmock.Sha256CheckerMock{}

	s := NewInstallingState(m, aim, scm, memFs)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
}

func TestStateInstallingWithSetActiveError(t *testing.T) {
	memFs := afero.NewMemMapFs()

	om := &objectmock.ObjectMock{}

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

	scm := &statesmock.Sha256CheckerMock{}

	s := NewInstallingState(m, aim, scm, memFs)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	om.On("Setup").Return(nil)
	om.On("Install", uh.settings.DownloadDir).Return(nil)
	om.On("Cleanup").Return(nil)

	// "expectedSha256sum" got from "validJSONMetadataWithActiveInactive" content
	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	scm.On("CheckDownloadedObjectSha256sum", memFs, uh.settings.DownloadDir, expectedSha256sum).Return(nil)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
}

func TestStateInstallingWithSetupError(t *testing.T) {
	memFs := afero.NewMemMapFs()

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

	scm := &statesmock.Sha256CheckerMock{}

	s := NewInstallingState(m, aim, scm, memFs)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	// "expectedSha256sum" got from "validJSONMetadata" content
	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	scm.On("CheckDownloadedObjectSha256sum", memFs, uh.settings.DownloadDir, expectedSha256sum).Return(nil)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
}

func TestStateInstallingWithInstallError(t *testing.T) {
	memFs := afero.NewMemMapFs()

	expectedErr := fmt.Errorf("install error")

	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	scm := &statesmock.Sha256CheckerMock{}

	s := NewInstallingState(m, aim, scm, memFs)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	om.On("Setup").Return(nil)
	om.On("Install", uh.settings.DownloadDir).Return(expectedErr)
	om.On("Cleanup").Return(nil)

	// "expectedSha256sum" got from "validJSONMetadata" content
	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	scm.On("CheckDownloadedObjectSha256sum", memFs, uh.settings.DownloadDir, expectedSha256sum).Return(nil)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
}

func TestStateInstallingWithCleanupError(t *testing.T) {
	memFs := afero.NewMemMapFs()

	expectedErr := fmt.Errorf("cleanup error")

	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	scm := &statesmock.Sha256CheckerMock{}

	s := NewInstallingState(m, aim, scm, memFs)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	om.On("Setup").Return(nil)
	om.On("Install", uh.settings.DownloadDir).Return(nil)
	om.On("Cleanup").Return(expectedErr)

	// "expectedSha256sum" got from "validJSONMetadata" content
	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	scm.On("CheckDownloadedObjectSha256sum", memFs, uh.settings.DownloadDir, expectedSha256sum).Return(nil)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
}

func TestStateInstallingWithInstallAndCleanupErrors(t *testing.T) {
	memFs := afero.NewMemMapFs()

	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	scm := &statesmock.Sha256CheckerMock{}

	s := NewInstallingState(m, aim, scm, memFs)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	om.On("Setup").Return(nil)
	om.On("Install", uh.settings.DownloadDir).Return(fmt.Errorf("install error"))
	om.On("Cleanup").Return(fmt.Errorf("cleanup error"))

	// "expectedSha256sum" got from "validJSONMetadata" content
	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	scm.On("CheckDownloadedObjectSha256sum", memFs, uh.settings.DownloadDir, expectedSha256sum).Return(nil)

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(fmt.Errorf("(install error); (cleanup error)")))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
}

func TestStateInstallingWithSha256Error(t *testing.T) {
	memFs := afero.NewMemMapFs()

	expectedErr := fmt.Errorf("sha256sum error")

	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	scm := &statesmock.Sha256CheckerMock{}

	s := NewInstallingState(m, aim, scm, memFs)

	uh, err := newTestUpdateHub(s)
	assert.NoError(t, err)

	// "expectedSha256sum" got from "validJSONMetadata" content
	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	scm.On("CheckDownloadedObjectSha256sum", memFs, uh.settings.DownloadDir, expectedSha256sum).Return(fmt.Errorf("sha256sum error"))

	nextState, _ := s.Handle(uh)
	expectedState := NewErrorState(NewTransientError(expectedErr))
	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
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
