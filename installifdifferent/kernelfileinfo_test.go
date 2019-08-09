/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package installifdifferent

import (
	"fmt"
	"os"
	"testing"

	"github.com/UpdateHub/updatehub/testsmocks/filemock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewKernelFileInfo(t *testing.T) {
	testCases := []struct {
		Name     string
		Filename string
		Arch     LinuxArch
		Type     KernelType
		Version  string
	}{
		{
			"ARMuImage",
			"testdata/kernel_file_info/arm-uImage",
			ARMLinuxArch,
			uImageKernelType,
			"4.1.15-1.2.0+g274a055",
		},
		{
			"ARMzImage",
			"testdata/kernel_file_info/arm-zImage",
			ARMLinuxArch,
			zImageKernelType,
			"4.4.1",
		},
		{
			"x86bzImage",
			"testdata/kernel_file_info/x86-bzImage",
			x86LinuxArch,
			bzImageKernelType,
			"4.1.30-1-MANJARO",
		},
		{
			"x86zImage",
			"testdata/kernel_file_info/x86-zImage",
			x86LinuxArch,
			zImageKernelType,
			"4.1.30-1-MANJARO",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			osFs := afero.NewOsFs()

			f, err := osFs.OpenFile(tc.Filename, os.O_RDONLY, 0)
			assert.NoError(t, err)

			kfi := NewKernelFileInfo(osFs, f)
			assert.NotNil(t, kfi)

			assert.Equal(t, tc.Arch, kfi.Arch)
			assert.Equal(t, tc.Type, kfi.Type)
			assert.Equal(t, tc.Version, kfi.Version)
		})
	}
}

func TestCaptureTextFromBinaryFileWithReadError(t *testing.T) {
	fsm := &filesystemmock.FileSystemBackendMock{}
	fm := &filemock.FileMock{}

	fsm.On("OpenFile", "/some-file", os.O_RDONLY, os.FileMode(0)).Return(fm, nil)

	fm.On("Read", mock.AnythingOfType("[]uint8")).Return(0, fmt.Errorf("read error"))

	f, err := fsm.OpenFile("/some-file", os.O_RDONLY, 0)
	assert.NoError(t, err)

	capturedText := CaptureTextFromBinaryFile(f, ".*")

	assert.Equal(t, "", capturedText)

	fsm.AssertExpectations(t)
	fm.AssertExpectations(t)
}
