/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"strings"
	"testing"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestStateInstalled(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	s := NewInstalledState(client.NewApiClient("address"), m)

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, err := newTestUpdateHub(s, aim)
	assert.NoError(t, err)

	uh.lastInstalledPackageUID = ""

	exists, err := afero.Exists(uh.Store, uh.Settings.RuntimeSettingsPath)
	assert.NoError(t, err)
	assert.False(t, exists)

	nextState, _ := s.Handle(uh)

	assert.IsType(t, &RebootingState{}, nextState)
	assert.Equal(t, m, s.UpdateMetadata())
	assert.Equal(t, s.UpdateMetadata().PackageUID(), uh.lastInstalledPackageUID)

	data, err := afero.ReadFile(uh.Store, uh.Settings.RuntimeSettingsPath)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "ProbeASAP=true"))

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateInstalledIgnoringProbeASAP(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	s := NewInstalledState(client.NewApiClient("address"), m)

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, err := newTestUpdateHub(s, aim)
	assert.NoError(t, err)

	uh.lastInstalledPackageUID = ""
	uh.IgnoreProbeASAP = true

	exists, err := afero.Exists(uh.Store, uh.Settings.RuntimeSettingsPath)
	assert.NoError(t, err)
	assert.False(t, exists)

	nextState, _ := s.Handle(uh)

	assert.IsType(t, &RebootingState{}, nextState)
	assert.Equal(t, m, s.UpdateMetadata())
	assert.Equal(t, s.UpdateMetadata().PackageUID(), uh.lastInstalledPackageUID)

	data, err := afero.ReadFile(uh.Store, uh.Settings.RuntimeSettingsPath)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "ProbeASAP=false"))

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateInstalledToMap(t *testing.T) {
	state := NewInstalledState(client.NewApiClient("address"), nil)

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "installed"

	assert.Equal(t, expectedMap, state.ToMap())
}
