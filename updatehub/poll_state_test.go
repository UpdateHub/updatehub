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

	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
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

	uh.SetState(NewPollState(uh.Settings.PollingInterval))

	next, _ := uh.GetState().Handle(uh)
	assert.IsType(t, &UpdateProbeState{}, next)

	for i := 1; i < 3; i++ {
		state, _ := next.Handle(uh)
		assert.IsType(t, &IdleState{}, state)
		next, _ = state.Handle(uh)
		assert.IsType(t, &PollState{}, next)
		next, _ = next.Handle(uh)
		assert.IsType(t, &UpdateProbeState{}, next)
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

	expectedState := NewUpdateProbeState()

	assert.Equal(t, expectedState, nextState)
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
