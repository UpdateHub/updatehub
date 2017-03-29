/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package flash

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/installmodes/internal/testsutils"
	"github.com/UpdateHub/updatehub/testsmocks/cmdlinemock"
	"github.com/UpdateHub/updatehub/testsmocks/mtdmock"
	"github.com/UpdateHub/updatehub/utils"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestFlashInit(t *testing.T) {
	val, err := installmodes.GetObject("flash")
	assert.NoError(t, err)

	f1, ok := val.(*FlashObject)
	if !ok {
		t.Error("Failed to cast return value of \"installmodes.GetObject()\" to FlashObject")
	}

	f2, ok := getObject().(*FlashObject)
	if !ok {
		t.Error("Failed to cast return value of \"getObject()\" to FlashObject")
	}

	assert.Equal(t, f2, f1)
}

func TestFlashGetObject(t *testing.T) {
	f, ok := getObject().(*FlashObject)

	if !ok {
		t.Error("Failed to cast return value of \"getObject()\" to FlashObject")
	}

	cmd := f.CmdLineExecuter
	_, ok = cmd.(*utils.CmdLine)

	if !ok {
		t.Error("Failed to cast default implementation of \"CmdLineExecuter\" to CmdLine")
	}
}

func TestFlashCheckRequirementsWithBinariesNotFound(t *testing.T) {
	testCases := []struct {
		Name   string
		Binary string
	}{
		{
			"NandwriteNotFound",
			"nandwrite",
		},
		{
			"FlashcpNotFound",
			"flashcp",
		},
		{
			"FlashEraseNotFound",
			"flash_erase",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// setup a temp dir on PATH
			testPath := testsutils.SetupCheckRequirementsDir(t, []string{"nandwrite", "flashcp", "flash_erase"})

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

func TestFlashCheckRequirementsWithBinariesFound(t *testing.T) {
	// setup a temp dir on PATH
	testPath := testsutils.SetupCheckRequirementsDir(t, []string{"nandwrite", "flashcp", "flash_erase"})
	defer os.RemoveAll(testPath)
	err := os.Setenv("PATH", testPath)
	assert.NoError(t, err)

	// test the call
	err = checkRequirements()

	assert.NoError(t, err)
}

func TestFlashSetupWithDeviceTargetType(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtddevice := "/dev/mtd8"

	mum := &mtdmock.MtdUtilsMock{}

	f := FlashObject{FileSystemBackend: memFs, MtdUtils: mum}
	f.TargetType = "device"
	f.Target = mtddevice
	err := f.Setup()
	assert.NoError(t, err)
	assert.Equal(t, mtddevice, f.targetDevice)

	mum.AssertExpectations(t)
}

func TestFlashSetupWithMtdnameTargetType(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtdname := "system0"
	mtddevice := "/dev/mtd5"

	mum := &mtdmock.MtdUtilsMock{}
	mum.On("GetTargetDeviceFromMtdName", memFs, mtdname).Return(mtddevice, nil)

	f := FlashObject{FileSystemBackend: memFs, MtdUtils: mum}
	f.TargetType = "mtdname"
	f.Target = mtdname
	err := f.Setup()
	assert.NoError(t, err)
	assert.Equal(t, mtddevice, f.targetDevice)

	mum.AssertExpectations(t)
}

func TestFlashSetupWithMtdnameTargetTypeWithTargetDeviceError(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtdname := "system0"

	mum := &mtdmock.MtdUtilsMock{}
	mum.On("GetTargetDeviceFromMtdName", memFs, mtdname).Return("", fmt.Errorf("Couldn't find a flash device corresponding to the mtdname '%s'", mtdname))

	f := FlashObject{FileSystemBackend: memFs, MtdUtils: mum}
	f.TargetType = "mtdname"
	f.Target = mtdname
	err := f.Setup()
	assert.EqualError(t, err, fmt.Sprintf("Couldn't find a flash device corresponding to the mtdname '%s'", mtdname))
	assert.Equal(t, "", f.targetDevice)

	mum.AssertExpectations(t)
}

func TestFlashSetupWithNotSupportedTargetTypes(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mum := &mtdmock.MtdUtilsMock{}

	f := FlashObject{FileSystemBackend: memFs, MtdUtils: mum}

	f.TargetType = "unknown-type"
	err := f.Setup()
	assert.EqualError(t, err, "target-type 'unknown-type' is not supported for the 'flash' handler. Its value must be either 'device' or 'mtdname'")
	assert.Equal(t, "", f.targetDevice)

	f.TargetType = "ubivolume"
	err = f.Setup()
	assert.EqualError(t, err, "target-type 'ubivolume' is not supported for the 'flash' handler. Its value must be either 'device' or 'mtdname'")
	assert.Equal(t, "", f.targetDevice)

	mum.AssertExpectations(t)
}

func TestFlashCleanupNil(t *testing.T) {
	f := FlashObject{}
	assert.Nil(t, f.Cleanup())
}

func TestFlashInstallSuccessWithNAND(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtddevice := "/dev/mtd9"
	sha256sum := "8e29c9df2bc3c417b460b02b566edc668195da9c75a1fcf2f63829a7c59fc07d"

	mum := &mtdmock.MtdUtilsMock{}
	mum.On("MtdIsNAND", mtddevice).Return(true, nil)

	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("flash_erase %s 0 0", mtddevice)).Return([]byte("combinedOutput"), nil)
	clm.On("Execute", fmt.Sprintf("nandwrite -p %s %s", mtddevice, sha256sum)).Return([]byte("combinedOutput"), nil)

	f := FlashObject{FileSystemBackend: memFs, MtdUtils: mum, CmdLineExecuter: clm}
	f.targetDevice = mtddevice
	f.Sha256sum = sha256sum
	err := f.Install()
	assert.NoError(t, err)

	mum.AssertExpectations(t)
	clm.AssertExpectations(t)
}

func TestFlashInstallSuccessWithNOR(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtddevice := "/dev/mtd9"
	sha256sum := "8e29c9df2bc3c417b460b02b566edc668195da9c75a1fcf2f63829a7c59fc07d"

	mum := &mtdmock.MtdUtilsMock{}
	mum.On("MtdIsNAND", mtddevice).Return(false, nil)

	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("flash_erase %s 0 0", mtddevice)).Return([]byte("combinedOutput"), nil)
	clm.On("Execute", fmt.Sprintf("flashcp %s %s", sha256sum, mtddevice)).Return([]byte("combinedOutput"), nil)

	f := FlashObject{FileSystemBackend: memFs, MtdUtils: mum, CmdLineExecuter: clm}
	f.targetDevice = mtddevice
	f.Sha256sum = sha256sum
	err := f.Install()
	assert.NoError(t, err)

	mum.AssertExpectations(t)
	clm.AssertExpectations(t)
}

func TestFlashInstallWithMtdIsNANDFailure(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtddevice := "/dev/mtd9"
	sha256sum := "8e29c9df2bc3c417b460b02b566edc668195da9c75a1fcf2f63829a7c59fc07d"

	mum := &mtdmock.MtdUtilsMock{}
	mum.On("MtdIsNAND", mtddevice).Return(false, fmt.Errorf("Error opening %s: no such device", mtddevice))

	clm := &cmdlinemock.CmdLineExecuterMock{}

	f := FlashObject{FileSystemBackend: memFs, MtdUtils: mum, CmdLineExecuter: clm}
	f.targetDevice = mtddevice
	f.Sha256sum = sha256sum
	err := f.Install()
	assert.EqualError(t, err, fmt.Sprintf("Error opening %s: no such device", mtddevice))

	mum.AssertExpectations(t)
	clm.AssertExpectations(t)
}

func TestFlashInstallWithFlashEraseFailure(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtddevice := "/dev/mtd9"
	sha256sum := "8e29c9df2bc3c417b460b02b566edc668195da9c75a1fcf2f63829a7c59fc07d"

	mum := &mtdmock.MtdUtilsMock{}
	mum.On("MtdIsNAND", mtddevice).Return(false, nil)

	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("flash_erase %s 0 0", mtddevice)).Return([]byte("error"), fmt.Errorf("flash_erase error"))

	f := FlashObject{FileSystemBackend: memFs, MtdUtils: mum, CmdLineExecuter: clm}
	f.targetDevice = mtddevice
	f.Sha256sum = sha256sum
	err := f.Install()
	assert.EqualError(t, err, "flash_erase error")

	mum.AssertExpectations(t)
	clm.AssertExpectations(t)
}

func TestFlashInstallWithFlashcpFailure(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtddevice := "/dev/mtd9"
	sha256sum := "8e29c9df2bc3c417b460b02b566edc668195da9c75a1fcf2f63829a7c59fc07d"

	mum := &mtdmock.MtdUtilsMock{}
	mum.On("MtdIsNAND", mtddevice).Return(false, nil)

	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("flash_erase %s 0 0", mtddevice)).Return([]byte("combinedOutput"), nil)
	clm.On("Execute", fmt.Sprintf("flashcp %s %s", sha256sum, mtddevice)).Return([]byte("error"), fmt.Errorf("flashcp error"))

	f := FlashObject{FileSystemBackend: memFs, MtdUtils: mum, CmdLineExecuter: clm}
	f.targetDevice = mtddevice
	f.Sha256sum = sha256sum
	err := f.Install()
	assert.EqualError(t, err, "flashcp error")

	mum.AssertExpectations(t)
	clm.AssertExpectations(t)
}

func TestFlashInstallWithNandwriteFailure(t *testing.T) {
	memFs := afero.NewMemMapFs()

	mtddevice := "/dev/mtd9"
	sha256sum := "8e29c9df2bc3c417b460b02b566edc668195da9c75a1fcf2f63829a7c59fc07d"

	mum := &mtdmock.MtdUtilsMock{}
	mum.On("MtdIsNAND", mtddevice).Return(true, nil)

	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("flash_erase %s 0 0", mtddevice)).Return([]byte("combinedOutput"), nil)
	clm.On("Execute", fmt.Sprintf("nandwrite -p %s %s", mtddevice, sha256sum)).Return([]byte("error"), fmt.Errorf("nandwrite error"))

	f := FlashObject{FileSystemBackend: memFs, MtdUtils: mum, CmdLineExecuter: clm}
	f.targetDevice = mtddevice
	f.Sha256sum = sha256sum
	err := f.Install()
	assert.EqualError(t, err, "nandwrite error")

	mum.AssertExpectations(t)
	clm.AssertExpectations(t)
}
