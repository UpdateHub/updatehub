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

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/UpdateHub/updatehub/testsmocks/rebootermock"
	"github.com/stretchr/testify/assert"
)

func TestStateRebootingID(t *testing.T) {
	s := NewRebootingState(client.NewApiClient("address"), nil)

	assert.Equal(t, UpdateHubState(UpdateHubStateRebooting), s.ID())
}

func TestStateRebootingUpdateMetadata(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	s := NewRebootingState(client.NewApiClient("address"), m)

	assert.Equal(t, m, s.UpdateMetadata())

	om.AssertExpectations(t)
}

func TestStateRebootingHandle(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	rm := &rebootermock.RebooterMock{}
	rm.On("Reboot").Return(nil)

	s := NewRebootingState(client.NewApiClient("address"), m)

	uh, err := newTestUpdateHub(s, nil)
	assert.NoError(t, err)

	uh.Rebooter = rm

	s.Handle(uh)

	rm.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateRebootingHandleWithError(t *testing.T) {
	apiClient := client.NewApiClient("address")

	expectedError := fmt.Errorf("reboot error")

	rm := &rebootermock.RebooterMock{}
	rm.On("Reboot").Return(expectedError)

	s := NewRebootingState(apiClient, nil)

	uh, err := newTestUpdateHub(s, nil)
	assert.NoError(t, err)

	uh.Rebooter = rm

	nextState, cancelled := s.Handle(uh)

	expectedState := NewErrorState(apiClient, nil, NewTransientError(expectedError))

	assert.Equal(t, expectedState, nextState)
	assert.Equal(t, false, cancelled)

	rm.AssertExpectations(t)
}
