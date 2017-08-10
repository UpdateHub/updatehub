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

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/installifdifferentmock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/UpdateHub/updatehub/testsmocks/progresstrackermock"
	"github.com/UpdateHub/updatehub/testsmocks/statesmock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestStateInstalling(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	testCases := []struct {
		name               string
		controller         *testController
		expectedState      State
		expectedProgresses []int
	}{
		{
			"WithoutError",
			&testController{downloadUpdateError: nil, installUpdateError: nil, progressList: []int{33, 66, 99, 100}},
			NewInstalledState(m),
			[]int{33, 66, 99, 100},
		},

		{
			"WithError",
			&testController{downloadUpdateError: nil, installUpdateError: errors.New("install error"), progressList: []int{33}},
			NewErrorState(m, NewTransientError(errors.New("install error"))),
			[]int{33},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()

			ptm := &progresstrackermock.ProgressTrackerMock{}
			for _, p := range tc.expectedProgresses {
				ptm.On("SetProgress", p).Once()
			}

			s := NewInstallingState(m, ptm, memFs)

			uh, err := newTestUpdateHub(s, nil)
			assert.NoError(t, err)
			uh.Store = memFs

			uh.Controller = tc.controller

			nextState, _ := s.Handle(uh)
			assert.Equal(t, tc.expectedState, nextState)

			ptm.AssertExpectations(t)
		})
	}

	om.AssertExpectations(t)
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

	s := NewInstallingState(m, ptm, memFs)

	uh, err := newTestUpdateHub(s, aim)
	assert.NoError(t, err)

	uh.lastInstalledPackageUID = m.PackageUID()

	nextState, _ := s.Handle(uh)
	expectedState := NewWaitingForRebootState(m)
	assert.Equal(t, expectedState, nextState)

	uh.SetState(nextState)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
	iidm.AssertExpectations(t)
	ptm.AssertExpectations(t)
}

func TestStateInstallingToMap(t *testing.T) {
	ptm := &progresstrackermock.ProgressTrackerMock{}

	state := NewInstallingState(nil, ptm, nil)

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
