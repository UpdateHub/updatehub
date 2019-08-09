/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package updatehub

import (
	"errors"
	"testing"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/controllermock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/UpdateHub/updatehub/testsmocks/progresstrackermock"
	errs "github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStateDownloadingWithSuccess(t *testing.T) {
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

	memFs := afero.NewMemMapFs()

	apiClient := client.NewApiClient("address")

	expectedState := NewDownloadedState(apiClient, m)
	expectedProgresses := []int{33, 66, 99, 100}

	ptm := &progresstrackermock.ProgressTrackerMock{}
	for _, p := range expectedProgresses {
		ptm.On("SetProgress", p).Once()
	}

	cm.On("DownloadUpdate", apiClient, m, mock.Anything, mock.AnythingOfType("chan<- int")).Run(func(args mock.Arguments) {
		progressChan := args.Get(3).(chan<- int)

		for _, p := range expectedProgresses {
			// "non-blocking" write to channel
			select {
			case progressChan <- p:
			default:
			}
		}
	}).Return(nil)

	s := NewDownloadingState(apiClient, m, ptm)

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

func TestStateDownloadingWithError(t *testing.T) {
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

	memFs := afero.NewMemMapFs()

	apiClient := client.NewApiClient("address")

	expectedState := NewErrorState(apiClient, m, NewTransientError(errors.New("download error")))
	expectedProgresses := []int{33}

	ptm := &progresstrackermock.ProgressTrackerMock{}
	for _, p := range expectedProgresses {
		ptm.On("SetProgress", p).Once()
	}

	cm.On("DownloadUpdate", apiClient, m, mock.Anything, mock.AnythingOfType("chan<- int")).Run(func(args mock.Arguments) {
		progressChan := args.Get(3).(chan<- int)

		for _, p := range expectedProgresses {
			// "non-blocking" write to channel
			select {
			case progressChan <- p:
			default:
			}
		}
	}).Return(errors.New("download error"))

	s := NewDownloadingState(apiClient, m, ptm)

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

func TestStateDownloadingTimeout(t *testing.T) {
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

	apiClient := client.NewApiClient("localhost")

	s := NewDownloadingState(apiClient, m, nil)

	uh, err := newTestUpdateHub(s, nil)
	assert.NoError(t, err)

	uh.Store = memFs

	timeoutReached := false

	cm := &controllermock.ControllerMock{}
	cm.On("DownloadUpdate", apiClient, m, mock.Anything, mock.AnythingOfType("chan<- int")).Run(func(args mock.Arguments) {
		timeoutReached = true
	}).Return(errs.Wrap(&testTimeoutError{}, "download update failed")).Once()
	cm.On("DownloadUpdate", apiClient, m, mock.Anything, mock.AnythingOfType("chan<- int")).Return(nil).Once()

	uh.Controller = cm

	nextState, _ := s.Handle(uh)

	assert.True(t, timeoutReached)

	expectedState := NewDownloadedState(apiClient, m)
	assert.Equal(t, expectedState, nextState)
}

func TestStateDownloadingSha256Error(t *testing.T) {
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

	apiClient := client.NewApiClient("localhost")

	s := NewDownloadingState(apiClient, m, nil)

	uh, err := newTestUpdateHub(s, nil)
	assert.NoError(t, err)

	uh.Store = memFs

	cm := &controllermock.ControllerMock{}
	cm.On("DownloadUpdate", apiClient, m, mock.Anything, mock.AnythingOfType("chan<- int")).Return(ErrSha256sum).Once()
	cm.On("DownloadUpdate", apiClient, m, mock.Anything, mock.AnythingOfType("chan<- int")).Return(nil).Once()

	uh.Controller = cm

	nextState, _ := s.Handle(uh)

	expectedState := NewDownloadedState(apiClient, m)
	assert.Equal(t, expectedState, nextState)

	cm.AssertExpectations(t)
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

type testTimeoutError struct {
	error
}

func (e *testTimeoutError) Timeout() bool {
	return true
}

func (e *testTimeoutError) Temporary() bool {
	return false
}

func (e *testTimeoutError) Error() string {
	return "timeout error"
}
