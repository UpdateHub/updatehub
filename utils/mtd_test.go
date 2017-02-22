package utils

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

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
