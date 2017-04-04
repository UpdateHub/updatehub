/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package raw

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

func TestRawInit(t *testing.T) {
	val, err := installmodes.GetObject("raw")
	assert.NoError(t, err)

	r1, ok := val.(*RawObject)
	if !ok {
		t.Error("Failed to cast return value of \"installmodes.GetObject()\" to RawObject")
	}

	osFs := afero.NewOsFs()
	r2 := &RawObject{
		LibArchiveBackend: &libarchive.LibArchive{},
		FileSystemBackend: osFs,
		Copier:            &utils.ExtendedIO{},
		ChunkSize:         128 * 1024,
		Skip:              0,
		Seek:              0,
		Count:             -1,
		Truncate:          true,
	}

	assert.Equal(t, r2, r1)
}

func TestRawSetupWithSuccess(t *testing.T) {
	r := RawObject{}
	r.TargetType = "device"
	err := r.Setup()
	assert.NoError(t, err)
}

func TestRawSetupWithNotSupportedTargetTypes(t *testing.T) {
	r := RawObject{}

	r.TargetType = "ubivolume"
	err := r.Setup()
	assert.EqualError(t, err, "target-type 'ubivolume' is not supported for the 'raw' handler. Its value must be 'device'")

	r.TargetType = "mtdname"
	err = r.Setup()
	assert.EqualError(t, err, "target-type 'mtdname' is not supported for the 'raw' handler. Its value must be 'device'")

	r.TargetType = "someother"
	err = r.Setup()
	assert.EqualError(t, err, "target-type 'someother' is not supported for the 'raw' handler. Its value must be 'device'")
}

func TestRawInstallWithCopyFileError(t *testing.T) {
	fsbm := &filesystemmock.FileSystemBackendMock{}

	lam := &libarchivemock.LibArchiveMock{}

	targetDevice := "/dev/xx1"
	sha256sum := "5bdbf286cb4adcff26befa2183f3167c053bc565036736eaa2ae429fe910d93c"
	compressed := false
	downloadDir := "/dummy-download-dir"
	sourcePath := path.Join(downloadDir, sha256sum)

	cm := &copymock.CopierMock{}
	cm.On("CopyFile", fsbm, lam, sourcePath, targetDevice, 128*1024, 0, 0, -1, true, compressed).Return(fmt.Errorf("copy file error"))

	r := RawObject{
		Copier:            cm,
		FileSystemBackend: fsbm,
		LibArchiveBackend: lam,
		ChunkSize:         128 * 1024,
		Count:             -1,
		Truncate:          true,
	}
	r.Target = targetDevice
	r.Sha256sum = sha256sum
	r.Compressed = compressed

	err := r.Install(downloadDir)

	assert.EqualError(t, err, "copy file error")
	cm.AssertExpectations(t)
	lam.AssertExpectations(t)
	fsbm.AssertExpectations(t)

	assert.Equal(t, targetDevice, r.GetTarget())
}

func TestRawInstallWithSuccess(t *testing.T) {
	testCases := []struct {
		Name              string
		Sha256sum         string
		Target            string
		TargetType        string
		ChunkSize         int
		Skip              int
		Seek              int
		Count             int
		Truncate          bool
		ExpectedChunkSize int
		Compressed        bool
	}{
		{
			"WithAllFields",
			"5bdbf286cb4adcff26befa2183f3167c053bc565036736eaa2ae429fe910d93c",
			"/dev/xx1",
			"device",
			2048,
			2,
			3,
			-1,
			true,
			2048,
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			fsbm := &filesystemmock.FileSystemBackendMock{}

			lam := &libarchivemock.LibArchiveMock{}

			downloadDir := "/dummy-download-dir"
			sourcePath := path.Join(downloadDir, tc.Sha256sum)

			cm := &copymock.CopierMock{}
			cm.On("CopyFile", fsbm, lam, sourcePath, tc.Target, tc.ExpectedChunkSize, tc.Skip, tc.Seek, tc.Count, tc.Truncate, tc.Compressed).Return(nil)

			r := RawObject{Copier: cm, FileSystemBackend: fsbm, LibArchiveBackend: lam}
			r.Target = tc.Target
			r.TargetType = tc.TargetType
			r.Sha256sum = tc.Sha256sum
			r.ChunkSize = tc.ChunkSize
			r.Skip = tc.Skip
			r.Seek = tc.Seek
			r.Count = tc.Count
			r.Truncate = tc.Truncate
			r.Compressed = tc.Compressed

			err := r.Install(downloadDir)

			assert.NoError(t, err)
			cm.AssertExpectations(t)
			lam.AssertExpectations(t)
			fsbm.AssertExpectations(t)

			assert.Equal(t, tc.Target, r.GetTarget())
		})
	}
}

func TestRawCleanupNil(t *testing.T) {
	r := RawObject{}
	assert.Nil(t, r.Cleanup())
}
