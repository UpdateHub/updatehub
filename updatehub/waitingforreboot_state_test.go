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

	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/stretchr/testify/assert"
)

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
