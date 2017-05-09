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
	"reflect"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/bouk/monkey"
	"github.com/stretchr/testify/assert"
)

func TestNewDaemon(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(nil, aim)
	d := NewDaemon(uh)

	assert.IsType(t, &Daemon{}, d)
	assert.Equal(t, uh, d.uh)

	aim.AssertExpectations(t)
}

func TestDaemonRun(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(nil, nil)
	d := NewDaemon(uh)

	state := NewStateTest(d)

	uh.State = state

	d.Run()

	assert.True(t, state.handled)
	assert.True(t, d.stop)

	aim.AssertExpectations(t)
}

func TestDaemonStop(t *testing.T) {
	d := NewDaemon(nil)

	d.Stop()

	assert.True(t, d.stop)
}

func TestDaemonFailedToReportStatus(t *testing.T) {
	logger, hook := test.NewNullLogger()

	defer hook.Reset()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(nil, aim)
	uh.Logger = logger

	d := NewDaemon(uh)

	uh.State = NewStateTest(d)

	defer monkey.PatchInstanceMethod(reflect.TypeOf(uh), "ReportCurrentState", func(uh *UpdateHub) error {
		return errors.New("")
	}).Unpatch()

	d.Run()

	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	assert.Equal(t, "Failed to report status", hook.LastEntry().Message)

	aim.AssertExpectations(t)
}

func TestDaemonExitStateStop(t *testing.T) {
	logger, hook := test.NewNullLogger()

	defer hook.Reset()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(nil, aim)
	uh.Logger = logger

	d := NewDaemon(uh)

	uh.State = NewErrorState(NewFatalError(errors.New("test")))

	assert.Equal(t, 1, d.Run())

	assert.IsType(t, &ExitState{}, uh.State)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	assert.Equal(t, "fatal error: test", hook.LastEntry().Message)
	assert.Equal(t, 1, uh.State.(*ExitState).exitCode)

	aim.AssertExpectations(t)
}

type StateTest struct {
	BaseState
	CancellableState

	handled bool
	d       *Daemon
}

func NewStateTest(d *Daemon) *StateTest {
	state := &StateTest{
		BaseState:        BaseState{id: UpdateHubDummyState},
		CancellableState: CancellableState{cancel: make(chan bool)},
		d:                d,
	}

	return state
}

func (state *StateTest) ID() UpdateHubState {
	return state.id
}

func (state *StateTest) Cancel(ok bool) bool {
	return state.CancellableState.Cancel(ok)
}

func (state *StateTest) Handle(uh *UpdateHub) (State, bool) {
	state.handled = true
	state.d.stop = true

	return state, false
}
