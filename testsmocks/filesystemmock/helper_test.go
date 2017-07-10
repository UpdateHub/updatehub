/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package filesystemmock

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestFormat(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fshm := &FileSystemHelperMock{}
	fshm.On("Format", "/dev/xxa1", "ext3", "-q").Return(expectedError)

	err := fshm.Format("/dev/xxa1", "ext3", "-q")

	assert.Equal(t, expectedError, err)

	fshm.AssertExpectations(t)
}

func TestMount(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fshm := &FileSystemHelperMock{}
	fshm.On("Mount", "/dev/xxa1", "/mnt", "ext3", "-o rw").Return(expectedError)

	err := fshm.Mount("/dev/xxa1", "/mnt", "ext3", "-o rw")

	assert.Equal(t, expectedError, err)

	fshm.AssertExpectations(t)
}

func TestUmount(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fshm := &FileSystemHelperMock{}
	fshm.On("Umount", "/mnt").Return(expectedError)

	err := fshm.Umount("/mnt")

	assert.Equal(t, expectedError, err)

	fshm.AssertExpectations(t)
}

func TestTempDir(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fs := afero.NewMemMapFs()

	fshm := &FileSystemHelperMock{}
	fshm.On("TempDir", fs, "prefix").Return("/tmp/prefix-ABCD", expectedError)

	d, err := fshm.TempDir(fs, "prefix")

	assert.Equal(t, "/tmp/prefix-ABCD", d)
	assert.Equal(t, expectedError, err)

	fshm.AssertExpectations(t)
}
