/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package copy

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/UpdateHub/updatehub/copy"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/testsmocks/copymock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/UpdateHub/updatehub/testsmocks/libarchivemock"
	"github.com/UpdateHub/updatehub/testsmocks/permissionsmock"
	"github.com/UpdateHub/updatehub/utils"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestCopyInit(t *testing.T) {
	val, err := installmodes.GetObject("copy")
	assert.NoError(t, err)

	cp1, ok := val.(*CopyObject)
	if !ok {
		t.Error("Failed to cast return value of \"installmodes.GetObject()\" to CopyObject")
	}

	osFs := afero.NewOsFs()
	cmdline := &utils.CmdLine{}
	cp2 := &CopyObject{
		FileSystemHelper: &utils.FileSystem{
			CmdLineExecuter: cmdline,
		},
		LibArchiveBackend: &libarchive.LibArchive{},
		FileSystemBackend: osFs,
		CopyBackend:       &copy.ExtendedIO{},
		Permissions:       &utils.PermissionsDefaultImpl{},
		ChunkSize:         128 * 1024,
	}

	assert.Equal(t, cp2, cp1)
}

func TestCopySetupWithSuccess(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}
	cm := &copymock.CopyMock{}

	targetDevice := "/dev/xx1"
	fsType := "ext4"
	formatOptions := "-y"
	targetType := "device"
	targetPath := "/kernel"
	tempDirPath := "/temp-dir-path"
	mountOptions := "-o rw"

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", memFs, "copy-handler").Return(tempDirPath, nil)
	fsm.On("Format", targetDevice, fsType, formatOptions).Return(nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)

	cp := CopyObject{
		FileSystemHelper:  fsm,
		CopyBackend:       cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
		Target:            targetDevice,
		TargetType:        targetType,
		TargetPath:        targetPath,
		FSType:            fsType,
		MountOptions:      mountOptions,
		FormatOptions:     formatOptions,
		MustFormat:        true,
	}

	err := cp.Setup()

	assert.NoError(t, err)
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	assert.Equal(t, "/temp-dir-path/kernel", cp.GetTarget())
}

func TestCopySetupWithTempDirError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}
	cm := &copymock.CopyMock{}

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", memFs, "copy-handler").Return("", fmt.Errorf("temp dir error"))
	cp := CopyObject{FileSystemHelper: fsm, CopyBackend: cm, FileSystemBackend: memFs, LibArchiveBackend: lam}

	cp.TargetType = "device"
	err := cp.Setup()

	assert.EqualError(t, err, "temp dir error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	assert.Equal(t, "", cp.GetTarget())
}

func TestCopySetupWithNotSupportedTargetTypes(t *testing.T) {
	cp := CopyObject{}

	cp.TargetType = "ubivolume"
	err := cp.Setup()
	assert.EqualError(t, err, "target-type 'ubivolume' is not supported for the 'copy' handler. Its value must be 'device'")

	cp.TargetType = "mtdname"
	err = cp.Setup()
	assert.EqualError(t, err, "target-type 'mtdname' is not supported for the 'copy' handler. Its value must be 'device'")

	cp.TargetType = "someother"
	err = cp.Setup()
	assert.EqualError(t, err, "target-type 'someother' is not supported for the 'copy' handler. Its value must be 'device'")
}

func TestCopySetupWithFormatError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}
	cm := &copymock.CopyMock{}

	targetDevice := "/dev/xx1"
	fsType := "ext4"
	formatOptions := "-y"
	tempDirPath := "/temp-dir-path"

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", memFs, "copy-handler").Return(tempDirPath, nil)
	fsm.On("Format", targetDevice, fsType, formatOptions).Return(fmt.Errorf("format error"))
	cp := CopyObject{FileSystemHelper: fsm, CopyBackend: cm, FileSystemBackend: memFs, LibArchiveBackend: lam}
	cp.MustFormat = true
	cp.Target = targetDevice
	cp.FSType = fsType
	cp.FormatOptions = formatOptions
	cp.TargetType = "device"

	err := cp.Setup()

	assert.EqualError(t, err, "format error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	assert.Equal(t, "", cp.GetTarget())
}

func TestCopySetupWithMountError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}
	cm := &copymock.CopyMock{}

	targetDevice := "/dev/xx1"
	fsType := "ext4"
	mountOptions := "-o rw"

	tempDirPath := "/dummy-path"

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", memFs, "copy-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(fmt.Errorf("mount error"))
	cp := CopyObject{FileSystemHelper: fsm, CopyBackend: cm, FileSystemBackend: memFs, LibArchiveBackend: lam}
	cp.Target = targetDevice
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.tempDirPath = tempDirPath
	cp.TargetPath = "/kernel"
	cp.TargetType = "device"

	err := cp.Setup()

	assert.EqualError(t, err, "mount error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	expectedTargetPath := path.Join(tempDirPath, cp.TargetPath)
	assert.Equal(t, expectedTargetPath, cp.GetTarget())
}

func TestCopyInstallWithMkdirAllError(t *testing.T) {
	expectedError := fmt.Errorf("mkdirall error")

	lam := &libarchivemock.LibArchiveMock{}
	fsbm := &filesystemmock.FileSystemBackendMock{}

	fsbm.On("MkdirAll", "/dummy-path/subdir", os.FileMode(0755)).Return(expectedError)

	targetDevice := "/dev/xx1"
	targetPath := "/subdir/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226"
	compressed := false

	tempDirPath := "/dummy-path"

	fsm := &filesystemmock.FileSystemHelperMock{}

	downloadDir := "/dummy-download-dir"

	cm := &copymock.CopyMock{}

	cp := CopyObject{
		FileSystemHelper:  fsm,
		CopyBackend:       cm,
		FileSystemBackend: fsbm,
		LibArchiveBackend: lam,
		ChunkSize:         128 * 1024,
	}
	cp.Target = targetDevice
	cp.TargetPath = targetPath
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.Sha256sum = sha256sum
	cp.Compressed = compressed
	cp.tempDirPath = tempDirPath

	err := cp.Install(downloadDir)

	assert.Equal(t, expectedError, err)
	fsm.AssertExpectations(t)
	fsbm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	expectedTargetPath := path.Join(tempDirPath, targetPath)
	assert.Equal(t, expectedTargetPath, cp.GetTarget())
}

func TestCopyInstallWithCopyFileError(t *testing.T) {
	lam := &libarchivemock.LibArchiveMock{}
	fsbm := &filesystemmock.FileSystemBackendMock{}

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226"
	compressed := false

	tempDirPath := "/dummy-path"
	fsbm.On("MkdirAll", tempDirPath, os.FileMode(0755)).Return(nil)

	fsm := &filesystemmock.FileSystemHelperMock{}

	downloadDir := "/dummy-download-dir"

	cm := &copymock.CopyMock{}
	cm.On("CopyFile", fsbm, lam, path.Join(downloadDir, sha256sum), path.Join(tempDirPath, targetPath), 128*1024, 0, 0, -1, true, compressed).Return(fmt.Errorf("copy file error"))

	cp := CopyObject{
		FileSystemHelper:  fsm,
		CopyBackend:       cm,
		FileSystemBackend: fsbm,
		LibArchiveBackend: lam,
		ChunkSize:         128 * 1024,
	}
	cp.Target = targetDevice
	cp.TargetPath = targetPath
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.Sha256sum = sha256sum
	cp.Compressed = compressed
	cp.tempDirPath = tempDirPath

	err := cp.Install(downloadDir)

	assert.EqualError(t, err, "copy file error")
	fsm.AssertExpectations(t)
	fsbm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	expectedTargetPath := path.Join(tempDirPath, targetPath)
	assert.Equal(t, expectedTargetPath, cp.GetTarget())
}

func TestCopyInstallWithChmodAndChownErrors(t *testing.T) {
	fsbm := &filesystemmock.FileSystemBackendMock{}
	lam := &libarchivemock.LibArchiveMock{}

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226"
	compressed := false
	mode := "0644"
	var uid, gid interface{}
	uid = "user"
	gid = "group"

	tempDirPath := "/dummy-path"
	fsbm.On("MkdirAll", tempDirPath, os.FileMode(0755)).Return(nil)

	fsm := &filesystemmock.FileSystemHelperMock{}

	downloadDir := "/dummy-download-dir"

	cm := &copymock.CopyMock{}
	cm.On("CopyFile", fsbm, lam, path.Join(downloadDir, sha256sum), path.Join(tempDirPath, targetPath), 128*1024, 0, 0, -1, true, compressed).Return(nil)

	pm := &permissionsmock.PermissionsMock{}
	pm.On("ApplyChmod", fsbm, path.Join(tempDirPath, targetPath), mode).Return(fmt.Errorf("chmod error"))
	pm.On("ApplyChown", path.Join(tempDirPath, targetPath), uid, gid).Return(fmt.Errorf("chown error"))

	cp := CopyObject{
		FileSystemHelper:  fsm,
		CopyBackend:       cm,
		Permissions:       pm,
		FileSystemBackend: fsbm,
		LibArchiveBackend: lam,
		ChunkSize:         128 * 1024,
		TargetMode:        mode,
		TargetUID:         uid,
		TargetGID:         gid,
	}
	cp.Target = targetDevice
	cp.TargetPath = targetPath
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.Sha256sum = sha256sum
	cp.Compressed = compressed
	cp.tempDirPath = tempDirPath

	err := cp.Install(downloadDir)

	assert.EqualError(t, err, "(chmod error); (chown error)")
	fsm.AssertExpectations(t)
	fsbm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)
	pm.AssertExpectations(t)

	expectedTargetPath := path.Join(tempDirPath, targetPath)
	assert.Equal(t, expectedTargetPath, cp.GetTarget())
}

func TestCopyInstallWithSuccess(t *testing.T) {
	testCases := []struct {
		Name          string
		Sha256sum     string
		Target        string
		TargetType    string
		TargetPath    string
		TargetMode    string
		TargetUID     interface{}
		TargetGID     interface{}
		FSType        string
		FormatOptions string
		MustFormat    bool
		MountOptions  string
		ChunkSize     int
		Compressed    bool
	}{
		{
			"WithAllFields",
			"2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226",
			"/dev/xx1",
			"device",
			"/inner-path",
			"0777",
			"user",
			"group",
			"ext4",
			"-y",
			true,
			"-o rw",
			2048,
			false,
		},
		{
			"WithSubdirInnerPath",
			"2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226",
			"/dev/xx1",
			"device",
			"/subdir/inner-path",
			"0777",
			"user",
			"group",
			"ext4",
			"-y",
			true,
			"-o rw",
			2048,
			false,
		},
		{
			"WithNegativeChunkSize",
			"2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226",
			"/dev/xx1",
			"device",
			"/inner-path",
			"0777",
			"user",
			"group",
			"ext4",
			"-y",
			true,
			"-o rw",
			-1,
			false,
		},
		{
			"WithChunkSizeZero",
			"2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226",
			"/dev/xx1",
			"device",
			"/inner-path",
			"0777",
			"user",
			"group",
			"ext4",
			"",
			false,
			"-o rw",
			0,
			false,
		},
		{
			"WithCompressed",
			"2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226",
			"/dev/xx1",
			"device",
			"/inner-path",
			"0777",
			"user",
			"group",
			"ext4",
			"",
			false,
			"-o rw",
			2048,
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			fsbm := &filesystemmock.FileSystemBackendMock{}
			lam := &libarchivemock.LibArchiveMock{}

			tempDirPath := "/dummy-path"

			fsbm.On("MkdirAll", filepath.Dir(path.Join(tempDirPath, tc.TargetPath)), os.FileMode(0755)).Return(nil)

			fsm := &filesystemmock.FileSystemHelperMock{}

			downloadDir := "/dummy-download-dir"

			cm := &copymock.CopyMock{}
			cm.On("CopyFile", fsbm, lam, path.Join(downloadDir, tc.Sha256sum), path.Join(tempDirPath, tc.TargetPath), tc.ChunkSize, 0, 0, -1, true, tc.Compressed).Return(nil)

			pm := &permissionsmock.PermissionsMock{}
			pm.On("ApplyChmod", fsbm, path.Join(tempDirPath, tc.TargetPath), tc.TargetMode).Return(nil)
			pm.On("ApplyChown", path.Join(tempDirPath, tc.TargetPath), tc.TargetUID, tc.TargetGID).Return(nil)

			cp := CopyObject{
				FileSystemHelper:  fsm,
				CopyBackend:       cm,
				Permissions:       pm,
				FileSystemBackend: fsbm,
				LibArchiveBackend: lam,
				Target:            tc.Target,
				TargetType:        tc.TargetType,
				TargetPath:        tc.TargetPath,
				TargetMode:        tc.TargetMode,
				TargetUID:         tc.TargetUID,
				TargetGID:         tc.TargetGID,
				FSType:            tc.FSType,
				MountOptions:      tc.MountOptions,
				FormatOptions:     tc.FormatOptions,
				MustFormat:        tc.MustFormat,
				ChunkSize:         tc.ChunkSize,
			}
			cp.Sha256sum = tc.Sha256sum
			cp.Compressed = tc.Compressed
			cp.tempDirPath = tempDirPath

			err := cp.Install(downloadDir)

			assert.NoError(t, err)
			fsm.AssertExpectations(t)
			fsbm.AssertExpectations(t)
			cm.AssertExpectations(t)
			lam.AssertExpectations(t)
			pm.AssertExpectations(t)

			expectedTargetPath := path.Join(tempDirPath, tc.TargetPath)
			assert.Equal(t, expectedTargetPath, cp.GetTarget())
		})
	}
}

func TestCopyCleanupWithUmountError(t *testing.T) {
	fsbm := &filesystemmock.FileSystemBackendMock{}
	lam := &libarchivemock.LibArchiveMock{}

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226"
	compressed := false
	mode := "0644"
	var uid, gid interface{}
	uid = "user"
	gid = "group"

	tempDirPath := "/dummy-path"

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("Umount", tempDirPath).Return(fmt.Errorf("umount error"))

	cm := &copymock.CopyMock{}

	pm := &permissionsmock.PermissionsMock{}

	cp := CopyObject{
		FileSystemHelper:  fsm,
		CopyBackend:       cm,
		Permissions:       pm,
		FileSystemBackend: fsbm,
		LibArchiveBackend: lam,
		ChunkSize:         128 * 1024,
		TargetMode:        mode,
		TargetUID:         uid,
		TargetGID:         gid,
	}
	cp.Target = targetDevice
	cp.TargetPath = targetPath
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.Sha256sum = sha256sum
	cp.Compressed = compressed
	cp.tempDirPath = tempDirPath

	err := cp.Cleanup()

	assert.EqualError(t, err, "umount error")
	fsm.AssertExpectations(t)
	fsbm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)
	pm.AssertExpectations(t)

	expectedTargetPath := path.Join(tempDirPath, targetPath)
	assert.Equal(t, expectedTargetPath, cp.GetTarget())
}

func TestCopyCleanupWithSuccess(t *testing.T) {
	tempDirPath := "/dummy-path"

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("Umount", tempDirPath).Return(nil)

	fsbm := &filesystemmock.FileSystemBackendMock{}
	fsbm.On("RemoveAll", tempDirPath).Return(nil)

	cp := CopyObject{FileSystemBackend: fsbm, FileSystemHelper: fsm}
	cp.tempDirPath = tempDirPath
	assert.Nil(t, cp.Cleanup())

	fsbm.AssertExpectations(t)
	fsm.AssertExpectations(t)
}

func TestCopyGetTarget(t *testing.T) {
	cp := CopyObject{}
	assert.Equal(t, "", cp.GetTarget())

	cp.tempDirPath = "/prefix"
	cp.TargetPath = "/file"
	assert.Equal(t, "/prefix/file", cp.GetTarget())
}
