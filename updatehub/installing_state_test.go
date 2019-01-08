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

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/controllermock"
	"github.com/UpdateHub/updatehub/testsmocks/installifdifferentmock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/UpdateHub/updatehub/testsmocks/progresstrackermock"
	"github.com/UpdateHub/updatehub/testsmocks/statesmock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStateInstallingWithSuccess(t *testing.T) {
	om := &objectmock.ObjectMock{}
	cm := &controllermock.ControllerMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	apiClient := client.NewApiClient("address")

	expectedState := NewInstalledState(apiClient, m)
	expectedProgresses := []int{33, 66, 99, 100}

	memFs := afero.NewMemMapFs()

	ptm := &progresstrackermock.ProgressTrackerMock{}
	for _, p := range expectedProgresses {
		ptm.On("SetProgress", p).Once()
	}

	cm.On("InstallUpdate", m, mock.AnythingOfType("chan<- int")).Run(func(args mock.Arguments) {
		progressChan := args.Get(1).(chan<- int)

		for _, p := range expectedProgresses {
			// "non-blocking" write to channel
			select {
			case progressChan <- p:
			default:
			}
		}
	}).Return(nil)

	s := NewInstallingState(apiClient, m, ptm, memFs)

	uh, err := newTestUpdateHub(s, nil)
	assert.NoError(t, err)
	uh.Store = memFs

	uh.Controller = cm

	nextState, _ := s.Handle(uh)
	assert.Equal(t, expectedState, nextState)

	ptm.AssertExpectations(t)
	om.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestStateInstallingWithError(t *testing.T) {
	om := &objectmock.ObjectMock{}
	cm := &controllermock.ControllerMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	apiClient := client.NewApiClient("address")

	expectedState := NewErrorState(apiClient, m, NewTransientError(errors.New("install error")))
	expectedProgresses := []int{33}

	memFs := afero.NewMemMapFs()

	ptm := &progresstrackermock.ProgressTrackerMock{}
	for _, p := range expectedProgresses {
		ptm.On("SetProgress", p).Once()
	}

	cm.On("InstallUpdate", m, mock.AnythingOfType("chan<- int")).Run(func(args mock.Arguments) {
		progressChan := args.Get(1).(chan<- int)

		for _, p := range expectedProgresses {
			// "non-blocking" write to channel
			select {
			case progressChan <- p:
			default:
			}
		}
	}).Return(errors.New("install error"))

	s := NewInstallingState(apiClient, m, ptm, memFs)

	uh, err := newTestUpdateHub(s, nil)
	assert.NoError(t, err)
	uh.Store = memFs

	uh.Controller = cm

	nextState, _ := s.Handle(uh)
	assert.Equal(t, expectedState, nextState)

	ptm.AssertExpectations(t)
	om.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestStateInstallingWithUpdateMetadataAlreadyInstalled(t *testing.T) {
	memFs := afero.NewMemMapFs()

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

	scm := &statesmock.Sha256CheckerMock{}

	iidm := &installifdifferentmock.InstallIfDifferentMock{}

	ptm := &progresstrackermock.ProgressTrackerMock{}

	apiClient := client.NewApiClient("address")

	s := NewInstallingState(apiClient, m, ptm, memFs)

	uh, err := newTestUpdateHub(s, aim)
	assert.NoError(t, err)

	uh.lastInstalledPackageUID = m.PackageUID()

	nextState, _ := s.Handle(uh)

	assert.IsType(t, &IdleState{}, nextState)

	uh.SetState(nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
	iidm.AssertExpectations(t)
	ptm.AssertExpectations(t)
}

func TestStateInstallingToMap(t *testing.T) {
	ptm := &progresstrackermock.ProgressTrackerMock{}

	state := NewInstallingState(client.NewApiClient("address"), nil, ptm, nil)

	ptm.On("GetProgress").Return(0).Once()
	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "installing"
	expectedMap["progress"] = 0
	assert.Equal(t, expectedMap, state.ToMap())

	ptm.On("GetProgress").Return(45).Once()
	expectedMap["progress"] = 45
	assert.Equal(t, expectedMap, state.ToMap())

	ptm.AssertExpectations(t)
}
