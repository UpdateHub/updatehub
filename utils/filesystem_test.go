/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/UpdateHub/updatehub/testsmocks/cmdlinemock"
)

func TestFormat(t *testing.T) {
	testCases := []struct {
		devicePath      string
		fsType          string
		formatOptions   string
		expectedCmdline string
	}{
		{
			"/dev/xxc3",
			"btrfs",
			"-L label",
			"mkfs.btrfs -L label /dev/xxc3",
		},
		{
			"/dev/xxc3",
			"ext2",
			"-L label",
			"mkfs.ext2 -F -L label /dev/xxc3",
		},
		{
			"/dev/xxc3",
			"ext3",
			"-L label",
			"mkfs.ext3 -F -L label /dev/xxc3",
		},
		{
			"/dev/xxc3",
			"ext4",
			"-L label",
			"mkfs.ext4 -F -L label /dev/xxc3",
		},
		{
			"/dev/xxc3",
			"vfat",
			"-L label",
			"mkfs.vfat -L label /dev/xxc3",
		},
		{
			"/dev/xxc3",
			"f2fs",
			"-L label",
			"mkfs.f2fs -L label /dev/xxc3",
		},
		{
			"/dev/xxc3",
			"ubifs",
			"-L label",
			"mkfs.ubifs -y -L label /dev/xxc3",
		},
		{
			"/dev/xxc3",
			"xfs",
			"-L label",
			"mkfs.xfs -f -L label /dev/xxc3",
		},
		{
			"/dev/xxc3",
			"jffs2",
			"-q",
			"flash_erase -j -q /dev/xxc3 0 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.fsType, func(t *testing.T) {
			fsBackend := afero.NewOsFs()

			testPath, err := afero.TempDir(fsBackend, "", "filesystem-test")
			assert.NoError(t, err)
			defer os.RemoveAll(testPath)
			err = os.Setenv("PATH", testPath)
			assert.NoError(t, err)

			binaryName := strings.Split(tc.expectedCmdline, " ")[0]
			binaryPath := path.Join(testPath, binaryName)

			err = afero.WriteFile(fsBackend, binaryPath, []byte("binary_content"), 0755)

			clem := &cmdlinemock.CmdLineExecuterMock{}
			clem.On("Execute", tc.expectedCmdline).Return([]byte("useless output"), nil)

			fs := &FileSystem{CmdLineExecuter: clem}

			err = fs.Format(tc.devicePath, tc.fsType, tc.formatOptions)
			assert.NoError(t, err)

			clem.AssertExpectations(t)
		})
	}
}

func TestFormatWithBinaryNotFound(t *testing.T) {
	clem := &cmdlinemock.CmdLineExecuterMock{}

	fs := &FileSystem{CmdLineExecuter: clem}

	err := fs.Format("/dev/xxc3", "ubifs", "")
	assert.EqualError(t, err, "exec: \"mkfs.ubifs\": executable file not found in $PATH")

	clem.AssertExpectations(t)
}

func TestFormatWithFsTypeNotSupported(t *testing.T) {
	clem := &cmdlinemock.CmdLineExecuterMock{}

	fs := &FileSystem{CmdLineExecuter: clem}

	err := fs.Format("/dev/xxc3", "unknownfs", "")
	assert.EqualError(t, err, "Couldn't format '/dev/xxc3': fs type 'unknownfs' is not supported")

	clem.AssertExpectations(t)
}

func TestFormatWithErrorOnExecuteCommand(t *testing.T) {
	devicePath := "/dev/xxc3"
	fsType := "ext4"
	formatOptions := "-L label"
	expectedCmdline := "mkfs.ext4 -F -L label /dev/xxc3"

	fsBackend := afero.NewOsFs()

	testPath, err := afero.TempDir(fsBackend, "", "filesystem-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)
	err = os.Setenv("PATH", testPath)
	assert.NoError(t, err)

	binaryName := strings.Split(expectedCmdline, " ")[0]
	binaryPath := path.Join(testPath, binaryName)

	err = afero.WriteFile(fsBackend, binaryPath, []byte("binary_content"), 0755)

	clem := &cmdlinemock.CmdLineExecuterMock{}
	clem.On("Execute", expectedCmdline).Return([]byte("useless output"), fmt.Errorf("cmdline error"))

	fs := &FileSystem{CmdLineExecuter: clem}

	err = fs.Format(devicePath, fsType, formatOptions)
	assert.EqualError(t, err, "couldn't format '/dev/xxc3'. cmdline error: cmdline error")

	clem.AssertExpectations(t)
}
