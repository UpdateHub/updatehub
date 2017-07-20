/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/installifdifferentmock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/UpdateHub/updatehub/testsmocks/progresstrackermock"
	"github.com/UpdateHub/updatehub/testsmocks/statesmock"
	"github.com/bouk/monkey"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type testController struct {
	extraPoll               time.Duration
	pollingInterval         time.Duration
	updateAvailable         bool
	fetchUpdateError        error
	installUpdateError      error
	reportCurrentStateError error
	progressList            []int
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

func (c *testController) CheckUpdate(retries int) (*metadata.UpdateMetadata, time.Duration) {
	if c.updateAvailable {
		return &metadata.UpdateMetadata{}, c.extraPoll
	}

	return nil, c.extraPoll
}

func (c *testController) FetchUpdate(updateMetadata *metadata.UpdateMetadata, cancel <-chan bool, progressChan chan<- int) error {
	for _, p := range c.progressList {
		// "non-blocking" write to channel
		select {
		case progressChan <- p:
		default:
		}
	}

	return c.fetchUpdateError
}

func (c *testController) InstallUpdate(updateMetadata *metadata.UpdateMetadata, progressChan chan<- int) error {
	for _, p := range c.progressList {
		// "non-blocking" write to channel
		select {
		case progressChan <- p:
		default:
		}
	}

	return c.installUpdateError
}

func (c *testController) ReportCurrentState() error {
	return c.reportCurrentStateError
}

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
		&DownloadingState{},
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
			assert.Equal(t, 5*time.Second, uh.Settings.ExtraPollingInterval)
		},
	},
}

func TestStateUpdateCheck(t *testing.T) {
	for _, tc := range checkUpdateCases {
		t.Run(tc.name, func(t *testing.T) {
			aim := &activeinactivemock.ActiveInactiveMock{}

			uh, err := newTestUpdateHub(tc.initialState, aim)
			assert.NoError(t, err)

			uh.Controller = tc.controller
			uh.Settings = tc.settings

			next, _ := uh.State.Handle(uh)

			assert.IsType(t, tc.nextState, next)

			tc.subTest(t, uh, next)

			aim.AssertExpectations(t)
		})
	}
}

func TestStateUpdateCheckToMap(t *testing.T) {
	state := NewUpdateCheckState()

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "update-check"

	assert.Equal(t, expectedMap, state.ToMap())
}

func TestStateDownloading(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	memFs := afero.NewMemMapFs()

	testCases := []struct {
		name               string
		controller         *testController
		expectedState      State
		expectedProgresses []int
	}{
		{
			"WithoutError",
			&testController{fetchUpdateError: nil, installUpdateError: nil, progressList: []int{33, 66, 99, 100}},
			NewDownloadedState(m),
			[]int{33, 66, 99, 100},
		},

		{
			"WithError",
			&testController{fetchUpdateError: errors.New("fetch error"), installUpdateError: nil, progressList: []int{33}},
			NewErrorState(m, NewTransientError(errors.New("fetch error"))),
			[]int{33},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ptm := &progresstrackermock.ProgressTrackerMock{}
			for _, p := range tc.expectedProgresses {
				ptm.On("SetProgress", p).Once()
			}

			s := NewDownloadingState(m, ptm)

			uh, err := newTestUpdateHub(s, nil)
			assert.NoError(t, err)
			uh.Store = memFs

			uh.Controller = tc.controller

			nextState, _ := s.Handle(uh)
			assert.Equal(t, tc.expectedState, nextState)

			ptm.AssertExpectations(t)
		})
	}

	om.AssertExpectations(t)
}

func TestStateDownloaded(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	s := NewDownloadedState(m)

	assert.Equal(t, UpdateHubStateDownloaded, int(s.ID()))
	assert.Equal(t, m, s.UpdateMetadata())

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, err := newTestUpdateHub(NewIdleState(), aim)
	assert.NoError(t, err)

	expectedNextState := NewInstallingState(m, &ProgressTrackerImpl{}, uh.Store)

	nextState, _ := s.Handle(uh)
	assert.Equal(t, expectedNextState, nextState)

	om.AssertExpectations(t)
	aim.AssertExpectations(t)
}

func TestStateDownloadingToMap(t *testing.T) {
	ptm := &progresstrackermock.ProgressTrackerMock{}

	state := NewDownloadingState(&metadata.UpdateMetadata{}, ptm)

	ptm.On("GetProgress").Return(0).Once()
	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "downloading"
	expectedMap["progress"] = 0
	assert.Equal(t, expectedMap, state.ToMap())

	ptm.On("GetProgress").Return(45).Once()
	expectedMap["progress"] = 45
	assert.Equal(t, expectedMap, state.ToMap())

	ptm.AssertExpectations(t)
}

func TestNewPollState(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	state := NewPollState(time.Second)
	assert.IsType(t, &PollState{}, state)
	assert.Equal(t, UpdateHubState(UpdateHubStatePoll), state.ID())
	assert.Equal(t, time.Second, state.interval)

	aim.AssertExpectations(t)
}

func TestStatePollToMap(t *testing.T) {
	state := NewPollState(3 * time.Second)

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "poll"

	assert.Equal(t, expectedMap, state.ToMap())
}

func TestPollingRetries(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, err := newTestUpdateHub(nil, aim)
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
	uh.Settings.PollingInterval = time.Second
	uh.Settings.LastPoll = time.Now()

	uh.State = NewPollState(uh.Settings.PollingInterval)

	next, _ := uh.State.Handle(uh)
	assert.IsType(t, &UpdateCheckState{}, next)

	for i := 1; i < 3; i++ {
		state, _ := next.Handle(uh)
		assert.IsType(t, &IdleState{}, state)
		next, _ = state.Handle(uh)
		assert.IsType(t, &PollState{}, next)
		next, _ = next.Handle(uh)
		assert.IsType(t, &UpdateCheckState{}, next)
		assert.Equal(t, i, uh.Settings.PollingRetries)
	}

	c.updateAvailable = true
	c.extraPoll = 0

	next, _ = next.Handle(uh)
	assert.IsType(t, &DownloadingState{}, next)
	assert.Equal(t, 0, uh.Settings.PollingRetries)

	aim.AssertExpectations(t)
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
			aim := &activeinactivemock.ActiveInactiveMock{}

			uh, _ := newTestUpdateHub(nil, aim)

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

			uh.Settings.PollingInterval = tc.pollingInterval
			uh.Settings.FirstPoll = tc.firstPoll
			uh.Settings.LastPoll = tc.firstPoll

			uh.StartPolling()

			poll := uh.State
			assert.IsType(t, &PollState{}, poll)

			poll.Handle(uh)
			assert.Equal(t, tc.expectedElapsedTime, elapsed)

			aim.AssertExpectations(t)
		})
	}
}

func TestPollingWithPollingIntervalSmallerThanTimeStep(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(nil, aim)
	uh.TimeStep = time.Hour

	s := NewPollState(0)
	s.interval = time.Minute

	nextState, _ := s.Handle(uh)

	expectedState := NewErrorState(nil, NewTransientError(fmt.Errorf("Can't handle polling with invalid interval. It must be greater than '%s'", uh.TimeStep)))

	assert.Equal(t, expectedState, nextState)

	aim.AssertExpectations(t)
}

func TestCancelPollState(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(nil, aim)

	poll := NewPollState(uh.Settings.PollingInterval)
	poll.interval = 10 * time.Second

	go func() {
		assert.True(t, poll.Cancel(true, NewIdleState()))
	}()

	poll.Handle(uh)

	assert.Equal(t, int64(0), poll.ticksCount)

	aim.AssertExpectations(t)
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
			aim := &activeinactivemock.ActiveInactiveMock{}

			uh, err := newTestUpdateHub(NewIdleState(), aim)
			assert.NoError(t, err)

			uh.Settings = tc.settings

			go func() {
				uh.State.Cancel(false, NewIdleState()) // write
			}()

			next, _ := uh.State.Handle(uh) // read
			assert.IsType(t, tc.nextState, next)

			aim.AssertExpectations(t)

			expectedMap := map[string]interface{}{}
			expectedMap["status"] = "idle"

			assert.Equal(t, expectedMap, uh.State.ToMap())
		})
	}
}

func TestStateIdleToMap(t *testing.T) {
	state := NewIdleState()

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "idle"

	assert.Equal(t, expectedMap, state.ToMap())
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

type TestObject struct {
	metadata.Object
}

func TestStateInstalling(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	testCases := []struct {
		name               string
		controller         *testController
		expectedState      State
		expectedProgresses []int
	}{
		{
			"WithoutError",
			&testController{fetchUpdateError: nil, installUpdateError: nil, progressList: []int{33, 66, 99, 100}},
			NewInstalledState(m),
			[]int{33, 66, 99, 100},
		},

		{
			"WithError",
			&testController{fetchUpdateError: nil, installUpdateError: errors.New("install error"), progressList: []int{33}},
			NewErrorState(m, NewTransientError(errors.New("install error"))),
			[]int{33},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()

			ptm := &progresstrackermock.ProgressTrackerMock{}
			for _, p := range tc.expectedProgresses {
				ptm.On("SetProgress", p).Once()
			}

			s := NewInstallingState(m, ptm, memFs)

			uh, err := newTestUpdateHub(s, nil)
			assert.NoError(t, err)
			uh.Store = memFs

			uh.Controller = tc.controller

			nextState, _ := s.Handle(uh)
			assert.Equal(t, tc.expectedState, nextState)

			ptm.AssertExpectations(t)
		})
	}

	om.AssertExpectations(t)
}

func TestStateInstallingWithUpdateMetadataAlreadyInstalled(t *testing.T) {
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

	iidm := &installifdifferentmock.InstallIfDifferentMock{}

	ptm := &progresstrackermock.ProgressTrackerMock{}

	s := NewInstallingState(m, ptm, memFs)

	uh, err := newTestUpdateHub(s, aim)
	assert.NoError(t, err)

	uh.lastInstalledPackageUID = m.PackageUID()

	nextState, _ := s.Handle(uh)
	expectedState := NewWaitingForRebootState(m)
	assert.Equal(t, expectedState, nextState)

	uh.State = nextState

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
	iidm.AssertExpectations(t)
	ptm.AssertExpectations(t)
}

func TestStateInstallingToMap(t *testing.T) {
	ptm := &progresstrackermock.ProgressTrackerMock{}

	state := NewInstallingState(nil, ptm, nil)

	ptm.On("GetProgress").Return(0).Once()
	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "installing"
	expectedMap["progress"] = 0
	assert.Equal(t, expectedMap, state.ToMap())

	ptm.On("GetProgress").Return(45).Once()
	expectedMap["progress"] = 45
	assert.Equal(t, expectedMap, state.ToMap())

	ptm.AssertExpectations(t)
}

func TestStateWaitingForReboot(t *testing.T) {
	m := &metadata.UpdateMetadata{}
	s := NewWaitingForRebootState(m)

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, err := newTestUpdateHub(s, aim)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewIdleState()
	// we can't assert Equal here because NewPollState() creates a
	// channel dynamically
	assert.IsType(t, expectedState, nextState)

	assert.Equal(t, m, s.UpdateMetadata())

	aim.AssertExpectations(t)
}

func TestStateWaitingForRebootToMap(t *testing.T) {
	state := NewWaitingForRebootState(nil)

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "waiting-for-reboot"

	assert.Equal(t, expectedMap, state.ToMap())
}

func TestStateInstalled(t *testing.T) {
	m := &metadata.UpdateMetadata{}
	s := NewInstalledState(m)

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, err := newTestUpdateHub(s, aim)
	assert.NoError(t, err)

	nextState, _ := s.Handle(uh)
	expectedState := NewIdleState()
	// we can't assert Equal here because NewPollState() creates a
	// channel dynamically
	assert.IsType(t, expectedState, nextState)

	assert.Equal(t, m, s.UpdateMetadata())

	aim.AssertExpectations(t)
}

func TestStateInstalledToMap(t *testing.T) {
	state := NewInstalledState(nil)

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "installed"

	assert.Equal(t, expectedMap, state.ToMap())
}

func TestNewExitState(t *testing.T) {
	state := NewExitState(1)

	assert.IsType(t, &ExitState{}, state)
	assert.Equal(t, 1, state.exitCode)
}

func TestNewExitStateToMap(t *testing.T) {
	state := NewExitState(1)

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "exit"

	assert.Equal(t, expectedMap, state.ToMap())
}

func TestExitStateHandle(t *testing.T) {
	state := NewExitState(1)

	assert.Panics(t, func() {
		state.Handle(nil)
	})
}

func TestNewErrorStateToMap(t *testing.T) {
	state := NewErrorState(nil, NewTransientError(fmt.Errorf("error message")))

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "error"
	expectedMap["error"] = "transient error: error message"

	assert.Equal(t, expectedMap, state.ToMap())
}
