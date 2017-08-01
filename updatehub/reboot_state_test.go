/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"fmt"
	"testing"

	"github.com/UpdateHub/updatehub/testsmocks/rebootermock"
	"github.com/stretchr/testify/assert"
)

func TestStateRebootID(t *testing.T) {
	s := NewRebootState()

	assert.Equal(t, UpdateHubState(UpdateHubStateReboot), s.ID())
}

func TestStateRebootHandle(t *testing.T) {
	rm := &rebootermock.RebooterMock{}
	rm.On("Reboot").Return(nil)

	s := NewRebootState()

	uh, err := newTestUpdateHub(s, nil)
	assert.NoError(t, err)

	uh.Rebooter = rm

	s.Handle(uh)

	rm.AssertExpectations(t)
}

func TestStateRebootHandleWithError(t *testing.T) {
	expectedError := fmt.Errorf("reboot error")

	rm := &rebootermock.RebooterMock{}
	rm.On("Reboot").Return(expectedError)

	s := NewRebootState()

	uh, err := newTestUpdateHub(s, nil)
	assert.NoError(t, err)

	uh.Rebooter = rm

	nextState, cancelled := s.Handle(uh)

	expectedState := NewErrorState(nil, NewTransientError(expectedError))

	assert.Equal(t, expectedState, nextState)
	assert.Equal(t, false, cancelled)

	rm.AssertExpectations(t)
}
