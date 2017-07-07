/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package filemock

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestRead(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	buffer := []byte{}

	fm := &FileMock{}
	fm.On("Read", buffer).Return(3, expectedError)

	n, err := fm.Read(buffer)

	assert.Equal(t, 3, n)
	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestReadAt(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	buffer := []byte{}
	offset := int64(4)

	fm := &FileMock{}
	fm.On("ReadAt", buffer, offset).Return(3, expectedError)

	n, err := fm.ReadAt(buffer, offset)

	assert.Equal(t, 3, n)
	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestSeek(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	offset := int64(4)
	whence := io.SeekStart

	fm := &FileMock{}
	fm.On("Seek", offset, whence).Return(int64(3), expectedError)

	n, err := fm.Seek(offset, whence)

	assert.Equal(t, int64(3), n)
	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestWrite(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	buffer := []byte("content")

	fm := &FileMock{}
	fm.On("Write", buffer).Return(3, expectedError)

	n, err := fm.Write(buffer)

	assert.Equal(t, 3, n)
	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestWriteAt(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	buffer := []byte("content")
	offset := int64(4)

	fm := &FileMock{}
	fm.On("WriteAt", buffer, offset).Return(3, expectedError)

	n, err := fm.WriteAt(buffer, offset)

	assert.Equal(t, 3, n)
	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestClose(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fm := &FileMock{}
	fm.On("Close").Return(expectedError)

	err := fm.Close()

	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestName(t *testing.T) {
	fm := &FileMock{}
	fm.On("Name").Return("filename")

	filename := fm.Name()

	assert.Equal(t, "filename", filename)

	fm.AssertExpectations(t)
}

func TestReadDir(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/tmp/a", []byte("file a"), 0644)
	defer fs.Remove("/tmp/a")

	expectedFI, err := fs.Stat("/tmp")
	assert.NoError(t, err)

	expectedError := fmt.Errorf("some error")

	fm := &FileMock{}
	ret := []os.FileInfo{expectedFI}
	fm.On("Readdir", 0).Return(ret, expectedError)

	d, err := fm.Readdir(0)

	assert.Equal(t, ret, d)
	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestReadDirNames(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fm := &FileMock{}
	dirList := []string{".", ".."}
	fm.On("Readdirnames", 0).Return(dirList, expectedError)

	dn, err := fm.Readdirnames(0)

	assert.Equal(t, dirList, dn)
	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestStat(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/tmp/a", []byte("file a"), 0644)
	defer fs.Remove("/tmp/a")

	expectedFI, err := fs.Stat("/tmp")
	assert.NoError(t, err)

	expectedError := fmt.Errorf("some error")

	fm := &FileMock{}
	fm.On("Stat").Return(expectedFI, expectedError)

	fi, err := fm.Stat()

	assert.Equal(t, expectedFI, fi)
	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestSync(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fm := &FileMock{}
	fm.On("Sync").Return(expectedError)

	err := fm.Sync()

	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestTruncate(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	size := int64(4)

	fm := &FileMock{}
	fm.On("Truncate", size).Return(expectedError)

	err := fm.Truncate(size)

	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}

func TestWriteString(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	s := "content"

	fm := &FileMock{}
	fm.On("WriteString", s).Return(3, expectedError)

	n, err := fm.WriteString(s)

	assert.Equal(t, 3, n)
	assert.Equal(t, expectedError, err)

	fm.AssertExpectations(t)
}
