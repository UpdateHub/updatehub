/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package copy

import (
	"fmt"
	"path"
	"testing"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/testsmocks/copymock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/UpdateHub/updatehub/testsmocks/libarchivemock"
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
	cp2 := &CopyObject{
		FileSystemHelper:  &utils.FileSystem{},
		LibArchiveBackend: &libarchive.LibArchive{},
		FileSystemBackend: osFs,
		Copier:            &utils.ExtendedIO{},
		ChunkSize:         128 * 1024,
	}

	assert.Equal(t, cp2, cp1)
}

func TestCopySetupWithSuccess(t *testing.T) {
	cp := CopyObject{}
	cp.TargetType = "device"
	err := cp.Setup()
	assert.NoError(t, err)
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

func TestCopyInstallWithFormatError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}
	cm := &copymock.CopierMock{}

	targetDevice := "/dev/xx1"
	fsType := "ext4"
	formatOptions := "-y"

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("Format", targetDevice, fsType, formatOptions).Return(fmt.Errorf("format error"))
	cp := CopyObject{FileSystemHelper: fsm, Copier: cm, FileSystemBackend: memFs, LibArchiveBackend: lam}
	cp.MustFormat = true
	cp.Target = targetDevice
	cp.FSType = fsType
	cp.FormatOptions = formatOptions

	err := cp.Install("/dummy-download-dir")

	assert.EqualError(t, err, "format error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	assert.Equal(t, "", cp.GetTarget())
}

func TestCopyInstallWithTempDirError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}
	cm := &copymock.CopierMock{}

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "copy-handler").Return("", fmt.Errorf("temp dir error"))
	cp := CopyObject{FileSystemHelper: fsm, Copier: cm, FileSystemBackend: memFs, LibArchiveBackend: lam}

	err := cp.Install("/dummy-download-dir")

	assert.EqualError(t, err, "temp dir error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	assert.Equal(t, "", cp.GetTarget())
}

func TestCopyInstallWithMountError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}
	cm := &copymock.CopierMock{}

	tempDirPath, err := afero.TempDir(memFs, "", "copy-handler")
	assert.NoError(t, err)

	targetDevice := "/dev/xx1"
	fsType := "ext4"
	mountOptions := "-o rw"

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "copy-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(fmt.Errorf("mount error"))
	cp := CopyObject{FileSystemHelper: fsm, Copier: cm, FileSystemBackend: memFs, LibArchiveBackend: lam}
	cp.Target = targetDevice
	cp.FSType = fsType
	cp.MountOptions = mountOptions

	err = cp.Install("/dummy-download-dir")

	assert.EqualError(t, err, "mount error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	tempDirExists, err := afero.Exists(memFs, tempDirPath)
	assert.False(t, tempDirExists)
	assert.NoError(t, err)

	assert.Equal(t, "", cp.GetTarget())
}

func TestCopyInstallWithCopyFileError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}

	tempDirPath, err := afero.TempDir(memFs, "", "copy-handler")
	assert.NoError(t, err)

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226"
	compressed := false

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "copy-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)
	fsm.On("Umount", tempDirPath).Return(nil)

	downloadDir := "/dummy-download-dir"

	cm := &copymock.CopierMock{}
	cm.On("CopyFile", memFs, lam, path.Join(downloadDir, sha256sum), path.Join(tempDirPath, targetPath), 128*1024, 0, 0, -1, true, compressed).Return(fmt.Errorf("copy file error"))

	cp := CopyObject{
		FileSystemHelper:  fsm,
		Copier:            cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
		ChunkSize:         128 * 1024,
	}
	cp.Target = targetDevice
	cp.TargetPath = targetPath
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.Sha256sum = sha256sum
	cp.Compressed = compressed

	err = cp.Install(downloadDir)

	assert.EqualError(t, err, "copy file error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	tempDirExists, err := afero.Exists(memFs, tempDirPath)
	assert.False(t, tempDirExists)
	assert.NoError(t, err)

	expectedTargetPath := path.Join(tempDirPath, targetPath)
	assert.Equal(t, expectedTargetPath, cp.GetTarget())
}

func TestCopyInstallWithUmountError(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}

	tempDirPath, err := afero.TempDir(memFs, "", "copy-handler")
	assert.NoError(t, err)

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226"
	compressed := false

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "copy-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)
	fsm.On("Umount", tempDirPath).Return(fmt.Errorf("umount error"))

	downloadDir := "/dummy-download-dir"

	cm := &copymock.CopierMock{}
	cm.On("CopyFile", memFs, lam, path.Join(downloadDir, sha256sum), path.Join(tempDirPath, targetPath), 128*1024, 0, 0, -1, true, compressed).Return(nil)

	cp := CopyObject{
		FileSystemHelper:  fsm,
		Copier:            cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
		ChunkSize:         128 * 1024,
	}
	cp.Target = targetDevice
	cp.TargetPath = targetPath
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.Sha256sum = sha256sum
	cp.Compressed = compressed

	err = cp.Install(downloadDir)

	assert.EqualError(t, err, "umount error")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	tempDirExists, err := afero.Exists(memFs, tempDirPath)
	assert.True(t, tempDirExists)
	assert.NoError(t, err)

	expectedTargetPath := path.Join(tempDirPath, targetPath)
	assert.Equal(t, expectedTargetPath, cp.GetTarget())
}

func TestCopyInstallWithCopyFileANDUmountErrors(t *testing.T) {
	memFs := afero.NewMemMapFs()
	lam := &libarchivemock.LibArchiveMock{}

	tempDirPath, err := afero.TempDir(memFs, "", "copy-handler")
	assert.NoError(t, err)

	targetDevice := "/dev/xx1"
	targetPath := "/inner-path"
	fsType := "ext4"
	mountOptions := "-o rw"
	sha256sum := "2ab0cfa4332841d4de81ea738d641ef943ddec60a6f4638adcc0091f5345a226"
	compressed := false

	fsm := &filesystemmock.FileSystemHelperMock{}
	fsm.On("TempDir", "copy-handler").Return(tempDirPath, nil)
	fsm.On("Mount", targetDevice, tempDirPath, fsType, mountOptions).Return(nil)
	fsm.On("Umount", tempDirPath).Return(fmt.Errorf("umount error"))

	downloadDir := "/dummy-download-dir"

	cm := &copymock.CopierMock{}
	cm.On("CopyFile", memFs, lam, path.Join(downloadDir, sha256sum), path.Join(tempDirPath, targetPath), 128*1024, 0, 0, -1, true, compressed).Return(fmt.Errorf("copy file error"))

	cp := CopyObject{
		FileSystemHelper:  fsm,
		Copier:            cm,
		FileSystemBackend: memFs,
		LibArchiveBackend: lam,
		ChunkSize:         128 * 1024,
	}
	cp.Target = targetDevice
	cp.TargetPath = targetPath
	cp.FSType = fsType
	cp.MountOptions = mountOptions
	cp.Sha256sum = sha256sum
	cp.Compressed = compressed

	err = cp.Install(downloadDir)

	assert.EqualError(t, err, "(copy file error); (umount error)")
	fsm.AssertExpectations(t)
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)

	tempDirExists, err := afero.Exists(memFs, tempDirPath)
	assert.True(t, tempDirExists)
	assert.NoError(t, err)

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
			memFs := afero.NewMemMapFs()
			lam := &libarchivemock.LibArchiveMock{}

			tempDirPath, err := afero.TempDir(memFs, "", "copy-handler")
			assert.NoError(t, err)

			fsm := &filesystemmock.FileSystemHelperMock{}
			if tc.MustFormat {
				fsm.On("Format", tc.Target, tc.FSType, tc.FormatOptions).Return(nil)
			}
			fsm.On("TempDir", "copy-handler").Return(tempDirPath, nil)
			fsm.On("Mount", tc.Target, tempDirPath, tc.FSType, tc.MountOptions).Return(nil)
			fsm.On("Umount", tempDirPath).Return(nil)

			downloadDir := "/dummy-download-dir"

			cm := &copymock.CopierMock{}
			cm.On("CopyFile", memFs, lam, path.Join(downloadDir, tc.Sha256sum), path.Join(tempDirPath, tc.TargetPath), tc.ChunkSize, 0, 0, -1, true, tc.Compressed).Return(nil)

			cp := CopyObject{
				FileSystemHelper:  fsm,
				Copier:            cm,
				FileSystemBackend: memFs,
				LibArchiveBackend: lam,
				Target:            tc.Target,
				TargetType:        tc.TargetType,
				TargetPath:        tc.TargetPath,
				FSType:            tc.FSType,
				MountOptions:      tc.MountOptions,
				FormatOptions:     tc.FormatOptions,
				MustFormat:        tc.MustFormat,
				ChunkSize:         tc.ChunkSize,
			}
			cp.Sha256sum = tc.Sha256sum
			cp.Compressed = tc.Compressed

			err = cp.Install(downloadDir)

			assert.NoError(t, err)
			fsm.AssertExpectations(t)
			cm.AssertExpectations(t)
			lam.AssertExpectations(t)

			tempDirExists, err := afero.Exists(memFs, tempDirPath)
			assert.False(t, tempDirExists)
			assert.NoError(t, err)

			expectedTargetPath := path.Join(tempDirPath, tc.TargetPath)
			assert.Equal(t, expectedTargetPath, cp.GetTarget())
		})
	}
}

func TestCopyCleanupNil(t *testing.T) {
	cp := CopyObject{}
	assert.Nil(t, cp.Cleanup())
}
