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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anacrolix/missinggo/httptoo"
	"github.com/bouk/monkey"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/updatehub/updatehub/activeinactive"
	"github.com/updatehub/updatehub/client"
	"github.com/updatehub/updatehub/copy"
	"github.com/updatehub/updatehub/installifdifferent"
	"github.com/updatehub/updatehub/installmodes"
	"github.com/updatehub/updatehub/installmodes/imxkobs"
	"github.com/updatehub/updatehub/metadata"
	"github.com/updatehub/updatehub/testsmocks/activeinactivemock"
	"github.com/updatehub/updatehub/testsmocks/cmdlinemock"
	"github.com/updatehub/updatehub/testsmocks/copymock"
	"github.com/updatehub/updatehub/testsmocks/fileinfomock"
	"github.com/updatehub/updatehub/testsmocks/filemock"
	"github.com/updatehub/updatehub/testsmocks/filesystemmock"
	"github.com/updatehub/updatehub/testsmocks/installifdifferentmock"
	"github.com/updatehub/updatehub/testsmocks/objectmock"
	"github.com/updatehub/updatehub/testsmocks/rebootermock"
	"github.com/updatehub/updatehub/testsmocks/reportermock"
	"github.com/updatehub/updatehub/testsmocks/statesmock"
	"github.com/updatehub/updatehub/testsmocks/updatermock"
	"github.com/updatehub/updatehub/utils"
)

const (
	validUpdateMetadata = `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "test", "sha256sum": "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa" }
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

	validUpdateMetadataWithThreeObjects = `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "test", "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" },
      { "mode": "test", "sha256sum": "b9632efa90820ff35d6cec0946f99bb8a6317b1e2ef877e501a3e12b2d04d0ae" },
      { "mode": "test", "sha256sum": "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa" }
    ]
  ]
}`

	updateMetadataWithValidSha256sum = `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "test", "target": "/dev/xxa1", "sha256sum": "5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9" },
      { "mode": "test", "target": "/dev/xxa2", "sha256sum": "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b" }
    ]
  ]
}`
)

var testPrivateKey *rsa.PrivateKey

func init() {
	var err error
	testPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}
}

func startDownloadUpdateInAnotherFunc(apiClient *client.ApiClient, uh *UpdateHub, um *metadata.UpdateMetadata) ([]int, error) {
	var progressList []int
	var err error

	progressChan := make(chan int, 10)

	m := sync.Mutex{}
	m.Lock()

	go func() {
		m.Lock()
		defer m.Unlock()

		err = uh.DownloadUpdate(apiClient, um, nil, progressChan)
		close(progressChan)
	}()

	m.Unlock()
	for p := range progressChan {
		progressList = append(progressList, p)
	}

	return progressList, err
}

func startInstallUpdateInAnotherFunc(uh *UpdateHub, um *metadata.UpdateMetadata) ([]int, error) {
	var progressList []int
	var err error

	progressChan := make(chan int, 10)

	m := sync.Mutex{}
	m.Lock()

	go func() {
		m.Lock()
		defer m.Unlock()

		err = uh.InstallUpdate(um, progressChan)
		close(progressChan)
	}()

	m.Unlock()
	for p := range progressChan {
		progressList = append(progressList, p)
	}

	return progressList, err
}

func TestProcessCurrentState(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	rm := &reportermock.ReporterMock{}
	cm := &cmdlinemock.CmdLineExecuterMock{}
	fs := afero.NewMemMapFs()

	afero.WriteFile(fs, "/usr/share/updatehub/state-change-callback", []byte("a"), 0755)

	expectedCallOrder := []string{"enter", "report", "leave"}
	callOrder := []string{}

	cm.On("Execute", "/usr/share/updatehub/state-change-callback enter downloaded").Run(func(args mock.Arguments) {
		callOrder = append(callOrder, "enter")
	}).Return([]byte(""), nil).Once()
	cm.On("Execute", "/usr/share/updatehub/state-change-callback leave downloaded").Run(func(args mock.Arguments) {
		callOrder = append(callOrder, "leave")
	}).Return([]byte(""), nil).Once()

	apiClient := client.NewApiClient("address")

	uh, _ := newTestUpdateHub(NewDownloadedState(apiClient, nil), aim)
	uh.CmdLineExecuter = cm
	uh.Store = fs
	uh.Reporter = rm

	rm.On("ReportState", apiClient.Request(), "", "", "downloaded", "", uh.FirmwareMetadata).Run(func(args mock.Arguments) {
		callOrder = append(callOrder, "report")
	}).Return(nil).Once()

	nextState := uh.ProcessCurrentState()

	assert.IsType(t, &InstallingState{}, nextState)
	assert.Equal(t, expectedCallOrder, callOrder)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestProcessCurrentStateWithNonExistantCallback(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &cmdlinemock.CmdLineExecuterMock{}
	fs := afero.NewMemMapFs()

	uh, _ := newTestUpdateHub(NewIdleState(), aim)
	uh.CmdLineExecuter = cm
	uh.Store = fs

	nextState := uh.ProcessCurrentState()

	assert.IsType(t, &PollState{}, nextState)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestProcessCurrentStateWithEnterError(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &cmdlinemock.CmdLineExecuterMock{}
	fs := afero.NewMemMapFs()

	afero.WriteFile(fs, "/usr/share/updatehub/state-change-callback", []byte("a"), 0755)

	expectedError := fmt.Errorf("some error")

	cm.On("Execute", "/usr/share/updatehub/state-change-callback enter idle").Return([]byte(""), expectedError).Once()

	uh, _ := newTestUpdateHub(NewIdleState(), aim)
	uh.CmdLineExecuter = cm
	uh.Store = fs

	nextState := uh.ProcessCurrentState()

	assert.IsType(t, &ErrorState{}, nextState)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestProcessCurrentStateWithLeaveError(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &cmdlinemock.CmdLineExecuterMock{}
	fs := afero.NewMemMapFs()

	afero.WriteFile(fs, "/usr/share/updatehub/state-change-callback", []byte("a"), 0755)

	expectedError := fmt.Errorf("some error")

	cm.On("Execute", "/usr/share/updatehub/state-change-callback enter idle").Return([]byte(""), nil).Once()
	cm.On("Execute", "/usr/share/updatehub/state-change-callback leave idle").Return([]byte(""), expectedError).Once()

	uh, _ := newTestUpdateHub(NewIdleState(), aim)
	uh.CmdLineExecuter = cm
	uh.Store = fs

	nextState := uh.ProcessCurrentState()

	assert.IsType(t, &ErrorState{}, nextState)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestProcessCurrentStateIsError(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &cmdlinemock.CmdLineExecuterMock{}
	rm := &reportermock.ReporterMock{}
	fs := afero.NewMemMapFs()

	afero.WriteFile(fs, "/usr/share/updatehub/error-callback", []byte("a"), 0755)

	expectedError := fmt.Errorf("some error")

	cm.On("Execute", "/usr/share/updatehub/error-callback 'transient error: some error'").Return([]byte(""), nil).Once()

	apiClient := client.NewApiClient("address")

	uh, _ := newTestUpdateHub(NewErrorState(apiClient, nil, NewTransientError(expectedError)), aim)
	uh.CmdLineExecuter = cm
	uh.Reporter = rm
	uh.Store = fs
	uh.previousState = NewIdleState()

	rm.On("ReportState", apiClient.Request(), "", "idle", "error", "some error", uh.FirmwareMetadata).Return(nil).Once()

	nextState := uh.ProcessCurrentState()

	assert.IsType(t, &ErrorState{}, uh.previousState)
	assert.IsType(t, &IdleState{}, nextState)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestProcessCurrentStateIsErrorWithNonExistantCallback(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &cmdlinemock.CmdLineExecuterMock{}
	rm := &reportermock.ReporterMock{}
	fs := afero.NewMemMapFs()

	expectedError := fmt.Errorf("some error")

	apiClient := client.NewApiClient("address")

	uh, _ := newTestUpdateHub(NewErrorState(apiClient, nil, NewTransientError(expectedError)), aim)
	uh.CmdLineExecuter = cm
	uh.Reporter = rm
	uh.Store = fs
	uh.previousState = NewIdleState()

	rm.On("ReportState", apiClient.Request(), "", "idle", "error", "some error", uh.FirmwareMetadata).Return(nil).Once()

	nextState := uh.ProcessCurrentState()

	assert.IsType(t, &IdleState{}, nextState)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestProcessCurrentStateWithEnterFlow(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &cmdlinemock.CmdLineExecuterMock{}
	fs := afero.NewMemMapFs()

	afero.WriteFile(fs, "/usr/share/updatehub/state-change-callback", []byte("a"), 0755)

	cm.On("Execute", "/usr/share/updatehub/state-change-callback enter poll").Return([]byte("cancel"), nil).Once()

	uh, _ := newTestUpdateHub(NewPollState(0), aim)
	uh.CmdLineExecuter = cm
	uh.Store = fs

	nextState := uh.ProcessCurrentState()

	assert.IsType(t, &IdleState{}, nextState)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestProcessCurrentStateWithLeaveFlow(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	cm := &cmdlinemock.CmdLineExecuterMock{}
	fs := afero.NewMemMapFs()

	afero.WriteFile(fs, "/usr/share/updatehub/state-change-callback", []byte("a"), 0755)

	cm.On("Execute", "/usr/share/updatehub/state-change-callback enter poll").Return([]byte(""), nil).Once()
	cm.On("Execute", "/usr/share/updatehub/state-change-callback leave poll").Return([]byte("cancel"), nil).Once()

	uh, _ := newTestUpdateHub(NewPollState(0), aim)
	uh.CmdLineExecuter = cm
	uh.Store = fs

	nextState := uh.ProcessCurrentState()

	assert.IsType(t, &IdleState{}, nextState)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestNewUpdateHub(t *testing.T) {
	gitversion := "2.1"
	memFs := afero.NewMemMapFs()
	initialState := NewIdleState()
	stateChangeCallbackPath := "/usr/share/updatehub/state-change-callback"
	errorCallbackPath := "/usr/share/updatehub/error-callback"
	validateCallbackPath := "/usr/share/updatehub/validate-callback"
	rollbackCallbackPath := "/usr/share/updatehub/rollback-callback"

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}

	settings := &Settings{}

	pubKey := &testPrivateKey.PublicKey

	uh := NewUpdateHub(gitversion, stateChangeCallbackPath, errorCallbackPath, validateCallbackPath, rollbackCallbackPath, memFs, *fm, pubKey, initialState, settings, client.NewApiClient("address"))

	assert.Equal(t, &activeinactive.DefaultImpl{CmdLineExecuter: &utils.CmdLine{}}, uh.ActiveInactiveBackend)
	assert.Equal(t, gitversion, uh.Version)
	assert.Equal(t, initialState, uh.GetState())
	assert.Equal(t, client.NewUpdateClient(), uh.Updater)
	assert.Equal(t, time.Minute, uh.TimeStep)
	assert.Equal(t, memFs, uh.Store)
	assert.Equal(t, *fm, uh.FirmwareMetadata)
	assert.Equal(t, settings, uh.Settings)
	assert.Equal(t, client.NewReportClient(), uh.Reporter)
	assert.Equal(t, &Sha256CheckerImpl{}, uh.Sha256Checker)
	assert.Equal(t, &installifdifferent.DefaultImpl{FileSystemBackend: memFs}, uh.InstallIfDifferentBackend)
	assert.Equal(t, copy.ExtendedIO{}, uh.CopyBackend)
	assert.Equal(t, stateChangeCallbackPath, uh.StateChangeCallbackPath)
	assert.Equal(t, errorCallbackPath, uh.ErrorCallbackPath)
}

func TestCheckDownloadedObjectSha256sum(t *testing.T) {
	memFs := afero.NewMemMapFs()
	testPath, err := afero.TempDir(memFs, "", "states-test")

	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	err = afero.WriteFile(memFs, path.Join(testPath, expectedSha256sum), []byte("test"), 0666)
	assert.NoError(t, err)

	sci := &Sha256CheckerImpl{}
	ok, err := sci.CheckDownloadedObjectSha256sum(memFs, testPath, expectedSha256sum)
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestCheckDownloadedObjectSha256sumWithOpenError(t *testing.T) {
	dummyPath := "/dummy"
	dummySha256sum := "dummy_hash"

	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("OpenFile", path.Join(dummyPath, dummySha256sum), os.O_RDONLY, os.FileMode(0)).Return(&filemock.FileMock{}, fmt.Errorf("open error"))

	sci := &Sha256CheckerImpl{}
	ok, err := sci.CheckDownloadedObjectSha256sum(fsm, dummyPath, dummySha256sum)
	assert.False(t, ok)
	assert.EqualError(t, err, "open error")

	fsm.AssertExpectations(t)
}

func TestCheckDownloadedObjectSha256sumWithSumsDontMatching(t *testing.T) {
	memFs := afero.NewMemMapFs()
	testPath, err := afero.TempDir(memFs, "", "states-test")

	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	err = afero.WriteFile(memFs, path.Join(testPath, expectedSha256sum), []byte("another"), 0666)
	assert.NoError(t, err)

	sci := &Sha256CheckerImpl{}
	ok, err := sci.CheckDownloadedObjectSha256sum(memFs, testPath, expectedSha256sum)
	assert.False(t, ok)
	assert.NoError(t, err)
}

func TestGetIndexOfObjectToBeInstalled(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadataWithActiveInactive))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(m.Objects))

	testCases := []struct {
		caseName  string
		active    int
		installTo int
	}{
		{
			"ActiveZero",
			0,
			1,
		},
		{
			"ActiveOne",
			1,
			0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			aim := &activeinactivemock.ActiveInactiveMock{}
			aim.On("Active").Return(tc.active, nil)
			index, err := GetIndexOfObjectToBeInstalled(aim, m)
			assert.NoError(t, err)
			assert.Equal(t, tc.installTo, index)
			aim.AssertExpectations(t)
		})
	}
}

func TestGetIndexOfObjectToBeInstalledWithActiveError(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	m, err := metadata.NewUpdateMetadata([]byte(validJSONMetadataWithActiveInactive))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(m.Objects))

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(1, fmt.Errorf("active error"))
	index, err := GetIndexOfObjectToBeInstalled(aim, m)
	assert.EqualError(t, err, "active error")
	assert.Equal(t, 0, index)

	aim.AssertExpectations(t)
}

func TestGetIndexOfObjectToBeInstalledWithMoreThanTwoObjects(t *testing.T) {
	// declaration just to register the imxkobs install mode
	_ = &imxkobs.ImxKobsObject{}

	activeInactiveJSONMetadataWithThreeObjects := `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      {
            "mode": "imxkobs",
            "target": "/dev/xx1",
            "target-type": "device"
          }
	    ]
        ,
	    [
	      {
            "mode": "imxkobs",
            "target": "/dev/xx2",
            "target-type": "device"
          }
	    ]
        ,
	    [
	      {
            "mode": "imxkobs",
            "target": "/dev/xx3",
            "target-type": "device"
          }
	    ]
	  ]
	}`

	m, err := metadata.NewUpdateMetadata([]byte(activeInactiveJSONMetadataWithThreeObjects))
	assert.NoError(t, err)
	assert.Equal(t, 3, len(m.Objects))

	aim := &activeinactivemock.ActiveInactiveMock{}
	index, err := GetIndexOfObjectToBeInstalled(aim, m)
	assert.EqualError(t, err, "update metadata must have 1 or 2 objects. Found 3")
	assert.Equal(t, 0, index)
}

func TestGetIndexOfObjectToBeInstalledWithNoObjects(t *testing.T) {
	activeInactiveJSONMetadataWithThreeObjects := `{
	  "product-uid": "0123456789",
	  "objects": [
	  ]
	}`

	m, err := metadata.NewUpdateMetadata([]byte(activeInactiveJSONMetadataWithThreeObjects))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(m.Objects))

	aim := &activeinactivemock.ActiveInactiveMock{}
	index, err := GetIndexOfObjectToBeInstalled(aim, m)
	assert.EqualError(t, err, "update metadata must have 1 or 2 objects. Found 0")
	assert.Equal(t, 0, index)
}

func TestUpdateHubProbeUpdate(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	testCases := []struct {
		name           string
		updateMetadata string
		extraPoll      time.Duration
	}{
		{
			"InvalidUpdateMetadata",
			"",
			-1,
		},

		{
			"ValidUpdateMetadata",
			validUpdateMetadata,
			13,
		},
	}

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)
	updateMetadataPath := path.Join(uh.Settings.DownloadDir, metadata.UpdateMetadataFilename)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedUpdateMetadata, _ := metadata.NewUpdateMetadata([]byte(tc.updateMetadata))

			err := afero.WriteFile(uh.Store, updateMetadataPath, []byte("dummyfile"), 0644)
			assert.NoError(t, err)

			sha256sum := sha256.Sum256([]byte(tc.updateMetadata))
			expectedSignature, err := rsa.SignPKCS1v15(rand.Reader, testPrivateKey, crypto.SHA256, sha256sum[:])
			assert.NoError(t, err)
			assert.NotEmpty(t, expectedSignature)

			var data struct {
				Retries int `json:"retries"`
				metadata.FirmwareMetadata
				LastInstalledPackage string `json:"last-installed-package,omitempty"`
			}

			data.FirmwareMetadata = uh.FirmwareMetadata
			data.Retries = 0
			data.LastInstalledPackage = "61be55a8e2f6b4e172338bddf184d6dbee29c98853e0a0485ecee7f27b9af0b4"

			uh.lastInstalledPackageUID = "61be55a8e2f6b4e172338bddf184d6dbee29c98853e0a0485ecee7f27b9af0b4"

			apiClient := client.NewApiClient("address")

			um := &updatermock.UpdaterMock{}
			um.On("ProbeUpdate", apiClient.Request(), client.UpgradesEndpoint, data).Return(expectedUpdateMetadata, expectedSignature, tc.extraPoll, nil)
			uh.Updater = um

			updateMetadata, signature, extraPoll, err := uh.ProbeUpdate(apiClient, 0)

			assert.Equal(t, expectedUpdateMetadata, updateMetadata)
			assert.Equal(t, expectedSignature, signature)
			assert.Equal(t, tc.extraPoll, extraPoll)
			assert.Nil(t, err)
			um.AssertExpectations(t)

			if tc.updateMetadata == "" {
				fileExists, err := afero.Exists(uh.Store, updateMetadataPath)
				assert.NoError(t, err)
				assert.False(t, fileExists)
			} else {
				data, err := afero.ReadFile(uh.Store, updateMetadataPath)
				assert.NoError(t, err)
				assert.Equal(t, tc.updateMetadata, string(data))
			}
		})
	}

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestUpdateHubProbeUpdateWithNilUpdateMetadata(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)
	updateMetadataPath := path.Join(uh.Settings.DownloadDir, metadata.UpdateMetadataFilename)

	err := afero.WriteFile(uh.Store, updateMetadataPath, []byte("dummyfile"), 0644)
	assert.NoError(t, err)

	var data struct {
		Retries int `json:"retries"`
		metadata.FirmwareMetadata
		LastInstalledPackage string `json:"last-installed-package,omitempty"`
	}

	data.FirmwareMetadata = uh.FirmwareMetadata
	data.Retries = 0
	data.LastInstalledPackage = ""

	apiClient := client.NewApiClient("address")

	um := &updatermock.UpdaterMock{}
	um.On("ProbeUpdate", apiClient.Request(), client.UpgradesEndpoint, data).Return(nil, []byte{}, time.Duration(3000), nil)

	uh.Updater = um

	updateMetadata, signature, extraPoll, err := uh.ProbeUpdate(apiClient, 0)

	assert.Equal(t, (*metadata.UpdateMetadata)(nil), updateMetadata)
	assert.Equal(t, []byte{}, signature)
	assert.Equal(t, time.Duration(3000), extraPoll)
	assert.Nil(t, err)
	um.AssertExpectations(t)

	fileExists, err := afero.Exists(uh.Store, updateMetadataPath)
	assert.NoError(t, err)
	assert.False(t, fileExists)

	aim.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestUpdateHubDownloadUpdate(t *testing.T) {
	om1 := &objectmock.ObjectMock{}
	om2 := &objectmock.ObjectMock{}
	om3 := &objectmock.ObjectMock{}

	objs := []metadata.Object{om1, om2, om3}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadataWithThreeObjects))
	assert.NoError(t, err)

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadataWithThreeObjects))

	um := &updatermock.UpdaterMock{}
	uh.Updater = um

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}
	uh.FirmwareMetadata = *fm

	scm := &statesmock.Sha256CheckerMock{}

	uh.Sha256Checker = scm

	// setup filesystembackend

	cpm := &copymock.CopyMock{}
	uh.CopyBackend = cpm

	fsm := &filesystemmock.FileSystemBackendMock{}
	uh.Store = fsm

	fsm.On("Open", uh.Settings.DownloadDir).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))

	// these sha256sum's are from "validUpdateMetadataWithThreeObjects" content

	// obj1
	objectUID1 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	uri1 := path.Join("/products", uh.FirmwareMetadata.ProductUID, "packages", packageUID, "objects", objectUID1)

	source1 := &filemock.FileMock{}
	source1.On("Close").Return(nil).Once()
	source1Content := []byte("content1")

	um.On("GetUpdateContentRange", uh.DefaultApiClient.Request(), uri1, int64(len(source1Content))).Return(&httptoo.BytesContentRange{}, nil).Once()
	um.On("DownloadUpdate", uh.DefaultApiClient.Request(), uri1).Return(source1, int64(len(source1Content)), nil).Once()

	scm.On("CheckDownloadedObjectSha256sum", uh.Store, uh.Settings.DownloadDir, objectUID1).Return(true, nil).Once()

	// obj2
	objectUID2 := "b9632efa90820ff35d6cec0946f99bb8a6317b1e2ef877e501a3e12b2d04d0ae"
	uri2 := path.Join("/products", uh.FirmwareMetadata.ProductUID, "packages", packageUID, "objects", objectUID2)

	source2 := &filemock.FileMock{}
	source2.On("Close").Return(nil).Once()
	source2Content := []byte("content2")

	um.On("DownloadUpdate", uh.DefaultApiClient.Request(), uri2).Return(source2, int64(len(source2Content)), nil).Once()
	um.On("GetUpdateContentRange", uh.DefaultApiClient.Request(), uri2, int64(len(source2Content))).Return(&httptoo.BytesContentRange{}, nil).Once()

	scm.On("CheckDownloadedObjectSha256sum", uh.Store, uh.Settings.DownloadDir, objectUID2).Return(true, nil).Once()

	// obj3
	objectUID3 := "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa"
	uri3 := path.Join("/products", uh.FirmwareMetadata.ProductUID, "/packages", packageUID, "objects", objectUID3)

	source3 := &filemock.FileMock{}
	source3.On("Close").Return(nil).Once()
	source3Content := []byte("content3")

	um.On("DownloadUpdate", uh.DefaultApiClient.Request(), uri3).Return(source3, int64(len(source3Content)), nil).Once()
	um.On("GetUpdateContentRange", uh.DefaultApiClient.Request(), uri3, int64(len(source3Content))).Return(&httptoo.BytesContentRange{}, nil).Once()

	scm.On("CheckDownloadedObjectSha256sum", uh.Store, uh.Settings.DownloadDir, objectUID3).Return(true, nil).Once()

	// file1
	target1Info := &fileinfomock.FileInfoMock{}
	target1Info.On("Size").Return(int64(len(source1Content)))
	target1 := &filemock.FileMock{}
	target1.On("Stat").Return(target1Info, nil).Once()
	target1.On("Close").Return(nil).Once()
	cpm.On("Copy", target1, source1, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID1), os.O_RDONLY, os.FileMode(0)).Return((*filemock.FileMock)(nil), fmt.Errorf("not found")).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID1), os.O_APPEND|os.O_CREATE|os.O_RDWR, os.FileMode(0666)).Return(target1, nil).Once()

	// file2
	target2Info := &fileinfomock.FileInfoMock{}
	target2Info.On("Size").Return(int64(len(source2Content)))
	target2 := &filemock.FileMock{}
	target2.On("Stat").Return(target2Info, nil).Once()
	target2.On("Close").Return(nil).Once()
	cpm.On("Copy", target2, source2, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID2), os.O_RDONLY, os.FileMode(0)).Return((*filemock.FileMock)(nil), fmt.Errorf("not found")).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID2), os.O_APPEND|os.O_CREATE|os.O_RDWR, os.FileMode(0666)).Return(target2, nil).Once()

	// file3
	target3Info := &fileinfomock.FileInfoMock{}
	target3Info.On("Size").Return(int64(len(source3Content)))
	target3 := &filemock.FileMock{}
	target3.On("Stat").Return(target3Info, nil).Once()
	target3.On("Close").Return(nil).Once()
	cpm.On("Copy", target3, source3, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID3), os.O_RDONLY, os.FileMode(0)).Return((*filemock.FileMock)(nil), fmt.Errorf("not found")).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID3), os.O_APPEND|os.O_CREATE|os.O_RDWR, os.FileMode(0666)).Return(target3, nil).Once()

	progressList, err := startDownloadUpdateInAnotherFunc(uh.DefaultApiClient, uh, updateMetadata)

	assert.NoError(t, err)
	assert.Equal(t, []int{33, 66, 99, 100}, progressList)

	aim.AssertExpectations(t)
	cpm.AssertExpectations(t)
	target1.AssertExpectations(t)
	target2.AssertExpectations(t)
	target3.AssertExpectations(t)
	source1.AssertExpectations(t)
	source2.AssertExpectations(t)
	source3.AssertExpectations(t)
	fsm.AssertExpectations(t)
	um.AssertExpectations(t)
	om1.AssertExpectations(t)
	om2.AssertExpectations(t)
	om3.AssertExpectations(t)
}

func TestUpdateHubDownloadUpdateWithObjectsAlreadyDownloaded(t *testing.T) {
	fs := afero.NewMemMapFs()

	om1 := &objectmock.ObjectMock{}
	om2 := &objectmock.ObjectMock{}
	om3 := &objectmock.ObjectMock{}

	objs := []metadata.Object{om1, om2, om3}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	um := &updatermock.UpdaterMock{}
	uh.Updater = um

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}
	uh.FirmwareMetadata = *fm

	// setup filesystembackend

	cpm := &copymock.CopyMock{}
	uh.CopyBackend = cpm

	uh.Store = fs

	downloadDir, err := afero.TempDir(fs, "", "updatehub-test")
	assert.NoError(t, err)
	defer os.RemoveAll(downloadDir)
	uh.Settings.DownloadDir = downloadDir

	// this sha256sum is from "validUpdateMetadata" content
	objectUID := "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa"
	err = afero.WriteFile(fs, path.Join(downloadDir, objectUID), []byte("content1"), 0644)
	assert.NoError(t, err)

	progressList, err := startDownloadUpdateInAnotherFunc(uh.DefaultApiClient, uh, updateMetadata)

	assert.NoError(t, err)
	assert.Equal(t, []int{100}, progressList)

	aim.AssertExpectations(t)
	cpm.AssertExpectations(t)
	um.AssertExpectations(t)
	om1.AssertExpectations(t)
	om2.AssertExpectations(t)
	om3.AssertExpectations(t)
}

func TestUpdateHubDownloadUpdateWithTargetFileError(t *testing.T) {
	om := &objectmock.ObjectMock{}

	objs := []metadata.Object{om}

	mode := newTestInstallMode(objs)
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
	fsm.On("Open", uh.Settings.DownloadDir).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID), os.O_RDONLY, os.FileMode(0)).Return((*filemock.FileMock)(nil), fmt.Errorf("not found")).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID), os.O_APPEND|os.O_CREATE|os.O_RDWR, os.FileMode(0666)).Return((*filemock.FileMock)(nil), fmt.Errorf("create error"))
	uh.Store = fsm

	progressList, err := startDownloadUpdateInAnotherFunc(uh.DefaultApiClient, uh, updateMetadata)

	assert.EqualError(t, err, "create error")
	assert.Equal(t, []int(nil), progressList)

	aim.AssertExpectations(t)
	cpm.AssertExpectations(t)
	fsm.AssertExpectations(t)
	um.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestUpdateHubDownloadUpdateWithUpdaterError(t *testing.T) {
	om := &objectmock.ObjectMock{}

	objs := []metadata.Object{om}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}
	uh.FirmwareMetadata = *fm

	apiClient := client.NewApiClient("address")

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadata))
	objectUID := updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum

	uri := path.Join("/products", uh.FirmwareMetadata.ProductUID, "packages", packageUID, "objects", objectUID)

	source := &filemock.FileMock{}

	um := &updatermock.UpdaterMock{}
	um.On("DownloadUpdate", apiClient.Request(), uri).Return(source, int64(0), fmt.Errorf("updater error"))
	uh.Updater = um

	// setup filesystembackend

	targetInfo := &fileinfomock.FileInfoMock{}
	targetInfo.On("Size").Return(int64(0))
	target := &filemock.FileMock{}
	target.On("Stat").Return(targetInfo, nil)
	target.On("Close").Return(nil)

	cpm := &copymock.CopyMock{}
	uh.CopyBackend = cpm

	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Open", uh.Settings.DownloadDir).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID), os.O_RDONLY, os.FileMode(0)).Return((*filemock.FileMock)(nil), fmt.Errorf("not found")).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID), os.O_APPEND|os.O_CREATE|os.O_RDWR, os.FileMode(0666)).Return(target, nil)
	uh.Store = fsm

	progressList, err := startDownloadUpdateInAnotherFunc(apiClient, uh, updateMetadata)

	assert.EqualError(t, err, "updater error")
	assert.Equal(t, []int(nil), progressList)

	aim.AssertExpectations(t)
	cpm.AssertExpectations(t)
	target.AssertExpectations(t)
	source.AssertExpectations(t)
	fsm.AssertExpectations(t)
	um.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestUpdateHubDownloadUpdateWithCopyError(t *testing.T) {
	om := &objectmock.ObjectMock{}

	objs := []metadata.Object{om}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}
	uh.FirmwareMetadata = *fm

	apiClient := client.NewApiClient("address")

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadata))
	objectUID := updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum

	uri := path.Join("/products", uh.FirmwareMetadata.ProductUID, "packages", packageUID, "objects", objectUID)

	source := &filemock.FileMock{}
	source.On("Close").Return(nil)
	sourceContent := []byte("content")

	um := &updatermock.UpdaterMock{}
	um.On("DownloadUpdate", apiClient.Request(), uri).Return(source, int64(len(sourceContent)), nil)
	uh.Updater = um

	// setup filesystembackend

	targetInfo := &fileinfomock.FileInfoMock{}
	targetInfo.On("Size").Return(int64(0))
	target := &filemock.FileMock{}
	target.On("Stat").Return(targetInfo, nil)
	target.On("Close").Return(nil)

	cpm := &copymock.CopyMock{}
	cpm.On("Copy", target, source, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, fmt.Errorf("copy error"))
	uh.CopyBackend = cpm

	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Open", uh.Settings.DownloadDir).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID), os.O_RDONLY, os.FileMode(0)).Return((*filemock.FileMock)(nil), fmt.Errorf("not found")).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID), os.O_APPEND|os.O_CREATE|os.O_RDWR, os.FileMode(0666)).Return(target, nil)
	uh.Store = fsm

	progressList, err := startDownloadUpdateInAnotherFunc(apiClient, uh, updateMetadata)

	assert.EqualError(t, err, "copy error")
	assert.Equal(t, []int(nil), progressList)

	aim.AssertExpectations(t)
	cpm.AssertExpectations(t)
	target.AssertExpectations(t)
	source.AssertExpectations(t)
	fsm.AssertExpectations(t)
	um.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestUpdateHubDownloadUpdateWithActiveInactive(t *testing.T) {
	om1 := &objectmock.ObjectMock{}
	om2 := &objectmock.ObjectMock{}
	om3 := &objectmock.ObjectMock{}
	om4 := &objectmock.ObjectMock{}

	objs := []metadata.Object{om1, om2, om3, om4}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(0, nil)

	uh, _ := newTestUpdateHub(&PollState{}, aim)
	uh.FirmwareMetadata.ProductUID = "148de9c5a7a44d19e56cd9ae1a554bf67847afb0c58f6e12fa29ac7ddfca9940"

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}
	uh.FirmwareMetadata = *fm

	scm := &statesmock.Sha256CheckerMock{}

	uh.Sha256Checker = scm

	apiClient := client.NewApiClient("address")

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadataWithActiveInactive))

	expectedURIPrefix := "/products"
	expectedURIPrefix = path.Join(expectedURIPrefix, uh.FirmwareMetadata.ProductUID)
	expectedURIPrefix = path.Join(expectedURIPrefix, "packages")
	expectedURIPrefix = path.Join(expectedURIPrefix, packageUID)
	expectedURIPrefix = path.Join(expectedURIPrefix, "objects")

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadataWithActiveInactive))
	assert.NoError(t, err)

	um := &updatermock.UpdaterMock{}

	fsm := &filesystemmock.FileSystemBackendMock{}
	uh.Store = fsm

	// download of file 1 setup
	file1Content := []byte("content1") // this matches with the sha256sum in "validUpdateMetadataWithActiveInactive"

	source1 := &filemock.FileMock{}
	source1.On("Close").Return(nil)

	objectUIDFirst := updateMetadata.Objects[1][0].GetObjectMetadata().Sha256sum
	uri1 := path.Join(expectedURIPrefix, objectUIDFirst)
	um.On("DownloadUpdate", apiClient.Request(), uri1).Return(source1, int64(len(file1Content)), nil)
	scm.On("CheckDownloadedObjectSha256sum", uh.Store, uh.Settings.DownloadDir, objectUIDFirst).Return(true, nil).Once()

	// download of file 2 setup
	file2Content := []byte("content2butbigger") // this matches with the sha256sum in "validUpdateMetadataWithActiveInactive"

	source2 := &filemock.FileMock{}
	source2.On("Close").Return(nil)

	objectUIDSecond := updateMetadata.Objects[1][1].GetObjectMetadata().Sha256sum
	uri2 := path.Join(expectedURIPrefix, objectUIDSecond)
	um.On("DownloadUpdate", apiClient.Request(), uri2).Return(source2, int64(len(file2Content)), nil)
	scm.On("CheckDownloadedObjectSha256sum", uh.Store, uh.Settings.DownloadDir, objectUIDSecond).Return(true, nil).Once()

	// setup filesystembackend
	target1Info := &fileinfomock.FileInfoMock{}
	target1Info.On("Size").Return(int64(0))
	target1 := &filemock.FileMock{}
	target1.On("Stat").Return(target1Info, nil)
	target1.On("Close").Return(nil)

	target2Info := &fileinfomock.FileInfoMock{}
	target2Info.On("Size").Return(int64(0))
	target2 := &filemock.FileMock{}
	target2.On("Stat").Return(target2Info, nil)
	target2.On("Close").Return(nil)

	fsm.On("Open", uh.Settings.DownloadDir).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUIDFirst), os.O_RDONLY, os.FileMode(0)).Return((*filemock.FileMock)(nil), fmt.Errorf("not found")).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUIDFirst), os.O_APPEND|os.O_CREATE|os.O_RDWR, os.FileMode(0666)).Return(target1, nil)
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUIDSecond), os.O_RDONLY, os.FileMode(0)).Return((*filemock.FileMock)(nil), fmt.Errorf("not found")).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUIDSecond), os.O_APPEND|os.O_CREATE|os.O_RDWR, os.FileMode(0666)).Return(target2, nil)

	// finish setup
	uh.Updater = um

	cpm := &copymock.CopyMock{}
	cpm.On("Copy", target1, source1, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil)
	cpm.On("Copy", target2, source2, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil)
	uh.CopyBackend = cpm

	progressList, err := startDownloadUpdateInAnotherFunc(apiClient, uh, updateMetadata)

	assert.NoError(t, err)
	assert.Equal(t, []int{50, 100}, progressList)

	aim.AssertExpectations(t)
	um.AssertExpectations(t)
	source1.AssertExpectations(t)
	source2.AssertExpectations(t)
	target1.AssertExpectations(t)
	target2.AssertExpectations(t)
	cpm.AssertExpectations(t)
	fsm.AssertExpectations(t)
	om1.AssertExpectations(t)
	om2.AssertExpectations(t)
	om3.AssertExpectations(t)
	om4.AssertExpectations(t)
}

func TestUpdateHubDownloadUpdateWithActiveInactiveError(t *testing.T) {
	om := &objectmock.ObjectMock{}

	objs := []metadata.Object{om}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(updateMetadataWithNoObjects))
	assert.NoError(t, err)

	um := &updatermock.UpdaterMock{}
	uh.Updater = um

	progressList, err := startDownloadUpdateInAnotherFunc(uh.DefaultApiClient, uh, updateMetadata)

	assert.EqualError(t, err, "update metadata must have 1 or 2 objects. Found 0")
	assert.Equal(t, []int(nil), progressList)

	aim.AssertExpectations(t)
	um.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestUpdateHubDownloadUpdateWithSha256Error(t *testing.T) {
	om := &objectmock.ObjectMock{}

	objs := []metadata.Object{om}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(&PollState{}, aim)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	apiClient := client.NewApiClient("address")

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadata))
	objectUID := updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum

	uri := path.Join("/products", uh.FirmwareMetadata.ProductUID, "packages", packageUID, "objects", objectUID)

	// setup filesystembackend
	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Open", uh.Settings.DownloadDir).Return((*filemock.FileMock)(nil), fmt.Errorf("open error"))
	uh.Store = fsm

	source := &filemock.FileMock{}
	source.On("Close").Return(nil)
	sourceContent := []byte("content")

	cpm := &copymock.CopyMock{}

	targetInfo := &fileinfomock.FileInfoMock{}
	targetInfo.On("Size").Return(int64(len(sourceContent)))
	target := &filemock.FileMock{}
	target.On("Stat").Return(targetInfo, nil).Once()
	target.On("Close").Return(nil).Once()
	cpm.On("Copy", target, source, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID), os.O_RDONLY, os.FileMode(0)).Return((*filemock.FileMock)(nil), fmt.Errorf("not found")).Once()
	fsm.On("OpenFile", path.Join(uh.Settings.DownloadDir, objectUID), os.O_APPEND|os.O_CREATE|os.O_RDWR, os.FileMode(0666)).Return(target, nil).Once()
	fsm.On("Remove", path.Join(uh.Settings.DownloadDir, objectUID)).Return(nil)
	fsm.On("Stat", path.Join(uh.Settings.DownloadDir, objectUID)).Return(targetInfo, nil)

	uh.CopyBackend = cpm

	um := &updatermock.UpdaterMock{}
	um.On("DownloadUpdate", apiClient.Request(), uri).Return(source, int64(len(sourceContent)), nil).Once()
	um.On("GetUpdateContentRange", apiClient.Request(), uri, int64(len(sourceContent))).Return(&httptoo.BytesContentRange{}, nil).Once()

	uh.Updater = um

	scm := &statesmock.Sha256CheckerMock{}
	scm.On("CheckDownloadedObjectSha256sum", uh.Store, uh.Settings.DownloadDir, objectUID).Return(false, nil).Once()
	uh.Sha256Checker = scm

	progressList, err := startDownloadUpdateInAnotherFunc(apiClient, uh, updateMetadata)

	assert.EqualError(t, err, ErrSha256sum.Error())
	assert.Equal(t, []int(nil), progressList)

	aim.AssertExpectations(t)
	um.AssertExpectations(t)
	om.AssertExpectations(t)
	scm.AssertExpectations(t)
}

func TestUpdateHubInstallUpdate(t *testing.T) {
	type testData struct {
		uh   *UpdateHub
		objs []metadata.Object
		aim  *activeinactivemock.ActiveInactiveMock
		iidm *installifdifferentmock.InstallIfDifferentMock
		scm  *statesmock.Sha256CheckerMock
		fm   *metadata.FirmwareMetadata
	}

	testCases := []struct {
		name                 string
		data                 *testData
		rawUpdateMetadata    string
		expectedError        error
		expectedProgressList []int
	}{
		{
			"WithSuccess",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}

				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)

				om1 := &objectmock.ObjectMock{}
				om1.On("Setup").Return(nil).Once()
				om1.On("Install", data.uh.Settings.DownloadDir).Return(nil).Once()
				om1.On("Cleanup").Return(nil).Once()

				om2 := &objectmock.ObjectMock{}
				om2.On("Setup").Return(nil).Once()
				om2.On("Install", data.uh.Settings.DownloadDir).Return(nil).Once()
				om2.On("Cleanup").Return(nil).Once()

				om3 := &objectmock.ObjectMock{}
				om3.On("Setup").Return(nil).Once()
				om3.On("Install", data.uh.Settings.DownloadDir).Return(nil).Once()
				om3.On("Cleanup").Return(nil).Once()

				data.objs = []metadata.Object{om1, om2, om3}

				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}
				data.iidm.On("Proceed", om1).Return(true, nil).Once()
				data.iidm.On("Proceed", om2).Return(true, nil).Once()
				data.iidm.On("Proceed", om3).Return(true, nil).Once()

				// these sha256sum's are from "validUpdateMetadataWithThreeObjects" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(true, nil)
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "b9632efa90820ff35d6cec0946f99bb8a6317b1e2ef877e501a3e12b2d04d0ae").Return(true, nil)
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa").Return(true, nil)

				return data
			}(),
			validUpdateMetadataWithThreeObjects,
			nil,
			[]int{33, 66, 99, 100},
		},
		{
			"WithCheckSupportedHardwareError",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "hardware-value",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}
				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)
				data.objs = []metadata.Object{&objectmock.ObjectMock{}}
				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}
				data.scm = &statesmock.Sha256CheckerMock{}

				return data
			}(),
			validUpdateMetadata,
			fmt.Errorf("unknown supported hardware format in the update metadata"),
			[]int(nil),
		},
		{
			"WithActiveInactive",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}
				data.aim.On("Active").Return(1, nil)
				data.aim.On("SetActive", 0).Return(nil)

				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)

				om1 := &objectmock.ObjectMock{}
				om1.On("Setup").Return(nil).Once()
				om1.On("Install", data.uh.Settings.DownloadDir).Return(nil).Once()
				om1.On("Cleanup").Return(nil).Once()

				om2 := &objectmock.ObjectMock{}
				om2.On("Setup").Return(nil).Once()
				om2.On("Install", data.uh.Settings.DownloadDir).Return(nil).Once()
				om2.On("Cleanup").Return(nil).Once()

				om3 := &objectmock.ObjectMock{}
				om4 := &objectmock.ObjectMock{}
				data.objs = []metadata.Object{om1, om2, om3, om4}

				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}
				data.iidm.On("Proceed", om1).Return(true, nil)
				data.iidm.On("Proceed", om2).Return(true, nil)

				// these sha256sum's are from "validUpdateMetadataWithActiveInactive" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(true, nil)
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb").Return(true, nil)

				return data
			}(),
			validUpdateMetadataWithActiveInactive,
			nil,
			[]int{50, 100},
		},
		{
			"WithActiveError",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}
				data.aim.On("Active").Return(0, fmt.Errorf("active error"))

				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)

				om1 := &objectmock.ObjectMock{}
				om2 := &objectmock.ObjectMock{}
				om3 := &objectmock.ObjectMock{}
				om4 := &objectmock.ObjectMock{}
				data.objs = []metadata.Object{om1, om2, om3, om4}

				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}
				data.scm = &statesmock.Sha256CheckerMock{}

				return data
			}(),
			validUpdateMetadataWithActiveInactive,
			fmt.Errorf("active error"),
			[]int(nil),
		},
		{
			"WithSetActiveError",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}
				data.aim.On("Active").Return(1, nil).Once()
				data.aim.On("SetActive", 0).Return(fmt.Errorf("set active error"))

				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)

				om1 := &objectmock.ObjectMock{}
				om1.On("Setup").Return(nil).Once()
				om1.On("Install", data.uh.Settings.DownloadDir).Return(nil).Once()
				om1.On("Cleanup").Return(nil).Once()

				om2 := &objectmock.ObjectMock{}
				om2.On("Setup").Return(nil).Once()
				om2.On("Install", data.uh.Settings.DownloadDir).Return(nil).Once()
				om2.On("Cleanup").Return(nil).Once()

				om3 := &objectmock.ObjectMock{}
				om4 := &objectmock.ObjectMock{}
				data.objs = []metadata.Object{om1, om2, om3, om4}

				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}
				data.iidm.On("Proceed", om1).Return(true, nil).Once()
				data.iidm.On("Proceed", om2).Return(true, nil).Once()

				// these sha256sum's are from "validUpdateMetadataWithActiveInactive" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(true, nil)
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb").Return(true, nil)

				return data
			}(),
			validUpdateMetadataWithActiveInactive,
			fmt.Errorf("set active error"),
			[]int{50, 100},
		},
		{
			"WithSetupError",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}

				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)

				om := &objectmock.ObjectMock{}
				om.On("Setup").Return(fmt.Errorf("setup error")).Once()
				data.objs = []metadata.Object{om}

				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}

				// these sha256sum's are from "validUpdateMetadata" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa").Return(true, nil)

				return data
			}(),
			validUpdateMetadata,
			fmt.Errorf("setup error"),
			[]int(nil),
		},
		{
			"WithInstallError",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}

				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)

				om := &objectmock.ObjectMock{}
				om.On("Setup").Return(nil).Once()
				om.On("Install", data.uh.Settings.DownloadDir).Return(fmt.Errorf("install error")).Once()
				om.On("Cleanup").Return(nil).Once()

				data.objs = []metadata.Object{om}

				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}
				data.iidm.On("Proceed", om).Return(true, nil).Once()

				// these sha256sum's are from "validUpdateMetadata" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa").Return(true, nil)

				return data
			}(),
			validUpdateMetadata,
			fmt.Errorf("install error"),
			[]int(nil),
		},
		{
			"WithCleanupError",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}

				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)

				om := &objectmock.ObjectMock{}
				om.On("Setup").Return(nil).Once()
				om.On("Install", data.uh.Settings.DownloadDir).Return(nil).Once()
				om.On("Cleanup").Return(fmt.Errorf("cleanup error")).Once()

				data.objs = []metadata.Object{om}

				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}
				data.iidm.On("Proceed", om).Return(true, nil).Once()

				// these sha256sum's are from "validUpdateMetadata" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa").Return(true, nil)

				return data
			}(),
			validUpdateMetadata,
			fmt.Errorf("cleanup error"),
			[]int(nil),
		},
		{
			"WithInstallAndCleanupErrors",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}

				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)

				om := &objectmock.ObjectMock{}
				om.On("Setup").Return(nil).Once()
				om.On("Install", data.uh.Settings.DownloadDir).Return(fmt.Errorf("install error")).Once()
				om.On("Cleanup").Return(fmt.Errorf("cleanup error")).Once()

				data.objs = []metadata.Object{om}

				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}
				data.iidm.On("Proceed", om).Return(true, nil).Once()

				// these sha256sum's are from "validUpdateMetadata" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa").Return(true, nil)

				return data
			}(),
			validUpdateMetadata,
			fmt.Errorf("(install error); (cleanup error)"),
			[]int(nil),
		},
		{
			"WithSha256Error",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}
				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)
				data.objs = []metadata.Object{&objectmock.ObjectMock{}}
				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}

				// these sha256sum's are from "validUpdateMetadata" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa").Return(false, ErrSha256sum)

				return data
			}(),
			validUpdateMetadata,
			ErrSha256sum,
			[]int(nil),
		},
		{
			"WithInvalidSha256",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}
				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)
				data.objs = []metadata.Object{&objectmock.ObjectMock{}}
				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}

				// these sha256sum's are from "validUpdateMetadata" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa").Return(false, nil)

				return data
			}(),
			validUpdateMetadata,
			ErrSha256sum,
			[]int(nil),
		},
		{
			"WithInstallIfDifferentError",
			func() *testData {
				data := &testData{}

				data.fm = &metadata.FirmwareMetadata{
					ProductUID:       "productuid-value",
					DeviceIdentity:   map[string]string{"id1": "id1-value"},
					DeviceAttributes: map[string]string{"attr1": "attr1-value"},
					Hardware:         "",
					Version:          "version-value",
				}

				data.aim = &activeinactivemock.ActiveInactiveMock{}

				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)

				om := &objectmock.ObjectMock{}
				om.On("Setup").Return(nil).Once()
				om.On("Cleanup").Return(nil).Once()

				data.objs = []metadata.Object{om}

				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}
				data.iidm.On("Proceed", om).Return(false, fmt.Errorf("installifdifferent error")).Once()

				// these sha256sum's are from "validUpdateMetadata" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa").Return(true, nil)

				return data
			}(),
			validUpdateMetadata,
			fmt.Errorf("installifdifferent error"),
			[]int(nil),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mode := newTestInstallMode(tc.data.objs)
			defer mode.Unregister()

			tc.data.uh.FirmwareMetadata = *tc.data.fm
			tc.data.uh.InstallIfDifferentBackend = tc.data.iidm
			tc.data.uh.Sha256Checker = tc.data.scm

			updateMetadata, err := metadata.NewUpdateMetadata([]byte(tc.rawUpdateMetadata))
			assert.NoError(t, err)

			progressList, err := startInstallUpdateInAnotherFunc(tc.data.uh, updateMetadata)

			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedProgressList, progressList)

			tc.data.aim.AssertExpectations(t)
			tc.data.iidm.AssertExpectations(t)
			tc.data.scm.AssertExpectations(t)

			for _, obj := range tc.data.objs {
				o := obj.(*objectmock.ObjectMock)
				o.AssertExpectations(t)
			}
		})
	}
}

func TestUpdateHubReportState(t *testing.T) {
	om := &objectmock.ObjectMock{}

	objs := []metadata.Object{om}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	testCases := []struct {
		name                string
		updateMetadata      *metadata.UpdateMetadata
		expectedUMSha256sum string
	}{
		{
			"WithValidUpdateMetadata",
			updateMetadata,
			utils.DataSha256sum([]byte(validUpdateMetadata)),
		},
		{
			"WithNilUpdateMetadata",
			nil,
			"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apiClient := client.NewApiClient("address")

			state := NewDownloadingState(apiClient, updateMetadata, &ProgressTrackerImpl{})
			state.updateMetadata = tc.updateMetadata

			aim := &activeinactivemock.ActiveInactiveMock{}
			rm := &reportermock.ReporterMock{}

			uh, _ := newTestUpdateHub(state, aim)
			uh.Reporter = rm
			uh.previousState = NewIdleState()

			// error the first report
			rm.On("ReportState", apiClient.Request(), tc.expectedUMSha256sum, "idle", "downloading", "", uh.FirmwareMetadata).Return(fmt.Errorf("report error")).Once()

			err := uh.ReportCurrentState()
			assert.EqualError(t, err, "report error")

			// the subsequent reports are successful. "Once()" is
			// important here since the same state shouldn't be
			// reported more than one time in a row
			rm.On("ReportState", apiClient.Request(), tc.expectedUMSha256sum, "idle", "downloading", "", uh.FirmwareMetadata).Return(nil).Once()

			err = uh.ReportCurrentState()
			assert.NoError(t, err)

			err = uh.ReportCurrentState()
			assert.NoError(t, err)

			err = uh.ReportCurrentState()
			assert.NoError(t, err)

			aim.AssertExpectations(t)
			rm.AssertExpectations(t)
		})
	}

	om.AssertExpectations(t)
}

func TestReportCurrentStateNotReportable(t *testing.T) {
	state := NewIdleState()

	aim := &activeinactivemock.ActiveInactiveMock{}
	rm := &reportermock.ReporterMock{}

	uh, _ := newTestUpdateHub(state, aim)
	uh.Reporter = rm

	// since "idle" is not reportable, no error and no report
	// registered
	err := uh.ReportCurrentState()
	assert.NoError(t, err)

	aim.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestStart(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name                 string
		pollingInterval      time.Duration
		extraPollingInterval time.Duration
		firstPoll            time.Time
		lastPoll             time.Time
		expectedState        State
		subTest              func(t *testing.T, uh *UpdateHub, state State)
		probeASAP            bool
	}{
		{
			"RegularPoll",
			time.Second,
			0,
			(time.Time{}).UTC(),
			(time.Time{}).UTC(),
			&PollState{},
			func(t *testing.T, uh *UpdateHub, state State) {
				data, err := afero.ReadFile(uh.Store, uh.Settings.RuntimeSettingsPath)
				assert.NoError(t, err)
				assert.True(t, strings.Contains(string(data), "ProbeASAP=false"))
				assert.True(t, strings.Contains(string(data), "Retries=0"))
				assert.True(t, strings.Contains(string(data), "ExtraInterval=0"))
				// timestamps are relative to "Now()" so just test if they were written
				assert.True(t, strings.Contains(string(data), "FirstPoll="))
				assert.True(t, strings.Contains(string(data), "LastPoll="))
			},
			false,
		},

		{
			"NeverDidPollBefore",
			time.Second,
			0,
			now.Add(-1 * time.Second),
			(time.Time{}).UTC(),
			&ProbeState{},
			func(t *testing.T, uh *UpdateHub, state State) {},
			false,
		},

		{
			"FirstPollAfterNow",
			time.Second,
			0,
			now.Add(24 * time.Second),
			now.Add(-1 * time.Second),
			&PollState{},
			func(t *testing.T, uh *UpdateHub, state State) {
				poll := state.(*PollState)
				assert.WithinDuration(t, uh.Settings.FirstPoll, now, uh.Settings.PollingInterval)
				assert.Condition(t, func() bool { return poll.ticksCount >= 0 })
			},
			false,
		},

		{
			"PendingRegularPoll",
			time.Second,
			0,
			now.Add(-4 * time.Second),
			now.Add(-2 * time.Second),
			&ProbeState{},
			func(t *testing.T, uh *UpdateHub, state State) {},
			false,
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
			false,
		},

		{
			"WithProbeASAPSet",
			time.Second,
			0,
			(time.Time{}).UTC(),
			(time.Time{}).UTC(),
			&ProbeState{},
			func(t *testing.T, uh *UpdateHub, state State) {
				assert.Equal(t, true, uh.Settings.ProbeASAP)

				data, err := afero.ReadFile(uh.Store, uh.Settings.RuntimeSettingsPath)
				assert.NoError(t, err)
				assert.True(t, strings.Contains(string(data), "ProbeASAP=true"))
				assert.True(t, strings.Contains(string(data), "Retries=0"))
				assert.True(t, strings.Contains(string(data), "ExtraInterval=0"))
				// timestamps are relative to "Now()" so just test if they were written
				assert.True(t, strings.Contains(string(data), "FirstPoll="))
				assert.True(t, strings.Contains(string(data), "LastPoll="))
			},
			true,
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

			uh.Settings.PollingInterval = tc.pollingInterval
			uh.Settings.ExtraPollingInterval = tc.extraPollingInterval
			uh.Settings.FirstPoll = tc.firstPoll
			uh.Settings.LastPoll = tc.lastPoll
			uh.Settings.ProbeASAP = tc.probeASAP

			uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

			uh.Start()
			assert.IsType(t, tc.expectedState, uh.GetState())

			tc.subTest(t, uh, uh.GetState())

			aim.AssertExpectations(t)
		})
	}
}

func TestStartWithSuccessfulInstallationValidation(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(0, nil).Once()

	aim.On("SetValidate").Return(nil)

	uh, _ := newTestUpdateHub(nil, aim)

	uh.Settings.ProbeASAP = true
	uh.Settings.UpgradeToInstallation = 0

	cm := &cmdlinemock.CmdLineExecuterMock{}
	uh.CmdLineExecuter = cm

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	// the callback will succeed because the file doesn't exists
	// (assume callback success)
	uh.Start()
	assert.IsType(t, &ProbeState{}, uh.GetState())

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestStartWithSuccessfulInstallationRollback(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(0, nil).Once()

	uh, _ := newTestUpdateHub(nil, aim)

	uh.Settings.ProbeASAP = true
	uh.Settings.UpgradeToInstallation = 1

	cm := &cmdlinemock.CmdLineExecuterMock{}
	uh.CmdLineExecuter = cm

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	// the callback will succeed because the file doesn't exists
	// (assume callback success)
	uh.Start()
	assert.IsType(t, &ProbeState{}, uh.GetState())

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestStartWithFailureToGetActivePartition(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	activeErr := fmt.Errorf("active error")
	aim.On("Active").Return(0, activeErr)

	uh, _ := newTestUpdateHub(nil, aim)

	uh.Settings.UpgradeToInstallation = 0

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	uh.Start()

	err := fmt.Errorf("couldn't get active partition, cannot detect whether the installation is successful or not. Error: %s", activeErr)
	assert.Equal(t, NewErrorState(uh.DefaultApiClient, nil, NewTransientError(err)), uh.GetState())

	aim.AssertExpectations(t)
}

func TestStartWithFailureOnValidateProcedure(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	aim.On("Active").Return(0, nil).Once()
	aim.On("Active").Return(0, nil).Once()
	aim.On("SetActive", 1).Return(nil).Once()

	uh, _ := newTestUpdateHub(nil, aim)

	uh.Settings.UpgradeToInstallation = 0

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	err := afero.WriteFile(uh.Store, uh.ValidateCallbackPath, []byte("dummy content"), 0755)
	assert.NoError(t, err)

	err = fmt.Errorf("validate error")
	cm := &cmdlinemock.CmdLineExecuterMock{}
	cm.On("Execute", uh.ValidateCallbackPath).Return([]byte("output"), err)
	uh.CmdLineExecuter = cm

	rm := &rebootermock.RebooterMock{}
	uh.Rebooter = rm
	rm.On("Reboot").Return(nil)

	uh.Start()

	assert.Equal(t, NewErrorState(uh.DefaultApiClient, nil, NewTransientError(err)), uh.GetState())

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestStartWithFailureOnRollbackProcedure(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	aim.On("Active").Return(0, nil).Once()

	uh, _ := newTestUpdateHub(nil, aim)

	uh.Settings.UpgradeToInstallation = 1

	uh.Store.Remove(uh.Settings.RuntimeSettingsPath)

	err := afero.WriteFile(uh.Store, uh.RollbackCallbackPath, []byte("dummy content"), 0755)
	assert.NoError(t, err)

	err = fmt.Errorf("rollback error")
	cm := &cmdlinemock.CmdLineExecuterMock{}
	cm.On("Execute", uh.RollbackCallbackPath).Return([]byte("output"), err)
	uh.CmdLineExecuter = cm

	rm := &rebootermock.RebooterMock{}
	uh.Rebooter = rm

	uh.Start()

	assert.Equal(t, NewErrorState(uh.DefaultApiClient, nil, NewTransientError(err)), uh.GetState())

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

type testObject struct {
	metadata.ObjectMetadata
}

func newTestInstallMode(objs []metadata.Object) installmodes.InstallMode {
	i := 0
	return installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject: func() interface{} {
			if i < len(objs) {
				i++
				return objs[i-1]
			}

			return fmt.Errorf("not enough registered objects")
		},
	})
}

func newTestUpdateHub(state State, aii activeinactive.Interface) (*UpdateHub, error) {
	fs := afero.NewMemMapFs()
	uh := &UpdateHub{
		Store:                   fs,
		PublicKey:               &testPrivateKey.PublicKey,
		state:                   state,
		TimeStep:                time.Second,
		ActiveInactiveBackend:   aii,
		CmdLineExecuter:         &utils.CmdLine{},
		StateChangeCallbackPath: "/usr/share/updatehub/state-change-callback",
		ErrorCallbackPath:       "/usr/share/updatehub/error-callback",
		ValidateCallbackPath:    "/usr/share/updatehub/validate-callback",
		RollbackCallbackPath:    "/usr/share/updatehub/rollback-callback",
	}

	uh.DefaultApiClient = client.NewApiClient("localhost")

	settings, err := LoadSettings(bytes.NewReader([]byte("")))
	if err != nil {
		return nil, err
	}

	uh.Settings = settings
	uh.Settings.PollingInterval = 1

	return uh, err
}

func TestValidateProcedureWithNonExistantCallback(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	aim.On("SetValidate").Return(nil)

	uh, err := newTestUpdateHub(nil, aim)
	assert.NoError(t, err)

	cm := &cmdlinemock.CmdLineExecuterMock{}
	uh.CmdLineExecuter = cm

	rm := &rebootermock.RebooterMock{}
	uh.Rebooter = rm

	err = uh.validateProcedure()

	assert.NoError(t, err)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestValidateProcedureWithCallbackSuccess(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	aim.On("SetValidate").Return(nil)

	uh, err := newTestUpdateHub(nil, aim)
	assert.NoError(t, err)

	err = afero.WriteFile(uh.Store, uh.ValidateCallbackPath, []byte("dummy content"), 0755)
	assert.NoError(t, err)

	cm := &cmdlinemock.CmdLineExecuterMock{}
	cm.On("Execute", uh.ValidateCallbackPath).Return([]byte("output"), nil)
	uh.CmdLineExecuter = cm

	rm := &rebootermock.RebooterMock{}
	uh.Rebooter = rm

	err = uh.validateProcedure()

	assert.NoError(t, err)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestValidateProcedureWithCallbackFailure(t *testing.T) {
	expectedError := fmt.Errorf("callback error")

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(1, nil)
	aim.On("SetActive", 0).Return(nil)

	uh, err := newTestUpdateHub(nil, aim)
	assert.NoError(t, err)

	uh.DefaultApiClient = client.NewApiClient("address")

	err = afero.WriteFile(uh.Store, uh.ValidateCallbackPath, []byte("dummy content"), 0755)
	assert.NoError(t, err)

	cm := &cmdlinemock.CmdLineExecuterMock{}
	cm.On("Execute", uh.ValidateCallbackPath).Return([]byte("output"), expectedError)
	uh.CmdLineExecuter = cm

	rm := &rebootermock.RebooterMock{}
	rm.On("Reboot").Return(nil)
	uh.Rebooter = rm

	err = uh.validateProcedure()

	assert.Equal(t, expectedError, err)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestValidateProcedureWithActiveFailure(t *testing.T) {
	expectedError := fmt.Errorf("active error")

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(0, expectedError)

	uh, err := newTestUpdateHub(nil, aim)
	assert.NoError(t, err)

	uh.DefaultApiClient = client.NewApiClient("address")

	err = afero.WriteFile(uh.Store, uh.ValidateCallbackPath, []byte("dummy content"), 0755)
	assert.NoError(t, err)

	cm := &cmdlinemock.CmdLineExecuterMock{}
	cm.On("Execute", uh.ValidateCallbackPath).Return([]byte("output"), fmt.Errorf("callback error"))
	uh.CmdLineExecuter = cm

	rm := &rebootermock.RebooterMock{}
	uh.Rebooter = rm

	err = uh.validateProcedure()

	assert.Equal(t, expectedError, err)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestValidateProcedureWithSetActiveFailure(t *testing.T) {
	expectedError := fmt.Errorf("set active error")

	aim := &activeinactivemock.ActiveInactiveMock{}
	aim.On("Active").Return(0, nil)
	aim.On("SetActive", 1).Return(expectedError)

	uh, err := newTestUpdateHub(nil, aim)
	assert.NoError(t, err)

	uh.DefaultApiClient = client.NewApiClient("address")

	err = afero.WriteFile(uh.Store, uh.ValidateCallbackPath, []byte("dummy content"), 0755)
	assert.NoError(t, err)

	cm := &cmdlinemock.CmdLineExecuterMock{}
	cm.On("Execute", uh.ValidateCallbackPath).Return([]byte("output"), fmt.Errorf("callback error"))
	uh.CmdLineExecuter = cm

	rm := &rebootermock.RebooterMock{}
	uh.Rebooter = rm

	err = uh.validateProcedure()

	assert.Equal(t, expectedError, err)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestRollbackProcedureWithNonExistantCallback(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, err := newTestUpdateHub(nil, aim)
	assert.NoError(t, err)

	cm := &cmdlinemock.CmdLineExecuterMock{}
	uh.CmdLineExecuter = cm

	rm := &rebootermock.RebooterMock{}
	uh.Rebooter = rm

	err = uh.rollbackProcedure()

	assert.NoError(t, err)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestRollbackProcedureWithCallbackSuccess(t *testing.T) {
	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, err := newTestUpdateHub(nil, aim)
	assert.NoError(t, err)

	err = afero.WriteFile(uh.Store, uh.RollbackCallbackPath, []byte("dummy content"), 0755)
	assert.NoError(t, err)

	cm := &cmdlinemock.CmdLineExecuterMock{}
	cm.On("Execute", uh.RollbackCallbackPath).Return([]byte("output"), nil)
	uh.CmdLineExecuter = cm

	rm := &rebootermock.RebooterMock{}
	uh.Rebooter = rm

	err = uh.rollbackProcedure()

	assert.NoError(t, err)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestRollbackProcedureWithCallbackFailure(t *testing.T) {
	expectedError := fmt.Errorf("callback error")

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, err := newTestUpdateHub(nil, aim)
	assert.NoError(t, err)

	uh.DefaultApiClient = client.NewApiClient("address")

	err = afero.WriteFile(uh.Store, uh.RollbackCallbackPath, []byte("dummy content"), 0755)
	assert.NoError(t, err)

	cm := &cmdlinemock.CmdLineExecuterMock{}
	cm.On("Execute", uh.RollbackCallbackPath).Return([]byte("output"), expectedError)
	uh.CmdLineExecuter = cm

	rm := &rebootermock.RebooterMock{}
	uh.Rebooter = rm

	err = uh.rollbackProcedure()

	assert.Equal(t, expectedError, err)

	aim.AssertExpectations(t)
	cm.AssertExpectations(t)
	rm.AssertExpectations(t)
}

func TestClearDownloadDir(t *testing.T) {
	om := &objectmock.ObjectMock{}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return om },
	})
	defer mode.Unregister()

	fs := afero.NewMemMapFs()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(NewIdleState(), aim)
	uh.Store = fs

	afero.WriteFile(fs, path.Join(uh.Settings.DownloadDir, "old-file"), []byte("a"), 0755)

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadataWithActiveInactive))
	assert.NoError(t, err)

	objectUID := updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum
	afero.WriteFile(fs, path.Join(uh.Settings.DownloadDir, objectUID), []byte("a"), 0755)

	uh.clearDownloadDir(updateMetadata.Objects[0])

	downloadDir, err := fs.Open(uh.Settings.DownloadDir)
	assert.NoError(t, err)

	expectedFiles := []string{objectUID}

	files, err := downloadDir.Readdirnames(0)
	assert.NoError(t, err)
	assert.Equal(t, expectedFiles, files, "Expected files in download dir should be only objects from update metadata")
}

func TestHasPendingDownload(t *testing.T) {
	om1 := &objectmock.ObjectMock{}
	om2 := &objectmock.ObjectMock{}
	om3 := &objectmock.ObjectMock{}
	om4 := &objectmock.ObjectMock{}

	objs := []metadata.Object{om1, om2, om3, om4}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	fs := afero.NewMemMapFs()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(NewIdleState(), aim)
	uh.Store = fs

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(updateMetadataWithValidSha256sum))
	assert.NoError(t, err)

	pending, err := uh.hasPendingDownload(updateMetadata)
	assert.NoError(t, err)

	assert.True(t, pending)
}

func TestHasNotPendingDownload(t *testing.T) {
	om1 := &objectmock.ObjectMock{}
	om2 := &objectmock.ObjectMock{}

	objs := []metadata.Object{om1, om2}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	fs := afero.NewMemMapFs()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(NewIdleState(), aim)
	uh.Store = fs

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(updateMetadataWithValidSha256sum))
	assert.NoError(t, err)

	for i, obj := range updateMetadata.Objects[0] {
		objectUID := obj.GetObjectMetadata().Sha256sum
		afero.WriteFile(fs, path.Join(uh.Settings.DownloadDir, objectUID), []byte(strconv.Itoa(i)), 0755)
	}

	pending, err := uh.hasPendingDownload(updateMetadata)
	assert.NoError(t, err)

	assert.False(t, pending)
}

func TestHasNotPendingDownloadWithMissingObject(t *testing.T) {
	om1 := &objectmock.ObjectMock{}
	om2 := &objectmock.ObjectMock{}

	objs := []metadata.Object{om1, om2}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	fs := afero.NewMemMapFs()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(NewIdleState(), aim)
	uh.Store = fs

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(updateMetadataWithValidSha256sum))
	assert.NoError(t, err)

	pending, err := uh.hasPendingDownload(updateMetadata)
	assert.NoError(t, err)

	assert.True(t, pending)

	aim.AssertExpectations(t)
}

func TestHasNotPendingDownloadWithInvalidSha256sum(t *testing.T) {
	om1 := &objectmock.ObjectMock{}
	om2 := &objectmock.ObjectMock{}

	objs := []metadata.Object{om1, om2}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	fs := afero.NewMemMapFs()

	aim := &activeinactivemock.ActiveInactiveMock{}

	uh, _ := newTestUpdateHub(NewIdleState(), aim)
	uh.Store = fs

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(updateMetadataWithValidSha256sum))
	assert.NoError(t, err)

	for _, obj := range updateMetadata.Objects[0] {
		objectUID := obj.GetObjectMetadata().Sha256sum
		afero.WriteFile(fs, path.Join(uh.Settings.DownloadDir, objectUID), []byte(""), 0755)
	}

	pending, err := uh.hasPendingDownload(updateMetadata)
	assert.NoError(t, err)

	assert.True(t, pending)
}

func TestHasNotPendingDownloadWithError(t *testing.T) {
	om1 := &objectmock.ObjectMock{}
	om2 := &objectmock.ObjectMock{}
	om3 := &objectmock.ObjectMock{}
	om4 := &objectmock.ObjectMock{}

	objs := []metadata.Object{om1, om2, om3, om4}

	mode := newTestInstallMode(objs)
	defer mode.Unregister()

	fs := afero.NewMemMapFs()

	expectedError := errors.New("error")

	aim := &activeinactivemock.ActiveInactiveMock{}

	aim.On("Active").Return(0, expectedError).Once()

	uh, _ := newTestUpdateHub(NewIdleState(), aim)
	uh.Store = fs

	updateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadataWithActiveInactive))
	assert.NoError(t, err)

	pending, err := uh.hasPendingDownload(updateMetadata)
	assert.Error(t, expectedError, err)

	assert.False(t, pending)

	aim.AssertExpectations(t)
}
