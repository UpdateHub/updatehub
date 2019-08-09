/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package permissionsmock

import (
	"fmt"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestApplyChmod(t *testing.T) {
	fs := afero.NewMemMapFs()
	expectedError := fmt.Errorf("some error")

	pm := &PermissionsMock{}
	pm.On("ApplyChmod", fs, "path", "0666").Return(expectedError)

	err := pm.ApplyChmod(fs, "path", "0666")

	assert.Equal(t, expectedError, err)

	pm.AssertExpectations(t)
}

func TestApplyChown(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	pm := &PermissionsMock{}
	pm.On("ApplyChown", "path", "0001", "0001").Return(expectedError)

	err := pm.ApplyChown("path", "0001", "0001")

	assert.Equal(t, expectedError, err)

	pm.AssertExpectations(t)
}
