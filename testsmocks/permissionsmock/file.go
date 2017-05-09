/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package permissionsmock

import (
	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
)

type PermissionsMock struct {
	mock.Mock
}

func (pm *PermissionsMock) ApplyChmod(fsb afero.Fs, filepath string, mode string) error {
	args := pm.Called(fsb, filepath, mode)
	return args.Error(0)
}

func (pm *PermissionsMock) ApplyChown(filepath string, uid interface{}, gid interface{}) error {
	args := pm.Called(filepath, uid, gid)
	return args.Error(0)
}
