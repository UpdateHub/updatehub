/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package mtd

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/UpdateHub/updatehub/testsmocks/cmdlinemock"
)

const ubinfoStdoutTemplate string = `Volume ID:   %d (on ubi%d)
Type:        dynamic
Alignment:   1
Size:        407 LEBs (52512768 bytes, 50.1 MiB)
State:       OK
Name:        %s
Character device major/minor: 247:1`

func TestUbifsUtilsImplWithASingleDeviceNode(t *testing.T) {
	ubivolume := "system0"
	deviceNumber := 1
	volumeID := 2

	memFs := afero.NewMemMapFs()
	memFs.MkdirAll("/dev", 0755)
	afero.WriteFile(memFs, fmt.Sprintf("/dev/ubi%d", deviceNumber), []byte("ubi_content"), 0755)

	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("ubinfo -d %d -N %s", deviceNumber, ubivolume)).Return([]byte(fmt.Sprintf(ubinfoStdoutTemplate, volumeID, deviceNumber, ubivolume)), nil)

	uui := &UbifsUtilsImpl{CmdLineExecuter: clm}
	targetDevice, err := uui.GetTargetDeviceFromUbiVolumeName(memFs, ubivolume)

	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("/dev/ubi%d_%d", deviceNumber, volumeID), targetDevice)

	clm.AssertExpectations(t)
}

func TestUbifsUtilsImplWithMultipleUbiDeviceNodes(t *testing.T) {
	ubivolume := "system0"
	deviceNumber := 1
	volumeID := 2

	memFs := afero.NewMemMapFs()
	memFs.MkdirAll("/dev", 0755)
	afero.WriteFile(memFs, fmt.Sprintf("/dev/ubi%d", deviceNumber-1), []byte("ubi_content"), 0755)
	afero.WriteFile(memFs, fmt.Sprintf("/dev/ubi%d", deviceNumber), []byte("ubi_content"), 0755)
	afero.WriteFile(memFs, fmt.Sprintf("/dev/ubi%d", deviceNumber+1), []byte("ubi_content"), 0755)

	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", fmt.Sprintf("ubinfo -d %d -N %s", deviceNumber-1, ubivolume)).Return([]byte(""), fmt.Errorf("Error executing command"))
	clm.On("Execute", fmt.Sprintf("ubinfo -d %d -N %s", deviceNumber, ubivolume)).Return([]byte(fmt.Sprintf(ubinfoStdoutTemplate, volumeID, deviceNumber, ubivolume)), nil)

	uui := &UbifsUtilsImpl{CmdLineExecuter: clm}
	targetDevice, err := uui.GetTargetDeviceFromUbiVolumeName(memFs, ubivolume)

	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("/dev/ubi%d_%d", deviceNumber, volumeID), targetDevice)

	clm.AssertExpectations(t)
}

func TestUbifsUtilsImplWithReadDirFailure(t *testing.T) {
	ubivolume := "system0"

	memFs := afero.NewMemMapFs()
	memFs.RemoveAll("/dev")

	clm := &cmdlinemock.CmdLineExecuterMock{}

	uui := &UbifsUtilsImpl{CmdLineExecuter: clm}
	targetDevice, err := uui.GetTargetDeviceFromUbiVolumeName(memFs, ubivolume)

	assert.EqualError(t, err, "open /dev: file does not exist")
	assert.Equal(t, "", targetDevice)

	clm.AssertExpectations(t)
}
