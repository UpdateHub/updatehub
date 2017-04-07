/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package libarchivemock

import (
	"github.com/UpdateHub/updatehub/libarchive"

	"github.com/stretchr/testify/mock"
)

type LibArchiveMock struct {
	mock.Mock
}

func (lam *LibArchiveMock) NewRead() libarchive.Archive {
	args := lam.Called()
	return args.Get(0).(libarchive.Archive)
}

func (lam *LibArchiveMock) ReadSupportFilterAll(a libarchive.Archive) {
	lam.Called(a)
}

func (lam *LibArchiveMock) ReadSupportFormatRaw(a libarchive.Archive) {
	lam.Called(a)
}

func (lam *LibArchiveMock) ReadSupportFormatAll(a libarchive.Archive) {
	lam.Called(a)
}

func (lam *LibArchiveMock) ReadSupportFormatEmpty(a libarchive.Archive) {
	lam.Called(a)
}

func (lam *LibArchiveMock) ReadOpenFileName(a libarchive.Archive, filename string, blockSize int) error {
	args := lam.Called(a, filename, blockSize)
	return args.Error(0)
}

func (lam *LibArchiveMock) ReadFree(a libarchive.Archive) {
	lam.Called(a)
}

func (lam *LibArchiveMock) ReadNextHeader(a libarchive.Archive, e *libarchive.ArchiveEntry) error {
	args := lam.Called(a, e)
	return args.Error(0)
}

func (lam *LibArchiveMock) ReadData(a libarchive.Archive, buffer []byte, length int) (int, error) {
	args := lam.Called(a, buffer, length)
	return args.Int(0), args.Error(1)
}

func (lam *LibArchiveMock) WriteDiskNew() libarchive.Archive {
	args := lam.Called()
	return args.Get(0).(libarchive.Archive)
}

func (lam *LibArchiveMock) WriteDiskSetOptions(a libarchive.Archive, flags int) {
	lam.Called(a, flags)
}

func (lam *LibArchiveMock) WriteDiskSetStandardLookup(a libarchive.Archive) {
	lam.Called(a)
}

func (lam *LibArchiveMock) WriteFree(a libarchive.Archive) {
	lam.Called(a)
}

func (lam *LibArchiveMock) WriteHeader(a libarchive.Archive, e libarchive.ArchiveEntry) error {
	args := lam.Called(a, e)
	return args.Error(0)
}

func (lam *LibArchiveMock) WriteFinishEntry(a libarchive.Archive) error {
	args := lam.Called(a)
	return args.Error(0)
}

func (lam *LibArchiveMock) EntrySize(e libarchive.ArchiveEntry) int64 {
	args := lam.Called(e)
	return args.Get(0).(int64)
}

func (lam *LibArchiveMock) EntrySizeIsSet(e libarchive.ArchiveEntry) bool {
	args := lam.Called(e)
	return args.Bool(0)
}

func (lam *LibArchiveMock) Unpack(tarballPath string, targetPath string, enableRaw bool) error {
	args := lam.Called(tarballPath, targetPath, enableRaw)
	return args.Error(0)
}
