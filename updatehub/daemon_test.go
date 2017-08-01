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
	"testing"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/reportermock"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
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

	uh.SetState(state)

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

func TestDaemonExitStateStop(t *testing.T) {
	logger, hook := test.NewNullLogger()
	log.SetLogger(logger)

	defer log.SetLogger(logrus.StandardLogger())
	defer hook.Reset()

	aim := &activeinactivemock.ActiveInactiveMock{}
	rm := &reportermock.ReporterMock{}

	uh, _ := newTestUpdateHub(nil, aim)
	uh.Reporter = rm

	rm.On("ReportState", uh.API.Request(), "", "error", "err_msg", uh.FirmwareMetadata).Return(nil).Once()

	d := NewDaemon(uh)

	uh.SetState(NewErrorState(nil, NewFatalError(errors.New("err_msg"))))

	assert.Equal(t, 1, d.Run())

	assert.IsType(t, &ExitState{}, uh.GetState())
	assert.Equal(t, 1, len(hook.Entries))
	assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level)
	assert.Equal(t, "fatal error: err_msg", hook.LastEntry().Message)
	assert.Equal(t, 1, uh.GetState().(*ExitState).exitCode)

	aim.AssertExpectations(t)
	rm.AssertExpectations(t)
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

func (state *StateTest) Cancel(ok bool, nextState State) bool {
	return state.CancellableState.Cancel(ok, nextState)
}

func (state *StateTest) Handle(uh *UpdateHub) (State, bool) {
	state.handled = true
	state.d.stop = true

	return state, false
}
