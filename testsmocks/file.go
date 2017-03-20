/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package testsmocks

import (
	"os"

	"github.com/stretchr/testify/mock"
)

type FileMock struct {
	mock.Mock
}

func (fm *FileMock) Read(p []byte) (n int, err error) {
	args := fm.Called(p)
	return args.Int(0), args.Error(1)
}

func (fm *FileMock) ReadAt(b []byte, off int64) (n int, err error) {
	args := fm.Called(b, off)
	return args.Int(0), args.Error(1)
}

func (fm *FileMock) Seek(offset int64, whence int) (ret int64, err error) {
	args := fm.Called(offset, whence)
	return args.Get(0).(int64), args.Error(1)
}

func (fm *FileMock) Write(b []byte) (n int, err error) {
	args := fm.Called(b)
	return args.Int(0), args.Error(1)
}

func (fm *FileMock) WriteAt(b []byte, off int64) (n int, err error) {
	args := fm.Called(b, off)
	return args.Int(0), args.Error(1)
}

func (fm *FileMock) Close() error {
	args := fm.Called()
	return args.Error(0)
}

func (fm *FileMock) Name() string {
	args := fm.Called()
	return args.String(0)
}

func (fm *FileMock) Readdir(count int) ([]os.FileInfo, error) {
	args := fm.Called(count)
	return args.Get(0).([]os.FileInfo), args.Error(1)
}

func (fm *FileMock) Readdirnames(n int) ([]string, error) {
	args := fm.Called(n)
	return args.Get(0).([]string), args.Error(1)
}

func (fm *FileMock) Stat() (os.FileInfo, error) {
	args := fm.Called()
	return args.Get(0).(os.FileInfo), args.Error(1)
}

func (fm *FileMock) Sync() error {
	args := fm.Called()
	return args.Error(0)
}

func (fm *FileMock) Truncate(size int64) error {
	args := fm.Called(size)
	return args.Error(0)
}

func (fm *FileMock) WriteString(s string) (ret int, err error) {
	args := fm.Called(s)
	return args.Int(0), args.Error(1)
}
