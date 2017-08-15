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
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/UpdateHub/updatehub/testsmocks/progresstrackermock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestStateDownloading(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	memFs := afero.NewMemMapFs()

	apiClient := client.NewApiClient("address")

	testCases := []struct {
		name               string
		controller         *testController
		expectedState      State
		expectedProgresses []int
	}{
		{
			"WithoutError",
			&testController{downloadUpdateError: nil, installUpdateError: nil, progressList: []int{33, 66, 99, 100}},
			NewDownloadedState(apiClient, m),
			[]int{33, 66, 99, 100},
		},

		{
			"WithError",
			&testController{downloadUpdateError: errors.New("download error"), installUpdateError: nil, progressList: []int{33}},
			NewErrorState(apiClient, m, NewTransientError(errors.New("download error"))),
			[]int{33},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ptm := &progresstrackermock.ProgressTrackerMock{}
			for _, p := range tc.expectedProgresses {
				ptm.On("SetProgress", p).Once()
			}

			s := NewDownloadingState(apiClient, m, ptm)

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

func TestStateDownloadingToMap(t *testing.T) {
	ptm := &progresstrackermock.ProgressTrackerMock{}

	c := client.NewApiClient("address")

	state := NewDownloadingState(c, &metadata.UpdateMetadata{}, ptm)

	ptm.On("GetProgress").Return(0).Once()
	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "downloading"
	expectedMap["progress"] = 0
	assert.Equal(t, expectedMap, state.ToMap())

	ptm.On("GetProgress").Return(45).Once()
	expectedMap["progress"] = 45
	assert.Equal(t, expectedMap, state.ToMap())

	ptm.AssertExpectations(t)
}
