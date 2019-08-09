/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package fileinfomock

import (
	"os"
	"time"

	"github.com/stretchr/testify/mock"
)

type FileInfoMock struct {
	mock.Mock
}

func (fim *FileInfoMock) Name() string {
	args := fim.Called()
	return args.String(0)
}

func (fim *FileInfoMock) Size() int64 {
	args := fim.Called()
	return args.Get(0).(int64)
}

func (fim *FileInfoMock) Mode() os.FileMode {
	args := fim.Called()
	return args.Get(0).(os.FileMode)
}

func (fim *FileInfoMock) ModTime() time.Time {
	args := fim.Called()
	return args.Get(0).(time.Time)
}

func (fim *FileInfoMock) IsDir() bool {
	args := fim.Called()
	return args.Bool(0)
}

func (fim *FileInfoMock) Sys() interface{} {
	args := fim.Called()
	return args.Get(0)
}
