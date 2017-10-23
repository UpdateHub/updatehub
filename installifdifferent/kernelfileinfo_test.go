/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package installifdifferent

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/updatehub/updatehub/testsmocks/filemock"
	"github.com/updatehub/updatehub/testsmocks/filesystemmock"
)

func TestNewKernelFileInfoWithInvalidFilename(t *testing.T) {
	memFs := afero.NewMemMapFs()

	filepath := "/tmp/inexistant"
	kfi, err := NewKernelFileInfo(memFs, filepath)
	assert.EqualError(t, err, "open /tmp/inexistant: file does not exist")
	assert.Nil(t, kfi)
}

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

			kfi, err := NewKernelFileInfo(osFs, tc.Filename)
			assert.NoError(t, err)
			assert.NotNil(t, kfi)

			assert.Equal(t, tc.Arch, kfi.Arch)
			assert.Equal(t, tc.Type, kfi.Type)
			assert.Equal(t, tc.Version, kfi.Version)
		})
	}
}

func TestCaptureTextFromBinaryFileWithInvalidFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	capturedText := CaptureTextFromBinaryFile(fs, "/inexistant-file", ".*")

	assert.Equal(t, "", capturedText)
}

func TestCaptureTextFromBinaryFileWithReadError(t *testing.T) {
	fsm := &filesystemmock.FileSystemBackendMock{}
	fm := &filemock.FileMock{}

	fsm.On("Open", "/some-file").Return(fm, nil)

	fm.On("Read", mock.AnythingOfType("[]uint8")).Return(0, fmt.Errorf("read error"))
	fm.On("Close").Return(nil)

	capturedText := CaptureTextFromBinaryFile(fsm, "/some-file", ".*")

	assert.Equal(t, "", capturedText)

	fsm.AssertExpectations(t)
	fm.AssertExpectations(t)
}
