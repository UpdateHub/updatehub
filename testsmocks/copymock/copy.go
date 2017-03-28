/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package copymock

import (
	"io"
	"time"

	"github.com/UpdateHub/updatehub/libarchive"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
)

type CopierMock struct {
	mock.Mock
}

func (cm *CopierMock) CopyFile(fsBackend afero.Fs, libarchiveBackend libarchive.API, sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error {
	args := cm.Called(fsBackend, libarchiveBackend, sourcePath, targetPath, chunkSize, skip, seek, count, truncate, compressed)
	return args.Error(0)
}

func (cm *CopierMock) Copy(wr io.Writer, rd io.Reader, timeout time.Duration, cancel <-chan bool, chunkSize int, skip int, count int, compressed bool) (bool, error) {
	args := cm.Called(wr, rd, timeout, cancel, chunkSize, skip, count, compressed)
	return args.Bool(0), args.Error(1)
}

func (cm *CopierMock) CopyToProcessStdin(fsBackend afero.Fs, libarchiveBackend libarchive.API, sourcePath string, processCmdline string, compressed bool) error {
	args := cm.Called(fsBackend, libarchiveBackend, sourcePath, processCmdline, compressed)
	return args.Error(0)
}
