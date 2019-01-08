/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/controllermock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/bouk/monkey"
	"github.com/stretchr/testify/assert"
)

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
	om := &objectmock.ObjectMock{}
	cm := &controllermock.ControllerMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	uh, err := newTestUpdateHub(nil, aim)
	assert.NoError(t, err)

	apiClient := uh.DefaultApiClient

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

	cm.On("ProbeUpdate", apiClient, 0).Return((*metadata.UpdateMetadata)(nil), []byte{}, time.Duration(-1), nil).Once()
	cm.On("ProbeUpdate", apiClient, 1).Return((*metadata.UpdateMetadata)(nil), []byte{}, time.Duration(-1), nil).Once()
	cm.On("ProbeUpdate", apiClient, 2).Return((*metadata.UpdateMetadata)(nil), []byte{}, time.Duration(-1), nil).Once()

	uh.Controller = cm
	uh.Settings.PollingInterval = time.Second
	uh.Settings.LastPoll = time.Now()

	uh.SetState(NewPollState(uh.Settings.PollingInterval))

	next, _ := uh.GetState().Handle(uh)
	assert.IsType(t, &ProbeState{}, next)

	for i := 1; i < 3; i++ {
		state, _ := next.Handle(uh)
		assert.IsType(t, &IdleState{}, state)
		next, _ = state.Handle(uh)
		assert.IsType(t, &PollState{}, next)
		next, _ = next.Handle(uh)
		assert.IsType(t, &ProbeState{}, next)
		assert.Equal(t, i, uh.Settings.PollingRetries)
	}

	um, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	sha256sum := sha256.Sum256([]byte(validJSONMetadata))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, testPrivateKey, crypto.SHA256, sha256sum[:])

	cm.On("ProbeUpdate", apiClient, 3).Return(um, signature, time.Duration(0), nil).Once()

	state, _ := next.Handle(uh)
	assert.IsType(t, &IdleState{}, state)
	next, _ = state.Handle(uh)
	assert.IsType(t, &PollState{}, next)
	next, _ = next.Handle(uh)
	assert.IsType(t, &ProbeState{}, next)
	assert.Equal(t, 3, uh.Settings.PollingRetries)

	next, _ = next.Handle(uh)
	assert.IsType(t, &DownloadingState{}, next)
	assert.Equal(t, 0, uh.Settings.PollingRetries)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	cm.AssertExpectations(t)
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

			uh.Start()

			poll := uh.GetState()
			assert.IsType(t, &PollState{}, poll)

			poll.Handle(uh)
			assert.Equal(t, tc.expectedElapsedTime, elapsed)

			aim.AssertExpectations(t)
		})
	}
}

func TestPollingWithIntervalSmallerThanTimeStep(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(nil, aim)
	uh.TimeStep = time.Second

	s := NewPollState(0)
	s.interval = time.Second / 10

	nextState, _ := s.Handle(uh)

	assert.IsType(t, &ProbeState{}, nextState)
	assert.Equal(t, uh.TimeStep, s.interval)

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
