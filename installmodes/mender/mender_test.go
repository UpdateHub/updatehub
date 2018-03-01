/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package mender

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/updatehub/updatehub/installmodes"
	"github.com/updatehub/updatehub/testsmocks/filemock"
	"github.com/updatehub/updatehub/testsmocks/filesystemmock"
)

func TestMenderInit(t *testing.T) {
	val, err := installmodes.GetObject("mender")
	assert.NoError(t, err)

	r1, ok := val.(*MenderObject)
	if !ok {
		t.Error("Failed to cast return value of \"installmodes.GetObject()\" to MenderObject")
	}

	r2 := &MenderObject{}

	assert.Equal(t, r2, r1)
}

func TestMenderSetupWithSuccess(t *testing.T) {
	r := MenderObject{}
	err := r.Setup()
	assert.NoError(t, err)
}

func TestMenderInstallWithSuccess(t *testing.T) {
	r := MenderObject{}

	err := r.Install("")

	assert.NoError(t, err)
}

func TestMenderSetupTarget(t *testing.T) {
	r := MenderObject{}

	targetMock := &filemock.FileMock{}
	fsbm := &filesystemmock.FileSystemBackendMock{}
	fsbm.On("OpenFile", "/target", os.O_RDONLY, os.FileMode(0)).Return(targetMock, nil)

	target, err := fsbm.OpenFile("/target", os.O_RDONLY, 0)
	assert.NoError(t, err)

	r.SetupTarget(target)
}

func TestMenderCleanupNil(t *testing.T) {
	r := MenderObject{}
	assert.Nil(t, r.Cleanup())
}
