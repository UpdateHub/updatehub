/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"bytes"
	"errors"
	"fmt"
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

	"github.com/UpdateHub/updatehub/activeinactive"
	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/copymock"
	"github.com/UpdateHub/updatehub/testsmocks/filemock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
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

			var data struct {
				Retries int `json:"retries"`
				metadata.FirmwareMetadata
			}

			data.FirmwareMetadata = uh.FirmwareMetadata
			data.Retries = 0

			um := &updatermock.UpdaterMock{}
			um.On("CheckUpdate", uh.API.Request(), client.UpgradesEndpoint, data).Return(expectedUpdateMetadata, tc.extraPoll, nil)

			uh.Updater = um

			updateMetadata, extraPoll := uh.CheckUpdate(0)

			assert.Equal(t, expectedUpdateMetadata, updateMetadata)
			assert.Equal(t, tc.extraPoll, extraPoll)
			um.AssertExpectations(t)
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

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadata))
	objectUID := updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum

	uri := path.Join("/", uh.FirmwareMetadata.ProductUID, packageUID, objectUID)

	source := &filemock.FileMock{}
	source.On("Close").Return(nil)
	sourceContent := []byte("content")

	um := &updatermock.UpdaterMock{}
	um.On("FetchUpdate", uh.API.Request(), uri).Return(source, int64(len(sourceContent)), nil)
	uh.Updater = um

	// setup filesystembackend

	target := &filemock.FileMock{}
	target.On("Close").Return(nil)

	cpm := &copymock.CopyMock{}
	cpm.On("Copy", target, source, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil)
	uh.CopyBackend = cpm

	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Create", path.Join(uh.settings.DownloadDir, objectUID)).Return(target, nil)
	uh.Store = fsm

	err = uh.FetchUpdate(updateMetadata, nil)
	assert.NoError(t, err)

	aim.AssertExpectations(t)
	cpm.AssertExpectations(t)
	target.AssertExpectations(t)
	source.AssertExpectations(t)
	fsm.AssertExpectations(t)
	um.AssertExpectations(t)
}

func TestUpdateHubFetchUpdateWithTargetFileError(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	um := &updatermock.UpdaterMock{}
	uh.Updater = um

	// setup filesystembackend
	objectUID := updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum

	cpm := &copymock.CopyMock{}
	uh.CopyBackend = cpm

	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Create", path.Join(uh.settings.DownloadDir, objectUID)).Return((*filemock.FileMock)(nil), fmt.Errorf("create error"))
	uh.Store = fsm

	err = uh.FetchUpdate(updateMetadata, nil)
	assert.EqualError(t, err, "create error")

	aim.AssertExpectations(t)
	cpm.AssertExpectations(t)
	fsm.AssertExpectations(t)
	um.AssertExpectations(t)
}

func TestUpdateHubFetchUpdateWithUpdaterError(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadata))
	objectUID := updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum

	uri := path.Join("/", uh.FirmwareMetadata.ProductUID, packageUID, objectUID)

	source := &filemock.FileMock{}

	um := &updatermock.UpdaterMock{}
	um.On("FetchUpdate", uh.API.Request(), uri).Return(source, int64(0), fmt.Errorf("updater error"))
	uh.Updater = um

	// setup filesystembackend

	target := &filemock.FileMock{}
	target.On("Close").Return(nil)

	cpm := &copymock.CopyMock{}
	uh.CopyBackend = cpm

	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Create", path.Join(uh.settings.DownloadDir, objectUID)).Return(target, nil)
	uh.Store = fsm

	err = uh.FetchUpdate(updateMetadata, nil)
	assert.EqualError(t, err, "updater error")

	aim.AssertExpectations(t)
	cpm.AssertExpectations(t)
	target.AssertExpectations(t)
	source.AssertExpectations(t)
	fsm.AssertExpectations(t)
	um.AssertExpectations(t)
}

func TestUpdateHubFetchUpdateWithCopyError(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadata))
	objectUID := updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum

	uri := path.Join("/", uh.FirmwareMetadata.ProductUID, packageUID, objectUID)

	source := &filemock.FileMock{}
	source.On("Close").Return(nil)
	sourceContent := []byte("content")

	um := &updatermock.UpdaterMock{}
	um.On("FetchUpdate", uh.API.Request(), uri).Return(source, int64(len(sourceContent)), nil)
	uh.Updater = um

	// setup filesystembackend

	target := &filemock.FileMock{}
	target.On("Close").Return(nil)

	cpm := &copymock.CopyMock{}
	cpm.On("Copy", target, source, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, fmt.Errorf("copy error"))
	uh.CopyBackend = cpm

	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Create", path.Join(uh.settings.DownloadDir, objectUID)).Return(target, nil)
	uh.Store = fsm

	err = uh.FetchUpdate(updateMetadata, nil)
	assert.EqualError(t, err, "copy error")

	aim.AssertExpectations(t)
	cpm.AssertExpectations(t)
	target.AssertExpectations(t)
	source.AssertExpectations(t)
	fsm.AssertExpectations(t)
	um.AssertExpectations(t)
}

func TestUpdateHubFetchUpdateWithActiveInactive(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(0, nil)

	uh, _ := newTestUpdateHub(&PollState{}, aim)
	uh.FirmwareMetadata.ProductUID = "148de9c5a7a44d19e56cd9ae1a554bf67847afb0c58f6e12fa29ac7ddfca9940"

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadataWithActiveInactive))

	expectedURIPrefix := "/"
	expectedURIPrefix = path.Join(expectedURIPrefix, uh.FirmwareMetadata.ProductUID)
	expectedURIPrefix = path.Join(expectedURIPrefix, packageUID)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadataWithActiveInactive))
	assert.NoError(t, err)

	um := &updatermock.UpdaterMock{}

	// download of file 1 setup
	file1Content := []byte("content1") // this matches with the sha256sum in "validUpdateMetadataWithActiveInactive"

	source1 := &filemock.FileMock{}
	source1.On("Close").Return(nil)

	objectUIDFirst := updateMetadata.Objects[1][0].GetObjectMetadata().Sha256sum
	uri1 := path.Join(expectedURIPrefix, objectUIDFirst)
	um.On("FetchUpdate", uh.API.Request(), uri1).Return(source1, int64(len(file1Content)), nil)

	// download of file 2 setup
	file2Content := []byte("content2butbigger") // this matches with the sha256sum in "validUpdateMetadataWithActiveInactive"

	source2 := &filemock.FileMock{}
	source2.On("Close").Return(nil)

	objectUIDSecond := updateMetadata.Objects[1][1].GetObjectMetadata().Sha256sum
	uri2 := path.Join(expectedURIPrefix, objectUIDSecond)
	um.On("FetchUpdate", uh.API.Request(), uri2).Return(source2, int64(len(file2Content)), nil)

	// setup filesystembackend
	target1 := &filemock.FileMock{}
	target1.On("Close").Return(nil)
	target2 := &filemock.FileMock{}
	target2.On("Close").Return(nil)

	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Create", path.Join(uh.settings.DownloadDir, objectUIDFirst)).Return(target1, nil)
	fsm.On("Create", path.Join(uh.settings.DownloadDir, objectUIDSecond)).Return(target2, nil)
	uh.Store = fsm

	// finish setup
	uh.Updater = um

	cpm := &copymock.CopyMock{}
	cpm.On("Copy", target1, source1, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil)
	cpm.On("Copy", target2, source2, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil)
	uh.CopyBackend = cpm

	err = uh.FetchUpdate(updateMetadata, nil)
	assert.NoError(t, err)

	aim.AssertExpectations(t)
	um.AssertExpectations(t)
	source1.AssertExpectations(t)
	source2.AssertExpectations(t)
	target1.AssertExpectations(t)
	target2.AssertExpectations(t)
	cpm.AssertExpectations(t)
	fsm.AssertExpectations(t)
}

func TestUpdateHubFetchUpdateWithActiveInactiveError(t *testing.T) {
	mode := newTestInstallMode()

	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(updateMetadataWithNoObjects))
	assert.NoError(t, err)

	um := &updatermock.UpdaterMock{}
	uh.Updater = um

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
	uh.Reporter = client.Reporter(testReporter{})

	err = uh.ReportCurrentState()
	assert.NoError(t, err)

	uh.Reporter = client.Reporter(testReporter{reportStateError: errors.New("error")})

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
			assert.IsType(t, tc.expectedState, uh.State)

			tc.subTest(t, uh, uh.State)

			aim.AssertExpectations(t)
		})
	}
}

func TestLoadUpdateHubSettingsWithOpenError(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	fsbm := &filesystemmock.FileSystemBackendMock{}

	uh, _ := newTestUpdateHub(nil, aim)
	uh.Store = fsbm
	uh.SystemSettingsPath = "/systempath"
	uh.RuntimeSettingsPath = "/runtimepath"

	fsbm.On("Open", uh.SystemSettingsPath).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))

	err := uh.LoadSettings()
	assert.EqualError(t, err, "open error")

	aim.AssertExpectations(t)
	fsbm.AssertExpectations(t)
}

func TestLoadUpdateHubSettings(t *testing.T) {
	testPath, err := ioutil.TempDir("", "updatehub-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	runtimeSettingsTestPath := path.Join(testPath, "runtime.conf")
	systemSettingsTestPath := path.Join(testPath, "system.conf")

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
			nil,
			func(t *testing.T, settings *Settings, err error) {
			},
		},

		{
			"RuntimeSettingsNotFound",
			"[Polling]\nEnabled=true",
			"",
			nil,
			func(t *testing.T, settings *Settings, err error) {
			},
		},

		{
			"InvalidSettingsFile",
			"test",
			"test",
			ini.ErrDelimiterNotFound{},
			func(t *testing.T, settings *Settings, err error) {
				assert.Equal(t, err.Error(), "key-value delimiter not found: test")
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

			uh.SystemSettingsPath = systemSettingsTestPath
			uh.RuntimeSettingsPath = runtimeSettingsTestPath

			if tc.systemSettings != "" {
				err := uh.Store.MkdirAll(filepath.Dir(systemSettingsTestPath), 0755)
				assert.NoError(t, err)
				err = afero.WriteFile(uh.Store, systemSettingsTestPath, []byte(tc.systemSettings), 0644)
				assert.NoError(t, err)
			}

			if tc.runtimeSettings != "" {
				err := uh.Store.MkdirAll(filepath.Dir(runtimeSettingsTestPath), 0755)
				assert.NoError(t, err)
				err = afero.WriteFile(uh.Store, runtimeSettingsTestPath, []byte(tc.runtimeSettings), 0644)
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
		Store:    afero.NewMemMapFs(),
		State:    state,
		TimeStep: time.Second,
		API:      client.NewApiClient("localhost"),
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
