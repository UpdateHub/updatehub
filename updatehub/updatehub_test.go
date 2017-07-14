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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bouk/monkey"
	"github.com/go-ini/ini"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"

	"github.com/UpdateHub/updatehub/activeinactive"
	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/installmodes/imxkobs"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/activeinactivemock"
	"github.com/UpdateHub/updatehub/testsmocks/copymock"
	"github.com/UpdateHub/updatehub/testsmocks/filemock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/UpdateHub/updatehub/testsmocks/installifdifferentmock"
	"github.com/UpdateHub/updatehub/testsmocks/objectmock"
	"github.com/UpdateHub/updatehub/testsmocks/reportermock"
	"github.com/UpdateHub/updatehub/testsmocks/statesmock"
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
)

func startFetchUpdateInAnotherFunc(uh *UpdateHub, um *metadata.UpdateMetadata) ([]int, error) {
	var progressList []int
	var err error

	progressChan := make(chan int, 10)

	m := sync.Mutex{}
	m.Lock()

	go func() {
		m.Lock()
		defer m.Unlock()

		err = uh.FetchUpdate(um, nil, progressChan)
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

func TestCheckDownloadedObjectSha256sum(t *testing.T) {
	memFs := afero.NewMemMapFs()
	testPath, err := afero.TempDir(memFs, "", "states-test")

	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	expectedSha256sum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	err = afero.WriteFile(memFs, path.Join(testPath, expectedSha256sum), []byte("test"), 0666)
	assert.NoError(t, err)

	sci := &Sha256CheckerImpl{}
	err = sci.CheckDownloadedObjectSha256sum(memFs, testPath, expectedSha256sum)
	assert.NoError(t, err)
}

func TestCheckDownloadedObjectSha256sumWithOpenError(t *testing.T) {
	dummyPath := "/dummy"
	dummySha256sum := "dummy_hash"

	fsm := &filesystemmock.FileSystemBackendMock{}
	fsm.On("Open", path.Join(dummyPath, dummySha256sum)).Return(&filemock.FileMock{}, fmt.Errorf("open error"))

	sci := &Sha256CheckerImpl{}
	err := sci.CheckDownloadedObjectSha256sum(fsm, dummyPath, dummySha256sum)
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
	err = sci.CheckDownloadedObjectSha256sum(memFs, testPath, expectedSha256sum)
	assert.EqualError(t, err, "sha256sum's don't match. Expected: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08 / Calculated: ae448ac86c4e8e4dec645729708ef41873ae79c6dff84eff73360989487f08e5")
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

	// these sha256sum's are from "validUpdateMetadataWithThreeObjects" content

	// obj1
	objectUID1 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	uri1 := path.Join("/products", uh.FirmwareMetadata.ProductUID, "packages", packageUID, "objects", objectUID1)

	source1 := &filemock.FileMock{}
	source1.On("Close").Return(nil).Once()
	source1Content := []byte("content1")

	um.On("FetchUpdate", uh.API.Request(), uri1).Return(source1, int64(len(source1Content)), nil).Once()

	// obj2
	objectUID2 := "b9632efa90820ff35d6cec0946f99bb8a6317b1e2ef877e501a3e12b2d04d0ae"
	uri2 := path.Join("/products", uh.FirmwareMetadata.ProductUID, "packages", packageUID, "objects", objectUID2)

	source2 := &filemock.FileMock{}
	source2.On("Close").Return(nil).Once()
	source2Content := []byte("content2")

	um.On("FetchUpdate", uh.API.Request(), uri2).Return(source2, int64(len(source2Content)), nil).Once()

	// obj3
	objectUID3 := "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa"
	uri3 := path.Join("/products", uh.FirmwareMetadata.ProductUID, "/packages", packageUID, "objects", objectUID3)

	source3 := &filemock.FileMock{}
	source3.On("Close").Return(nil).Once()
	source3Content := []byte("content3")

	um.On("FetchUpdate", uh.API.Request(), uri3).Return(source3, int64(len(source3Content)), nil).Once()

	// setup filesystembackend

	cpm := &copymock.CopyMock{}
	uh.CopyBackend = cpm

	fsm := &filesystemmock.FileSystemBackendMock{}
	uh.Store = fsm

	// file1
	target1 := &filemock.FileMock{}
	target1.On("Close").Return(nil).Once()
	cpm.On("Copy", target1, source1, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil).Once()
	fsm.On("Create", path.Join(uh.Settings.DownloadDir, objectUID1)).Return(target1, nil).Once()

	// file2
	target2 := &filemock.FileMock{}
	target2.On("Close").Return(nil).Once()
	cpm.On("Copy", target2, source2, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil).Once()
	fsm.On("Create", path.Join(uh.Settings.DownloadDir, objectUID2)).Return(target2, nil).Once()

	// file3
	target3 := &filemock.FileMock{}
	target3.On("Close").Return(nil).Once()
	cpm.On("Copy", target3, source3, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil).Once()
	fsm.On("Create", path.Join(uh.Settings.DownloadDir, objectUID3)).Return(target3, nil).Once()

	progressList, err := startFetchUpdateInAnotherFunc(uh, updateMetadata)

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

func TestUpdateHubFetchUpdateWithTargetFileError(t *testing.T) {
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
	fsm.On("Create", path.Join(uh.Settings.DownloadDir, objectUID)).Return((*filemock.FileMock)(nil), fmt.Errorf("create error"))
	uh.Store = fsm

	progressList, err := startFetchUpdateInAnotherFunc(uh, updateMetadata)

	assert.EqualError(t, err, "create error")
	assert.Equal(t, []int(nil), progressList)

	aim.AssertExpectations(t)
	cpm.AssertExpectations(t)
	fsm.AssertExpectations(t)
	um.AssertExpectations(t)
	om.AssertExpectations(t)
}

func TestUpdateHubFetchUpdateWithUpdaterError(t *testing.T) {
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

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadata))
	objectUID := updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum

	uri := path.Join("/products", uh.FirmwareMetadata.ProductUID, "packages", packageUID, "objects", objectUID)

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
	fsm.On("Create", path.Join(uh.Settings.DownloadDir, objectUID)).Return(target, nil)
	uh.Store = fsm

	progressList, err := startFetchUpdateInAnotherFunc(uh, updateMetadata)

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

func TestUpdateHubFetchUpdateWithCopyError(t *testing.T) {
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

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadata))
	objectUID := updateMetadata.Objects[0][0].GetObjectMetadata().Sha256sum

	uri := path.Join("/products", uh.FirmwareMetadata.ProductUID, "packages", packageUID, "objects", objectUID)

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
	fsm.On("Create", path.Join(uh.Settings.DownloadDir, objectUID)).Return(target, nil)
	uh.Store = fsm

	progressList, err := startFetchUpdateInAnotherFunc(uh, updateMetadata)

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

func TestUpdateHubFetchUpdateWithActiveInactive(t *testing.T) {
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

	packageUID := utils.DataSha256sum([]byte(validUpdateMetadataWithActiveInactive))

	expectedURIPrefix := "/products"
	expectedURIPrefix = path.Join(expectedURIPrefix, uh.FirmwareMetadata.ProductUID)
	expectedURIPrefix = path.Join(expectedURIPrefix, "packages")
	expectedURIPrefix = path.Join(expectedURIPrefix, packageUID)
	expectedURIPrefix = path.Join(expectedURIPrefix, "objects")

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
	fsm.On("Create", path.Join(uh.Settings.DownloadDir, objectUIDFirst)).Return(target1, nil)
	fsm.On("Create", path.Join(uh.Settings.DownloadDir, objectUIDSecond)).Return(target2, nil)
	uh.Store = fsm

	// finish setup
	uh.Updater = um

	cpm := &copymock.CopyMock{}
	cpm.On("Copy", target1, source1, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil)
	cpm.On("Copy", target2, source2, 30*time.Second, (<-chan bool)(nil), utils.ChunkSize, 0, -1, false).Return(false, nil)
	uh.CopyBackend = cpm

	progressList, err := startFetchUpdateInAnotherFunc(uh, updateMetadata)

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

func TestUpdateHubFetchUpdateWithActiveInactiveError(t *testing.T) {
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

	progressList, err := startFetchUpdateInAnotherFunc(uh, updateMetadata)

	assert.EqualError(t, err, "update metadata must have 1 or 2 objects. Found 0")
	assert.Equal(t, []int(nil), progressList)

	aim.AssertExpectations(t)
	um.AssertExpectations(t)
	om.AssertExpectations(t)
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
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(nil)
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "b9632efa90820ff35d6cec0946f99bb8a6317b1e2ef877e501a3e12b2d04d0ae").Return(nil)
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa").Return(nil)

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
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(nil)
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb").Return(nil)

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
				data.aim.On("Active").Return(1, nil)
				data.aim.On("SetActive", 0).Return(fmt.Errorf("set active error"))

				data.uh, _ = newTestUpdateHub(&PollState{}, data.aim)

				om1 := &objectmock.ObjectMock{}
				om1.On("Setup").Return(nil).Once()
				om1.On("Install", data.uh.Settings.DownloadDir).Return(nil).Once()
				om1.On("Cleanup").Return(nil).Once()

				om2 := &objectmock.ObjectMock{}
				om3 := &objectmock.ObjectMock{}
				om4 := &objectmock.ObjectMock{}
				data.objs = []metadata.Object{om1, om2, om3, om4}

				data.iidm = &installifdifferentmock.InstallIfDifferentMock{}
				data.iidm.On("Proceed", om1).Return(true, nil)

				// these sha256sum's are from "validUpdateMetadataWithActiveInactive" content
				data.scm = &statesmock.Sha256CheckerMock{}
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(nil)

				return data
			}(),
			validUpdateMetadataWithActiveInactive,
			fmt.Errorf("set active error"),
			[]int(nil),
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
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(nil)

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
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(nil)

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
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(nil)

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
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(nil)

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
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(fmt.Errorf("sha256 error"))

				return data
			}(),
			validUpdateMetadata,
			fmt.Errorf("sha256 error"),
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
				data.scm.On("CheckDownloadedObjectSha256sum", data.uh.Store, data.uh.Settings.DownloadDir, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855").Return(nil)

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
			state := NewDownloadingState(updateMetadata, &ProgressTrackerImpl{})
			state.updateMetadata = tc.updateMetadata

			aim := &activeinactivemock.ActiveInactiveMock{}
			rm := &reportermock.ReporterMock{}

			uh, _ := newTestUpdateHub(state, aim)
			uh.Reporter = rm

			// error the first report
			rm.On("ReportState", uh.API.Request(), tc.expectedUMSha256sum, "downloading").Return(fmt.Errorf("report error")).Once()

			err := uh.ReportCurrentState()
			assert.EqualError(t, err, "report error")

			// the subsequent reports are successful. "Once()" is
			// important here since the same state shouldn't be
			// reported more than one time in a row
			rm.On("ReportState", uh.API.Request(), tc.expectedUMSha256sum, "downloading").Return(nil).Once()

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

			uh.Settings.PollingInterval = tc.pollingInterval
			uh.Settings.ExtraPollingInterval = tc.extraPollingInterval
			uh.Settings.FirstPoll = tc.firstPoll
			uh.Settings.LastPoll = tc.lastPoll

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

			tc.subTest(t, uh.Settings, err)

			aim.AssertExpectations(t)
		})
	}
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
		Store:    fs,
		State:    state,
		TimeStep: time.Second,
		API:      client.NewApiClient("localhost"),
		ActiveInactiveBackend: aii,
	}

	settings, err := LoadSettings(bytes.NewReader([]byte("")))
	if err != nil {
		return nil, err
	}

	uh.Settings = settings
	uh.Settings.PollingInterval = 1

	return uh, err
}
