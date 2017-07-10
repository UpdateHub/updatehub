/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package filesystemmock

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestOpen(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/tmp/a", []byte("file a"), 0644)
	defer fs.Remove("/tmp/a")

	expectedFile, err := fs.Open("/tmp/a")
	assert.NoError(t, err)
	defer expectedFile.Close()

	fsbm := &FileSystemBackendMock{}
	fsbm.On("Open", "file.txt").Return(expectedFile, expectedError)

	f, err := fsbm.Open("file.txt")

	assert.Equal(t, expectedFile, f)
	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}

func TestCreate(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/tmp/a", []byte("file a"), 0644)
	defer fs.Remove("/tmp/a")

	expectedFile, err := fs.Open("/tmp/a")
	assert.NoError(t, err)
	defer expectedFile.Close()

	fsbm := &FileSystemBackendMock{}
	fsbm.On("Create", "file.txt").Return(expectedFile, expectedError)

	f, err := fsbm.Create("file.txt")

	assert.Equal(t, expectedFile, f)
	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}

func TestOpenFile(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/tmp/a", []byte("file a"), 0644)
	defer fs.Remove("/tmp/a")

	expectedFile, err := fs.Open("/tmp/a")
	assert.NoError(t, err)
	defer expectedFile.Close()

	fsbm := &FileSystemBackendMock{}
	fsbm.On("OpenFile", "file.txt", os.O_RDWR, os.FileMode(0666)).Return(expectedFile, expectedError)

	f, err := fsbm.OpenFile("file.txt", os.O_RDWR, os.FileMode(0666))

	assert.Equal(t, expectedFile, f)
	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}

func TestChmod(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fsbm := &FileSystemBackendMock{}
	fsbm.On("Chmod", "file.txt", os.FileMode(0666)).Return(expectedError)

	err := fsbm.Chmod("file.txt", os.FileMode(0666))

	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}

func TestChtimes(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fsbm := &FileSystemBackendMock{}
	atime, _ := time.Parse("2006-Jan-02", "2017-Feb-03")
	mtime, _ := time.Parse("2006-Jan-02", "2017-Jan-04")
	fsbm.On("Chtimes", "file.txt", atime, mtime).Return(expectedError)

	err := fsbm.Chtimes("file.txt", atime, mtime)

	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}

func TestMkdir(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fsbm := &FileSystemBackendMock{}
	fsbm.On("Mkdir", "dir", os.FileMode(0666)).Return(expectedError)

	err := fsbm.Mkdir("dir", os.FileMode(0666))

	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}

func TestMkdirAll(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fsbm := &FileSystemBackendMock{}
	fsbm.On("MkdirAll", "dir", os.FileMode(0666)).Return(expectedError)

	err := fsbm.MkdirAll("dir", os.FileMode(0666))

	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}

func TestName(t *testing.T) {
	fsbm := &FileSystemBackendMock{}
	fsbm.On("Name").Return("testname")

	n := fsbm.Name()

	assert.Equal(t, "testname", n)

	fsbm.AssertExpectations(t)
}

func TestRemove(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fsbm := &FileSystemBackendMock{}
	fsbm.On("Remove", "file.txt").Return(expectedError)

	err := fsbm.Remove("file.txt")

	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}

func TestRemoveAll(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fsbm := &FileSystemBackendMock{}
	fsbm.On("RemoveAll", "dir").Return(expectedError)

	err := fsbm.RemoveAll("dir")

	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}

func TestRename(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	fsbm := &FileSystemBackendMock{}
	fsbm.On("Rename", "old.txt", "new.txt").Return(expectedError)

	err := fsbm.Rename("old.txt", "new.txt")

	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}

func TestStat(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/tmp/a", []byte("file a"), 0644)
	defer fs.Remove("/tmp/a")

	expectedFI, err := fs.Stat("/tmp")
	assert.NoError(t, err)

	expectedError := fmt.Errorf("some error")

	fsbm := &FileSystemBackendMock{}
	fsbm.On("Stat", "file.txt").Return(expectedFI, expectedError)

	fi, err := fsbm.Stat("file.txt")

	assert.Equal(t, expectedFI, fi)
	assert.Equal(t, expectedError, err)

	fsbm.AssertExpectations(t)
}
