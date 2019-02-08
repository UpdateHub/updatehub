/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package zephyr

import (
	"os"
	"testing"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/testsmocks/filemock"
	"github.com/UpdateHub/updatehub/testsmocks/filesystemmock"
	"github.com/stretchr/testify/assert"
)

func TestZephyrInit(t *testing.T) {
	val, err := installmodes.GetObject("zephyr")
	assert.NoError(t, err)

	r1, ok := val.(*ZephyrObject)
	if !ok {
		t.Error("Failed to cast return value of \"installmodes.GetObject()\" to ZephyrObject")
	}

	r2 := &ZephyrObject{}

	assert.Equal(t, r2, r1)
}

func TestZephyrSetupWithSuccess(t *testing.T) {
	r := ZephyrObject{}
	assert.Panics(t, func() { r.Setup() })
}

func TestZephyrInstallWithSuccess(t *testing.T) {
	r := ZephyrObject{}

	assert.Panics(t, func() { r.Install("") })
}

func TestZephyrSetupTarget(t *testing.T) {
	r := ZephyrObject{}

	targetMock := &filemock.FileMock{}
	fsbm := &filesystemmock.FileSystemBackendMock{}
	fsbm.On("OpenFile", "/target", os.O_RDONLY, os.FileMode(0)).Return(targetMock, nil)

	target, err := fsbm.OpenFile("/target", os.O_RDONLY, 0)
	assert.NoError(t, err)

	assert.Panics(t, func() { r.SetupTarget(target) })
}

func TestZephyrCleanupNil(t *testing.T) {
	r := ZephyrObject{}
	assert.Panics(t, func() { r.Cleanup() })
}
