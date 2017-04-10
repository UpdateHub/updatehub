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
	"reflect"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/test"
	"github.com/bouk/monkey"
	"github.com/stretchr/testify/assert"
)

func TestNewDaemon(t *testing.T) {
	uh, _ := newTestUpdateHub(nil)
	d := NewDaemon(uh)

	assert.IsType(t, &Daemon{}, d)
	assert.Equal(t, uh, d.uh)
}

func TestDaemonRun(t *testing.T) {
	uh, _ := newTestUpdateHub(nil)
	d := NewDaemon(uh)

	state := NewStateTest(d)

	uh.state = state

	d.Run()

	assert.True(t, state.handled)
	assert.True(t, d.stop)
}

func TestDaemonStop(t *testing.T) {
	d := NewDaemon(nil)

	d.Stop()

	assert.True(t, d.stop)
}

func TestDaemonFailedToReportStatus(t *testing.T) {
	logger, hook := test.NewNullLogger()

	defer hook.Reset()

	uh, _ := newTestUpdateHub(nil)
	uh.logger = logger

	d := NewDaemon(uh)

	uh.state = NewStateTest(d)

	defer monkey.PatchInstanceMethod(reflect.TypeOf(uh), "ReportCurrentState", func(uh *UpdateHub) error {
		return errors.New("")
	}).Unpatch()

	d.Run()

	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	assert.Equal(t, "Failed to report status", hook.LastEntry().Message)
}

func TestDaemonExitStateStop(t *testing.T) {
	logger, hook := test.NewNullLogger()

	defer hook.Reset()

	uh, _ := newTestUpdateHub(nil)
	uh.logger = logger

	d := NewDaemon(uh)

	uh.state = NewErrorState(NewFatalError(errors.New("test")))

	assert.Equal(t, 1, d.Run())

	assert.IsType(t, &ExitState{}, uh.state)
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	assert.Equal(t, "fatal error: test", hook.LastEntry().Message)
	assert.Equal(t, 1, uh.state.(*ExitState).exitCode)
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
