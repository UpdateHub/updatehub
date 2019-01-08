/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package libarchivemock

import (
	"fmt"
	"testing"

	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/stretchr/testify/assert"
)

func TestNewRead(t *testing.T) {
	expectedArchive := libarchive.Archive{}

	lam := &LibArchiveMock{}
	lam.On("NewRead").Return(expectedArchive)

	a := lam.NewRead()

	assert.Equal(t, expectedArchive, a)

	lam.AssertExpectations(t)
}

func TestReadSupportFilterAll(t *testing.T) {
	a := libarchive.Archive{}

	lam := &LibArchiveMock{}
	lam.On("ReadSupportFilterAll", a).Return(fmt.Errorf("some error"))

	err := lam.ReadSupportFilterAll(a)

	assert.EqualError(t, err, "some error")

	lam.AssertExpectations(t)
}

func TestReadSupportFormatRaw(t *testing.T) {
	a := libarchive.Archive{}

	lam := &LibArchiveMock{}
	lam.On("ReadSupportFormatRaw", a).Return(fmt.Errorf("some error"))

	err := lam.ReadSupportFormatRaw(a)

	assert.EqualError(t, err, "some error")

	lam.AssertExpectations(t)
}

func TestReadSupportFormatAll(t *testing.T) {
	a := libarchive.Archive{}

	lam := &LibArchiveMock{}
	lam.On("ReadSupportFormatAll", a).Return(fmt.Errorf("some error"))

	err := lam.ReadSupportFormatAll(a)

	assert.EqualError(t, err, "some error")

	lam.AssertExpectations(t)
}

func TestReadSupportFormatEmpty(t *testing.T) {
	a := libarchive.Archive{}

	lam := &LibArchiveMock{}
	lam.On("ReadSupportFormatEmpty", a).Return(fmt.Errorf("some error"))

	err := lam.ReadSupportFormatEmpty(a)

	assert.EqualError(t, err, "some error")

	lam.AssertExpectations(t)
}

func TestReadOpenFileName(t *testing.T) {
	a := libarchive.Archive{}
	expectedError := fmt.Errorf("some error")

	lam := &LibArchiveMock{}
	lam.On("ReadOpenFileName", a, "file", 512).Return(expectedError)

	err := lam.ReadOpenFileName(a, "file", 512)

	assert.Equal(t, expectedError, err)

	lam.AssertExpectations(t)
}

func TestReadFree(t *testing.T) {
	a := libarchive.Archive{}

	lam := &LibArchiveMock{}
	lam.On("ReadFree", a)

	lam.ReadFree(a)

	lam.AssertExpectations(t)
}

func TestReadNextHeader(t *testing.T) {
	a := libarchive.Archive{}
	expectedError := fmt.Errorf("some error")

	e := &libarchive.ArchiveEntry{}

	lam := &LibArchiveMock{}
	lam.On("ReadNextHeader", a, e).Return(expectedError)

	err := lam.ReadNextHeader(a, e)

	assert.Equal(t, expectedError, err)

	lam.AssertExpectations(t)
}

func TestReadData(t *testing.T) {
	a := libarchive.Archive{}
	expectedError := fmt.Errorf("some error")
	buffer := make([]byte, 256)

	lam := &LibArchiveMock{}
	lam.On("ReadData", a, buffer, 256).Return(0, expectedError)

	n, err := lam.ReadData(a, buffer, 256)

	assert.Equal(t, 0, n)
	assert.Equal(t, expectedError, err)

	lam.AssertExpectations(t)
}

func TestReadDataSkip(t *testing.T) {
	a := libarchive.Archive{}
	expectedError := fmt.Errorf("some error")

	lam := &LibArchiveMock{}
	lam.On("ReadDataSkip", a).Return(expectedError)

	err := lam.ReadDataSkip(a)

	assert.Equal(t, expectedError, err)

	lam.AssertExpectations(t)
}

func TestWriteDiskNew(t *testing.T) {
	expectedArchive := libarchive.Archive{}

	lam := &LibArchiveMock{}
	lam.On("WriteDiskNew").Return(expectedArchive)

	a := lam.WriteDiskNew()

	assert.Equal(t, expectedArchive, a)

	lam.AssertExpectations(t)
}

func TestWriteDiskSetOptions(t *testing.T) {
	a := libarchive.Archive{}

	lam := &LibArchiveMock{}
	lam.On("WriteDiskSetOptions", a, 8)

	lam.WriteDiskSetOptions(a, 8)

	lam.AssertExpectations(t)
}

func TestWriteDiskSetStandardLookup(t *testing.T) {
	a := libarchive.Archive{}

	lam := &LibArchiveMock{}
	lam.On("WriteDiskSetStandardLookup", a)

	lam.WriteDiskSetStandardLookup(a)

	lam.AssertExpectations(t)
}

func TestWriteFree(t *testing.T) {
	a := libarchive.Archive{}

	lam := &LibArchiveMock{}
	lam.On("WriteFree", a)

	lam.WriteFree(a)

	lam.AssertExpectations(t)
}

func TestWriteHeader(t *testing.T) {
	a := libarchive.Archive{}
	expectedError := fmt.Errorf("some error")

	e := libarchive.ArchiveEntry{}

	lam := &LibArchiveMock{}
	lam.On("WriteHeader", a, e).Return(expectedError)

	err := lam.WriteHeader(a, e)

	assert.Equal(t, expectedError, err)

	lam.AssertExpectations(t)
}

func TestWriteFinishEntry(t *testing.T) {
	a := libarchive.Archive{}
	expectedError := fmt.Errorf("some error")

	lam := &LibArchiveMock{}
	lam.On("WriteFinishEntry", a).Return(expectedError)

	err := lam.WriteFinishEntry(a)

	assert.Equal(t, expectedError, err)

	lam.AssertExpectations(t)
}

func TestEntrySize(t *testing.T) {
	e := libarchive.ArchiveEntry{}

	lam := &LibArchiveMock{}
	lam.On("EntrySize", e).Return(int64(5))

	s := lam.EntrySize(e)

	assert.Equal(t, int64(5), s)

	lam.AssertExpectations(t)
}

func TestEntrySizeIsSet(t *testing.T) {
	e := libarchive.ArchiveEntry{}

	lam := &LibArchiveMock{}
	lam.On("EntrySizeIsSet", e).Return(false)

	b := lam.EntrySizeIsSet(e)

	assert.Equal(t, false, b)

	lam.AssertExpectations(t)
}

func TestEntryPathname(t *testing.T) {
	e := libarchive.ArchiveEntry{}

	lam := &LibArchiveMock{}
	lam.On("EntryPathname", e).Return("file")

	n := lam.EntryPathname(e)

	assert.Equal(t, "file", n)

	lam.AssertExpectations(t)
}

func TestUnpack(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	lam := &LibArchiveMock{}
	lam.On("Unpack", "tarball", "targetpath", true).Return(expectedError)

	err := lam.Unpack("tarball", "targetpath", true)

	assert.Equal(t, expectedError, err)

	lam.AssertExpectations(t)
}
