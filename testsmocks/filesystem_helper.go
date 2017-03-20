/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package testsmocks

import "github.com/stretchr/testify/mock"

type FileSystemHelperMock struct {
	mock.Mock
}

func (fsm *FileSystemHelperMock) Format(targetDevice string, fsType string, formatOptions string) error {
	args := fsm.Called(targetDevice, fsType, formatOptions)
	return args.Error(0)
}

func (fsm *FileSystemHelperMock) Mount(targetDevice string, mountPath string, fsType string, mountOptions string) error {
	args := fsm.Called(targetDevice, mountPath, fsType, mountOptions)
	return args.Error(0)
}

func (fsm *FileSystemHelperMock) Umount(mountPath string) error {
	args := fsm.Called(mountPath)
	return args.Error(0)
}

func (fsm *FileSystemHelperMock) TempDir(prefix string) (string, error) {
	args := fsm.Called(prefix)
	return args.String(0), args.Error(1)
}
