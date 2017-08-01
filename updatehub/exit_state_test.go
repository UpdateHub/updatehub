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

	"github.com/stretchr/testify/assert"
)

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
