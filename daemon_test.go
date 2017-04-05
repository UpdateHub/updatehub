/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"testing"

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
