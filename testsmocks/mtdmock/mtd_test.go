/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package mtdmock

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestGetTargetDeviceFromMtdName(t *testing.T) {
	fs := afero.NewMemMapFs()
	expectedError := fmt.Errorf("some error")

	mum := &MtdUtilsMock{}
	mum.On("GetTargetDeviceFromMtdName", fs, "mtdname").Return("/dev/xxa1", expectedError)

	d, err := mum.GetTargetDeviceFromMtdName(fs, "mtdname")

	assert.Equal(t, "/dev/xxa1", d)
	assert.Equal(t, expectedError, err)

	mum.AssertExpectations(t)
}

func TestMtdIsNAND(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	mum := &MtdUtilsMock{}
	mum.On("MtdIsNAND", "/dev/xxa1").Return(true, expectedError)

	b, err := mum.MtdIsNAND("/dev/xxa1")

	assert.Equal(t, true, b)
	assert.Equal(t, expectedError, err)

	mum.AssertExpectations(t)
}
