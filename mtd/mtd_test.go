/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package mtd

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestMtdIsNANDWithOpenError(t *testing.T) {
	mui := MtdUtilsImpl{}

	isNand, err := mui.MtdIsNAND("/tmp/non-existant")

	assert.EqualError(t, err, "Couldn't open flash device '/tmp/non-existant': No such file or directory")
	assert.False(t, isNand)
}

func TestMtdIsNANDWithIoctlError(t *testing.T) {
	mui := MtdUtilsImpl{}

	tempFile, err := ioutil.TempFile("", "copy-test")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.Write([]byte("dummy"))
	assert.NoError(t, err)

	tempFile.Close()

	isNand, err := mui.MtdIsNAND(tempFile.Name())

	assert.EqualError(t, err, fmt.Sprintf("Error executing MEMGETINFO ioctl on '%s': Inappropriate ioctl for device", tempFile.Name()))
	assert.False(t, isNand)
}

func TestGetTargetDeviceFromMtdName(t *testing.T) {
	mtdname := "system0"
	expectedTargetDevice := "/dev/mtd2"

	procMtdContent := []byte(
		`dev:	 size	erasesize  name
mtd0: 10000000 00020000 "system2"
mtd1: 10000000 00020000 "system1"
mtd2: 10000000 00020000 "system0"
`)

	memFs := afero.NewMemMapFs()
	afero.WriteFile(memFs, "/proc/mtd", procMtdContent, 444)

	mui := MtdUtilsImpl{}
	targetDevice, err := mui.GetTargetDeviceFromMtdName(memFs, mtdname)

	assert.NoError(t, err)
	assert.Equal(t, expectedTargetDevice, targetDevice)
}

func TestGetTargetDeviceFromMtdNameWithNoDevicePathFound(t *testing.T) {
	mtdname := "system5" // must not be in the "procMtdContent"
	expectedTargetDevice := ""

	procMtdContent := []byte(
		`dev:	 size	erasesize  name
mtd0: 10000000 00020000 "system2"
mtd1: 10000000 00020000 "system1"
mtd2: 10000000 00020000 "system0"
`)

	memFs := afero.NewMemMapFs()
	afero.WriteFile(memFs, "/proc/mtd", procMtdContent, 444)

	mui := MtdUtilsImpl{}
	targetDevice, err := mui.GetTargetDeviceFromMtdName(memFs, mtdname)

	assert.EqualError(t, err, "Couldn't find a flash device corresponding to the mtdname 'system5'")
	assert.Equal(t, expectedTargetDevice, targetDevice)
}

func TestGetTargetDeviceFromMtdNameWithErrorOpeningProcMtd(t *testing.T) {
	mtdname := "system0"
	expectedTargetDevice := ""

	memFs := afero.NewMemMapFs()

	mui := MtdUtilsImpl{}
	targetDevice, err := mui.GetTargetDeviceFromMtdName(memFs, mtdname)

	assert.EqualError(t, err, "open /proc/mtd: file does not exist")
	assert.Equal(t, expectedTargetDevice, targetDevice)
}

func TestGetTargetDeviceFromMtdNameWithProcMtdInvalidFormat(t *testing.T) {
	mtdname := "system0"
	expectedTargetDevice := ""

	procMtdContent := []byte(
		`dev:	 size	erasesize  name
mtd0: 10000000 "system0"
mtd1: 10000000 00020000 00020000 "system0"
mt2: 10000000 00020000 "system0"
mtdd3: 10000000 00020000 "system0"
mtdZ: 10000000 00020000 "system0"
`)

	memFs := afero.NewMemMapFs()
	afero.WriteFile(memFs, "/proc/mtd", procMtdContent, 444)

	mui := MtdUtilsImpl{}
	targetDevice, err := mui.GetTargetDeviceFromMtdName(memFs, mtdname)
	assert.EqualError(t, err, "Couldn't find a flash device corresponding to the mtdname 'system0'")
	assert.Equal(t, expectedTargetDevice, targetDevice)
}
