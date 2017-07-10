/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package ubifsmock

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestGetTargetDeviceFromUbiVolumeName(t *testing.T) {
	fs := afero.NewMemMapFs()
	expectedError := fmt.Errorf("some error")

	uum := &UbifsUtilsMock{}
	uum.On("GetTargetDeviceFromUbiVolumeName", fs, "volume").Return("/dev/xxa1", expectedError)

	d, err := uum.GetTargetDeviceFromUbiVolumeName(fs, "volume")

	assert.Equal(t, "/dev/xxa1", d)
	assert.Equal(t, expectedError, err)

	uum.AssertExpectations(t)
}
