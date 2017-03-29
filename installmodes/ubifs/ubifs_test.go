/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package ubifs

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/installmodes/internal/testsutils"
	"github.com/UpdateHub/updatehub/testsmocks/cmdlinemock"
	"github.com/UpdateHub/updatehub/testsmocks/copymock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/UpdateHub/updatehub/testsmocks/libarchivemock"
	"github.com/UpdateHub/updatehub/testsmocks/ubifsmock"
	"github.com/UpdateHub/updatehub/utils"

	"github.com/stretchr/testify/assert"
)

func TestUbifsInit(t *testing.T) {
	val, err := installmodes.GetObject("ubifs")
	assert.NoError(t, err)

	f1, ok := val.(*UbifsObject)
	if !ok {
		t.Error("Failed to cast return value of \"installmodes.GetObject()\" to UbifsObject")
	}

	f2, ok := getObject().(*UbifsObject)
	if !ok {
		t.Error("Failed to cast return value of \"getObject()\" to UbifsObject")
	}

	assert.Equal(t, f2, f1)
}

func TestUbifsGetObject(t *testing.T) {
	f, ok := getObject().(*UbifsObject)

	if !ok {
		t.Error("Failed to cast return value of \"getObject()\" to UbifsObject")
	}

	cmd := f.CmdLineExecuter
	_, ok = cmd.(*utils.CmdLine)

	if !ok {
		t.Error("Failed to cast default implementation of \"CmdLineExecuter\" to CmdLine")
	}
}

func TestUbifsCheckRequirementsWithBinariesNotFound(t *testing.T) {
	testCases := []struct {
		Name   string
		Binary string
	}{
		{
			"UbiUpdateVolNotFound",
			"ubiupdatevol",
		},
		{
			"UbInfoNotFound",
			"ubinfo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// setup a temp dir on PATH
			testPath := testsutils.SetupCheckRequirementsDir(t, []string{"ubiupdatevol", "ubinfo"})

			defer os.RemoveAll(testPath)
			err := os.Setenv("PATH", testPath)
			assert.NoError(t, err)

			// remove binary
			os.Remove(path.Join(testPath, tc.Binary))

			// test the call
			err = checkRequirements()

			assert.EqualError(t, err, fmt.Sprintf("exec: \"%s\": executable file not found in $PATH", tc.Binary))
		})
	}
}

func TestUbifsCheckRequirementsWithBinariesFound(t *testing.T) {
	// setup a temp dir on PATH
	testPath := testsutils.SetupCheckRequirementsDir(t, []string{"ubiupdatevol", "ubinfo"})
	defer os.RemoveAll(testPath)
	err := os.Setenv("PATH", testPath)
	assert.NoError(t, err)

	// test the call
	err = checkRequirements()

	assert.NoError(t, err)
}

func TestUbifsSetupWithUbivolumeTargetType(t *testing.T) {
	ufs := UbifsObject{}
	ufs.TargetType = "ubivolume"
	ufs.Target = "system0"
	err := ufs.Setup()
	assert.NoError(t, err)
}

func TestUbifsSetupWithNotSupportedTargetTypes(t *testing.T) {
	clm := &cmdlinemock.CmdLineExecuterMock{}

	ufs := UbifsObject{CmdLineExecuter: clm}

	ufs.TargetType = "unknown-type"
	err := ufs.Setup()
	assert.EqualError(t, err, "target-type 'unknown-type' is not supported for the 'ubifs' handler. Its value must be 'ubivolume'")

	ufs.TargetType = "mtdname"
	err = ufs.Setup()
	assert.EqualError(t, err, "target-type 'mtdname' is not supported for the 'ubifs' handler. Its value must be 'ubivolume'")

	clm.AssertExpectations(t)
}

func TestUbifsCleanupNil(t *testing.T) {
	ufs := UbifsObject{}
	assert.Nil(t, ufs.Cleanup())
}

func TestUbifsInstallWithSuccessNonCompressed(t *testing.T) {
	ubivolume := "system0"
	compressed := false
	targetDevice := "/dev/mtd3"
	sha256sum := "71c88745e5a72067f94aae0ecec6d45af8b0f6e1a37ef695df0b56711e192b86"

	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("ubiupdatevol %s %s", targetDevice, sha256sum)).Return([]byte("combinedoutput"), nil)

	fsm := &filesystemmock.FileSystemBackendMock{}

	uum := &ubifsmock.UbifsUtilsMock{}
	uum.On("GetTargetDeviceFromUbiVolumeName", fsm, ubivolume).Return(targetDevice, nil)

	ufs := UbifsObject{
		CmdLineExecuter:   clm,
		UbifsUtils:        uum,
		FileSystemBackend: fsm,
	}
	ufs.TargetType = "ubivolume"
	ufs.Target = ubivolume
	ufs.Sha256sum = sha256sum
	ufs.Compressed = compressed
	err := ufs.Install()
	assert.NoError(t, err)

	clm.AssertExpectations(t)
	uum.AssertExpectations(t)
	fsm.AssertExpectations(t)
}

func TestUbifsInstallWithSuccessCompressed(t *testing.T) {
	ubivolume := "system0"
	compressed := true
	targetDevice := "/dev/mtd3"
	sha256sum := "71c88745e5a72067f94aae0ecec6d45af8b0f6e1a37ef695df0b56711e192b86"
	srcPath := sha256sum
	uncompressedSize := 12345678.0
	cmdline := fmt.Sprintf("ubiupdatevol -s %.0f %s -", uncompressedSize, targetDevice)

	clm := &cmdlinemock.CmdLineExecuterMock{}

	fsm := &filesystemmock.FileSystemBackendMock{}

	uum := &ubifsmock.UbifsUtilsMock{}
	uum.On("GetTargetDeviceFromUbiVolumeName", fsm, ubivolume).Return(targetDevice, nil)

	lam := &libarchivemock.LibArchiveMock{}

	cpm := &copymock.CopierMock{}
	cpm.On("CopyToProcessStdin", fsm, lam, srcPath, cmdline, compressed).Return(nil)

	ufs := UbifsObject{
		CmdLineExecuter:   clm,
		UbifsUtils:        uum,
		LibArchiveBackend: lam,
		FileSystemBackend: fsm,
		Copier:            cpm,
	}
	ufs.TargetType = "ubivolume"
	ufs.Target = ubivolume
	ufs.Sha256sum = sha256sum
	ufs.Compressed = compressed
	ufs.UncompressedSize = uncompressedSize

	err := ufs.Install()
	assert.NoError(t, err)

	clm.AssertExpectations(t)
	uum.AssertExpectations(t)
	fsm.AssertExpectations(t)
	lam.AssertExpectations(t)
	cpm.AssertExpectations(t)
}

func TestUbifsInstallWithCopyToProcessStdinFailure(t *testing.T) {
	ubivolume := "system0"
	compressed := true
	targetDevice := "/dev/mtd3"
	sha256sum := "71c88745e5a72067f94aae0ecec6d45af8b0f6e1a37ef695df0b56711e192b86"
	srcPath := sha256sum
	uncompressedSize := 12345678.0
	cmdline := fmt.Sprintf("ubiupdatevol -s %.0f %s -", uncompressedSize, targetDevice)

	clm := &cmdlinemock.CmdLineExecuterMock{}

	fsm := &filesystemmock.FileSystemBackendMock{}

	uum := &ubifsmock.UbifsUtilsMock{}
	uum.On("GetTargetDeviceFromUbiVolumeName", fsm, ubivolume).Return(targetDevice, nil)

	lam := &libarchivemock.LibArchiveMock{}

	cpm := &copymock.CopierMock{}
	cpm.On("CopyToProcessStdin", fsm, lam, srcPath, cmdline, compressed).Return(fmt.Errorf("process error"))

	ufs := UbifsObject{
		CmdLineExecuter:   clm,
		UbifsUtils:        uum,
		LibArchiveBackend: lam,
		FileSystemBackend: fsm,
		Copier:            cpm,
	}
	ufs.TargetType = "ubivolume"
	ufs.Target = ubivolume
	ufs.Sha256sum = sha256sum
	ufs.Compressed = compressed
	ufs.UncompressedSize = uncompressedSize

	err := ufs.Install()
	assert.EqualError(t, err, "process error")

	clm.AssertExpectations(t)
	uum.AssertExpectations(t)
	fsm.AssertExpectations(t)
	lam.AssertExpectations(t)
	cpm.AssertExpectations(t)
}

func TestUbifsInstallWithGetTargetDeviceFromUbiVolumeNameFailure(t *testing.T) {
	ubivolume := "system0"
	compressed := false
	sha256sum := "71c88745e5a72067f94aae0ecec6d45af8b0f6e1a37ef695df0b56711e192b86"

	clm := &cmdlinemock.CmdLineExecuterMock{}

	fsm := &filesystemmock.FileSystemBackendMock{}

	uum := &ubifsmock.UbifsUtilsMock{}
	uum.On("GetTargetDeviceFromUbiVolumeName", fsm, ubivolume).Return("", fmt.Errorf("UBI volume '%s' wasn't found", ubivolume))

	ufs := UbifsObject{
		CmdLineExecuter:   clm,
		UbifsUtils:        uum,
		FileSystemBackend: fsm,
	}
	ufs.TargetType = "ubivolume"
	ufs.Target = ubivolume
	ufs.Sha256sum = sha256sum
	ufs.Compressed = compressed
	err := ufs.Install()
	assert.EqualError(t, err, fmt.Sprintf("UBI volume '%s' wasn't found", ubivolume))

	clm.AssertExpectations(t)
	uum.AssertExpectations(t)
	fsm.AssertExpectations(t)
}

func TestUbifsInstallWithUbiUpdateVolFailure(t *testing.T) {
	ubivolume := "system0"
	compressed := false
	targetDevice := "/dev/mtd3"
	sha256sum := "71c88745e5a72067f94aae0ecec6d45af8b0f6e1a37ef695df0b56711e192b86"

	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("ubiupdatevol %s %s", targetDevice, sha256sum)).Return([]byte("error"), fmt.Errorf("Error executing command"))

	fsm := &filesystemmock.FileSystemBackendMock{}

	uum := &ubifsmock.UbifsUtilsMock{}
	uum.On("GetTargetDeviceFromUbiVolumeName", fsm, ubivolume).Return(targetDevice, nil)

	ufs := UbifsObject{
		CmdLineExecuter:   clm,
		UbifsUtils:        uum,
		FileSystemBackend: fsm,
	}
	ufs.TargetType = "ubivolume"
	ufs.Target = ubivolume
	ufs.Sha256sum = sha256sum
	ufs.Compressed = compressed
	err := ufs.Install()
	assert.EqualError(t, err, "Error executing command")

	clm.AssertExpectations(t)
	uum.AssertExpectations(t)
	fsm.AssertExpectations(t)
}
