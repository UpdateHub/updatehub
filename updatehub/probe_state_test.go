/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	errs "github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/updatehub/updatehub/client"
	"github.com/updatehub/updatehub/installmodes"
	"github.com/updatehub/updatehub/metadata"
	"github.com/updatehub/updatehub/testsmocks/activeinactivemock"
	"github.com/updatehub/updatehub/testsmocks/controllermock"
	"github.com/updatehub/updatehub/testsmocks/objectmock"
)

func TestStateProbeWithUpdateAvailable(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	om := &objectmock.ObjectMock{}
	cm := &controllermock.ControllerMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	um, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	apiClient := client.NewApiClient("address")

	uh, err := newTestUpdateHub(NewProbeState(apiClient), aim)
	assert.NoError(t, err)

	uh.Controller = cm

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	sha256sum := sha256.Sum256([]byte(validJSONMetadata))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, testPrivateKey, crypto.SHA256, sha256sum[:])

	cm.On("ProbeUpdate", apiClient, 0).Return(um, signature, time.Duration(0), nil)

	next, _ := uh.GetState().Handle(uh)

	assert.Equal(t, NewDownloadingState(apiClient, um, &ProgressTrackerImpl{}), next)

	data, err := afero.ReadFile(uh.Store, uh.Settings.RuntimeSettingsPath)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "ProbeASAP=false"))
	assert.True(t, strings.Contains(string(data), "Retries=0"))
	assert.True(t, strings.Contains(string(data), "ExtraInterval=0"))
	// timestamps are relative to "Now()" so just test if they were written
	assert.True(t, strings.Contains(string(data), "FirstPoll="))
	assert.True(t, strings.Contains(string(data), "LastPoll="))

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestStateProbeWithUpdateNotAvailable(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &controllermock.ControllerMock{}

	apiClient := client.NewApiClient("address")

	uh, err := newTestUpdateHub(NewProbeState(apiClient), aim)
	assert.NoError(t, err)

	uh.Controller = cm

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	cm.On("ProbeUpdate", apiClient, 0).Return((*metadata.UpdateMetadata)(nil), []byte{}, time.Duration(0), nil)

	next, _ := uh.GetState().Handle(uh)

	assert.IsType(t, &IdleState{}, next)

	data, err := afero.ReadFile(uh.Store, uh.Settings.RuntimeSettingsPath)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "ProbeASAP=false"))
	assert.True(t, strings.Contains(string(data), "Retries=1"))
	assert.True(t, strings.Contains(string(data), "ExtraInterval=0"))
	// timestamps are relative to "Now()" so just test if they were written
	assert.True(t, strings.Contains(string(data), "FirstPoll="))
	assert.True(t, strings.Contains(string(data), "LastPoll="))

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestStateProbeWithExtraPoll(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &controllermock.ControllerMock{}

	apiClient := client.NewApiClient("address")

	uh, err := newTestUpdateHub(NewProbeState(apiClient), aim)
	assert.NoError(t, err)

	uh.Controller = cm
	uh.Settings = &Settings{
		PollingSettings: PollingSettings{
			PersistentPollingSettings: PersistentPollingSettings{
				FirstPoll: time.Now().Add(-5 * time.Second),
			},
			PollingInterval: 15 * time.Second,
		},
	}

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	cm.On("ProbeUpdate", apiClient, 0).Return((*metadata.UpdateMetadata)(nil), []byte{}, time.Duration(5*time.Second), nil)

	next, _ := uh.GetState().Handle(uh)

	assert.IsType(t, &PollState{}, next)
	poll := next.(*PollState)
	assert.Equal(t, 5*time.Second, poll.interval)
	assert.Equal(t, 5*time.Second, uh.Settings.ExtraPollingInterval)

	data, err := afero.ReadFile(uh.Store, uh.Settings.RuntimeSettingsPath)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "ProbeASAP=false"))
	assert.True(t, strings.Contains(string(data), "Retries=0"))
	assert.True(t, strings.Contains(string(data), "ExtraInterval=5000000")) // 5s
	// timestamps are relative to "Now()" so just test if they were written
	assert.True(t, strings.Contains(string(data), "FirstPoll="))
	assert.True(t, strings.Contains(string(data), "LastPoll="))

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestStateProbeWithUpdateAvailableButAlreadyInstalled(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &controllermock.ControllerMock{}

	apiClient := client.NewApiClient("address")

	uh, err := newTestUpdateHub(NewProbeState(apiClient), aim)
	assert.NoError(t, err)

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	uh.lastInstalledPackageUID = m.PackageUID()

	cm.On("ProbeUpdate", apiClient, 0).Return(m, []byte{}, time.Duration(0), nil)

	uh.Controller = cm
	uh.Settings = &Settings{}

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	next, _ := uh.GetState().Handle(uh)

	assert.IsType(t, &IdleState{}, next)

	data, err := afero.ReadFile(uh.Store, uh.Settings.RuntimeSettingsPath)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "ProbeASAP=false"))
	assert.True(t, strings.Contains(string(data), "Retries=0"))
	assert.True(t, strings.Contains(string(data), "ExtraInterval=0"))
	// timestamps are relative to "Now()" so just test if they were written
	assert.True(t, strings.Contains(string(data), "FirstPoll="))
	assert.True(t, strings.Contains(string(data), "LastPoll="))

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestStateProbeTimeout(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	om := &objectmock.ObjectMock{}
	cm := &controllermock.ControllerMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	um, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	apiClient := client.NewApiClient("address")
	apiClient.CheckRedirect = nil

	uh, err := newTestUpdateHub(NewProbeState(apiClient), aim)
	assert.NoError(t, err)

	uh.Controller = cm

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	sha256sum := sha256.Sum256([]byte(validJSONMetadata))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, testPrivateKey, crypto.SHA256, sha256sum[:])

	timeoutReached := false
	cm.On("ProbeUpdate", apiClient, 0).Run(func(args mock.Arguments) {
		timeoutReached = true
	}).Return((*metadata.UpdateMetadata)(nil), []byte{}, time.Duration(0), errs.Wrap(&testTimeoutError{}, "download update failed")).Once()
	cm.On("ProbeUpdate", apiClient, 1).Return(um, signature, time.Duration(0), nil).Once()

	next, _ := uh.GetState().Handle(uh)
	assert.IsType(t, &ProbeState{}, next)
	next, _ = uh.GetState().Handle(uh)

	assert.True(t, timeoutReached)

	assert.Equal(t, NewDownloadingState(apiClient, um, &ProgressTrackerImpl{}), next)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestStateProbeNotHavePendingDownload(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &controllermock.ControllerMock{}

	om1 := &objectmock.ObjectMock{}
	om2 := &objectmock.ObjectMock{}
	objs := []metadata.Object{om1, om2}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	um, err := metadata.NewUpdateMetadata([]byte(updateMetadataWithValidSha256sum))
	assert.NoError(t, err)

	apiClient := client.NewApiClient("address")

	uh, err := newTestUpdateHub(NewProbeState(apiClient), aim)
	assert.NoError(t, err)

	uh.Controller = cm

	uh.Store = afero.NewMemMapFs()

	for i, obj := range um.Objects[0] {
		objectUID := obj.GetObjectMetadata().Sha256sum
		afero.WriteFile(uh.Store, path.Join(uh.Settings.DownloadDir, objectUID), []byte(strconv.Itoa(i)), 0755)
	}

	sha256sum := sha256.Sum256([]byte(updateMetadataWithValidSha256sum))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, testPrivateKey, crypto.SHA256, sha256sum[:])

	cm.On("ProbeUpdate", apiClient, 0).Return(um, signature, time.Duration(0), nil)

	next, _ := uh.GetState().Handle(uh)

	assert.Equal(t, NewInstallingState(apiClient, um, &ProgressTrackerImpl{}, uh.Store), next)

	cm.AssertExpectations(t)
}

func TestStateProbeASAP(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	om := &objectmock.ObjectMock{}
	cm := &controllermock.ControllerMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	um, err := metadata.NewUpdateMetadata([]byte(validJSONMetadata))
	assert.NoError(t, err)

	apiClient := client.NewApiClient("address")

	uh, err := newTestUpdateHub(NewProbeState(apiClient), aim)
	assert.NoError(t, err)

	uh.Controller = cm

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	uh.Settings.ProbeASAP = true

	sha256sum := sha256.Sum256([]byte(validJSONMetadata))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, testPrivateKey, crypto.SHA256, sha256sum[:])

	cm.On("ProbeUpdate", apiClient, 0).Return(um, signature, time.Duration(0), nil)

	next, _ := uh.GetState().Handle(uh)

	assert.Equal(t, NewDownloadingState(apiClient, um, &ProgressTrackerImpl{}), next)

	data, err := afero.ReadFile(uh.Store, uh.Settings.RuntimeSettingsPath)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(data), "ProbeASAP=false"))
	assert.True(t, strings.Contains(string(data), "Retries=0"))
	assert.True(t, strings.Contains(string(data), "ExtraInterval=0"))
	// timestamps are relative to "Now()" so just test if they were written
	assert.True(t, strings.Contains(string(data), "FirstPoll="))
	assert.True(t, strings.Contains(string(data), "LastPoll="))

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestStateProbeToMap(t *testing.T) {
	state := NewProbeState(client.NewApiClient("address"))

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "probe"

	assert.Equal(t, expectedMap, state.ToMap())
}
