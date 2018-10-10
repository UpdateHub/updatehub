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
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
)

func TestStateDownloaded(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, err := newTestUpdateHub(NewIdleState(), aim)
	assert.NoError(t, err)

	s := NewDownloadedState(uh.DefaultApiClient, m)

	assert.Equal(t, UpdateHubStateDownloaded, int(s.ID()))
	assert.Equal(t, m, s.UpdateMetadata())

	expectedNextState := NewInstallingState(uh.DefaultApiClient, m, &ProgressTrackerImpl{}, uh.Store)

	nextState, _ := s.Handle(uh)
	assert.Equal(t, expectedNextState, nextState)

	om.AssertExpectations(t)
	aim.AssertExpectations(t)
}
