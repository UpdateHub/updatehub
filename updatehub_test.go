/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/bouk/monkey"
	"github.com/go-ini/ini"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/UpdateHub/updatehub/activeinactive"
	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/filemock"
	"github.com/UpdateHub/updatehub/testsmocks/updatermock"
	"github.com/UpdateHub/updatehub/utils"
)

const (
	validUpdateMetadata = `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "test", "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" }
    ]
  ]
}`

	validUpdateMetadataWithActiveInactive = `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "test", "target": "/dev/xxa1", "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" },
      { "mode": "test", "target": "/dev/xxa2", "sha256sum": "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb" }
    ]
    ,
    [
      { "mode": "test", "target": "/dev/xxb1", "sha256sum": "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa" },
      { "mode": "test", "target": "/dev/xxb2", "sha256sum": "b9632efa90820ff35d6cec0946f99bb8a6317b1e2ef877e501a3e12b2d04d0ae" }
    ]
  ]
}`

	updateMetadataWithNoObjects = `{
  "product-uid": "123",
  "objects": [
  ]
}`
)

func TestUpdateHubCheckUpdate(t *testing.T) {
	testCases := []struct {
		name           string
		updateMetadata string
		extraPoll      time.Duration
		err            error
	}{
		{
			"InvalidUpdateMetadata",
			"",
			0,
			nil,
		},

		{
			"ValidUpdateMetadata",
			validUpdateMetadata,
			13,
			nil,
		},
	}

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedUpdateMetadata, _ := metadata.NewUpdateMetadata([]byte(tc.updateMetadata))

			updater := testUpdater{
				updateMetadata: expectedUpdateMetadata,
				extraPoll:      tc.extraPoll,
			}

			uh.updater = client.Updater(updater)

			updateMetadata, extraPoll := uh.CheckUpdate(0)

			assert.Equal(t, expectedUpdateMetadata, updateMetadata)
			assert.Equal(t, tc.extraPoll, extraPoll)
		})
	}

	aim.AssertExpectations(t)
}

func TestUpdateHubFetchUpdate(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	updater := testUpdater{
		updateMetadata: updateMetadata,
		extraPoll:      0,
		updateBytes:    []byte("0123456789"),
	}

	uh.updater = client.Updater(updater)

	err = uh.FetchUpdate(updateMetadata, nil)
	assert.NoError(t, err)

	data, err := afero.ReadFile(uh.store, path.Join(uh.settings.DownloadDir, updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum))
	assert.NoError(t, err)
	assert.Equal(t, updater.updateBytes, data)

	aim.AssertExpectations(t)
}

func TestUpdateHubFetchUpdateWithActiveInactive(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(0, nil)

	uh, _ := newTestUpdateHub(&PollState{}, aim)
	uh.firmwareMetadata.ProductUID = "148de9c5a7a44d19e56cd9ae1a554bf67847afb0c58f6e12fa29ac7ddfca9940"

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadataWithActiveInactive))

	expectedURIPrefix := "/"
	expectedURIPrefix = path.Join(expectedURIPrefix, uh.firmwareMetadata.ProductUID)
	expectedURIPrefix = path.Join(expectedURIPrefix, packageUID)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadataWithActiveInactive))
	assert.NoError(t, err)

	um := &updatermock.UpdaterMock{}

	// download of file 1 setup
	file1Content := []byte("content1") // this matches with the sha256sum in "validUpdateMetadataWithActiveInactive"

	fm1 := &filemock.FileMock{}
	fm1.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		copy(arg, file1Content)
	}).Return(len(file1Content), nil).Once()
	fm1.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Once()

	objectUIDFirst := updateMetadata.Objects[1][0].GetObjectMetadata().Sha256sum
	uri1 := path.Join(expectedURIPrefix, objectUIDFirst)
	um.On("FetchUpdate", uh.api.Request(), uri1).Return(fm1, int64(len(file1Content)), nil)

	// download of file 2 setup
	file2Content := []byte("content2butbigger") // this matches with the sha256sum in "validUpdateMetadataWithActiveInactive"

	fm2 := &filemock.FileMock{}
	fm2.On("Read", mock.AnythingOfType("[]uint8")).Run(func(args mock.Arguments) {
		arg := args.Get(0).([]uint8)
		copy(arg, file2Content)
	}).Return(len(file2Content), nil).Once()
	fm2.On("Read", mock.AnythingOfType("[]uint8")).Return(0, io.EOF).Once()

	objectUIDSecond := updateMetadata.Objects[1][1].GetObjectMetadata().Sha256sum
	uri2 := path.Join(expectedURIPrefix, objectUIDSecond)
	um.On("FetchUpdate", uh.api.Request(), uri2).Return(fm2, int64(len(file2Content)), nil)

	// finish setup
	uh.updater = um

	err = uh.FetchUpdate(updateMetadata, nil)
	assert.NoError(t, err)

	// since the "Active()" returned "0", we are expecting the "1"
	// (inactive) files to be downloaded
	data, err := afero.ReadFile(uh.store, path.Join(uh.settings.DownloadDir, objectUIDFirst))
	assert.NoError(t, err)
	assert.Equal(t, file1Content, data)

	data, err = afero.ReadFile(uh.store, path.Join(uh.settings.DownloadDir, objectUIDSecond))
	assert.NoError(t, err)
	assert.Equal(t, file2Content, data)

	// and the "0" (active) files to NOT be downloaded
	fileExists, err := afero.Exists(uh.store, path.Join(uh.settings.DownloadDir, updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum))
	assert.NoError(t, err)
	assert.False(t, fileExists)

	fileExists, err = afero.Exists(uh.store, path.Join(uh.settings.DownloadDir, updateMetadata.Objects[0][1].GetObjectMetadata().Sha256sum))
	assert.NoError(t, err)
	assert.False(t, fileExists)

	aim.AssertExpectations(t)
	um.AssertExpectations(t)
	fm1.AssertExpectations(t)
	fm2.AssertExpectations(t)
}

func TestUpdateHubFetchUpdateWithActiveInactiveError(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(updateMetadataWithNoObjects))
	assert.NoError(t, err)

	um := &updatermock.UpdaterMock{}
	uh.updater = um

	err = uh.FetchUpdate(updateMetadata, nil)
	assert.EqualError(t, err, "update metadata must have 1 or 2 objects. Found 0")

	aim.AssertExpectations(t)
	um.AssertExpectations(t)
}

func TestUpdateHubReportState(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	state := &testReportableState{}
	state.updateMetadata = updateMetadata

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(state, aim)
	uh.reporter = client.Reporter(testReporter{})

	err = uh.ReportCurrentState()
	assert.NoError(t, err)

	uh.reporter = client.Reporter(testReporter{reportStateError: errors.New("error")})

	err = uh.ReportCurrentState()
	assert.Error(t, err)
	assert.EqualError(t, err, "error")

	aim.AssertExpectations(t)
}

func TestStartPolling(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name                 string
		pollingInterval      time.Duration
		extraPollingInterval time.Duration
		firstPoll            time.Time
		lastPoll             time.Time
		expectedState        State
		subTest              func(t *testing.T, uh *UpdateHub, state State)
	}{
		{
			"RegularPoll",
			time.Second,
			0,
			(time.Time{}).UTC(),
			(time.Time{}).UTC(),
			&PollState{},
			func(t *testing.T, uh *UpdateHub, state State) {},
		},

		{
			"NeverDidPollBefore",
			time.Second,
			0,
			now.Add(-1 * time.Second),
			(time.Time{}).UTC(),
			&UpdateCheckState{},
			func(t *testing.T, uh *UpdateHub, state State) {},
		},

		{
			"PendingRegularPoll",
			time.Second,
			0,
			now.Add(-4 * time.Second),
			now.Add(-2 * time.Second),
			&UpdateCheckState{},
			func(t *testing.T, uh *UpdateHub, state State) {},
		},

		{
			"PendingExtraPoll",
			10 * time.Second,
			3 * time.Second,
			now.Add(-25 * time.Second),
			now.Add(-5 * time.Second),
			&PollState{},
			func(t *testing.T, uh *UpdateHub, state State) {
				poll := state.(*PollState)
				assert.Equal(t, 3*time.Second, poll.interval)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate time passage from now
			defer func() *monkey.PatchGuard {
				seconds := -1
				return monkey.Patch(time.Now, func() time.Time {
					seconds++
					return now.Add(time.Second * time.Duration(seconds))
				})
			}().Unpatch()

			aim := &activeinactivemock.ActiveInactiveMock{}

			uh, _ := newTestUpdateHub(nil, aim)

			uh.settings.PollingInterval = tc.pollingInterval
			uh.settings.ExtraPollingInterval = tc.extraPollingInterval
			uh.settings.FirstPoll = tc.firstPoll
			uh.settings.LastPoll = tc.lastPoll

			uh.StartPolling()
			assert.IsType(t, tc.expectedState, uh.state)

			tc.subTest(t, uh, uh.state)

			aim.AssertExpectations(t)
		})
	}
}

func TestLoadUpdateHubSettings(t *testing.T) {
	testCases := []struct {
		name            string
		systemSettings  string
		runtimeSettings string
		expectedError   interface{}
		subTest         func(t *testing.T, settings *Settings, err error)
	}{
		{
			"SystemSettingsNotFound",
			"",
			"",
			&os.PathError{},
			func(t *testing.T, settings *Settings, err error) {
				assert.Equal(t, err.(*os.PathError).Path, systemSettingsPath)
			},
		},

		{
			"RuntimeSettingsNotFound",
			"[Polling]\nEnabled=true",
			"",
			&os.PathError{},
			func(t *testing.T, settings *Settings, err error) {
				assert.Equal(t, err.(*os.PathError).Path, runtimeSettingsPath)
			},
		},

		{
			"InvalidSettingsFile",
			"test",
			"test",
			ini.ErrDelimiterNotFound{},
			func(t *testing.T, settings *Settings, err error) {
			},
		},

		{
			"ValidSettingsFile",
			"[Polling]\nEnabled=true",
			"[Polling]\nExtraInterval=1",
			nil,
			func(t *testing.T, settings *Settings, err error) {
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			aim := &activeinactivemock.ActiveInactiveMock{}

			uh, _ := newTestUpdateHub(nil, aim)

			if tc.systemSettings != "" {
				err := uh.store.MkdirAll(filepath.Dir(systemSettingsPath), 0755)
				assert.NoError(t, err)
				err = afero.WriteFile(uh.store, systemSettingsPath, []byte(tc.systemSettings), 0644)
				assert.NoError(t, err)
			}

			if tc.runtimeSettings != "" {
				err := uh.store.MkdirAll(filepath.Dir(runtimeSettingsPath), 0755)
				assert.NoError(t, err)
				err = afero.WriteFile(uh.store, runtimeSettingsPath, []byte(tc.runtimeSettings), 0644)
				assert.NoError(t, err)
			}

			err := uh.LoadSettings()
			assert.IsType(t, tc.expectedError, err)

			tc.subTest(t, uh.settings, err)

			aim.AssertExpectations(t)
		})
	}
}

type testObject struct {
	metadata.ObjectMetadata
}

type testUpdater struct {
	// CheckUpdate
	updateMetadata   *metadata.UpdateMetadata
	extraPoll        time.Duration
	checkUpdateError error
	// FetchUpdate
	updateBytes      []byte
	fetchUpdateError error
}

func (t testUpdater) CheckUpdate(api client.ApiRequester, data interface{}) (interface{}, time.Duration, error) {
	return t.updateMetadata, t.extraPoll, t.checkUpdateError
}

func (t testUpdater) FetchUpdate(api client.ApiRequester, uri string) (io.ReadCloser, int64, error) {
	rd := bytes.NewReader(t.updateBytes)
	return ioutil.NopCloser(rd), int64(len(t.updateBytes)), t.fetchUpdateError
}

type testReporter struct {
	reportStateError error
}

func (r testReporter) ReportState(api client.ApiRequester, packageUID string, state string) error {
	return r.reportStateError
}

func newTestInstallMode() installmodes.InstallMode {
	return installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return &testObject{} },
	})
}

func newTestUpdateHub(state State, aii activeinactive.Interface) (*UpdateHub, error) {
	uh := &UpdateHub{
		store:    afero.NewMemMapFs(),
		state:    state,
		timeStep: time.Second,
		api:      client.NewApiClient("localhost"),
		activeInactiveBackend: aii,
	}

	settings, err := LoadSettings(bytes.NewReader([]byte("")))
	if err != nil {
		return nil, err
	}

	uh.settings = settings
	uh.settings.PollingInterval = 1

	return uh, err
}
