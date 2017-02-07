package testsmocks

import (
	"os"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
)

type FileSystemBackendMock struct {
	*mock.Mock
}

func (fom FileSystemBackendMock) Open(name string) (afero.File, error) {
	args := fom.Called(name)
	return args.Get(0).(afero.File), args.Error(1)
}

func (fom FileSystemBackendMock) Create(name string) (afero.File, error) {
	args := fom.Called(name)
	return args.Get(0).(afero.File), args.Error(1)
}

func (fom FileSystemBackendMock) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	args := fom.Called(name, flag, perm)
	return args.Get(0).(afero.File), args.Error(1)
}

func (fom FileSystemBackendMock) Chmod(name string, mode os.FileMode) error {
	args := fom.Called(name, mode)
	return args.Error(0)
}

func (fom FileSystemBackendMock) Chtimes(name string, atime time.Time, mtime time.Time) error {
	args := fom.Called(name, atime, mtime)
	return args.Error(0)
}

func (fom FileSystemBackendMock) Mkdir(name string, perm os.FileMode) error {
	args := fom.Called(name, perm)
	return args.Error(0)
}

func (fom FileSystemBackendMock) MkdirAll(path string, perm os.FileMode) error {
	args := fom.Called(path, perm)
	return args.Error(0)
}

func (fom FileSystemBackendMock) Name() string {
	args := fom.Called()
	return args.String(0)
}

func (fom FileSystemBackendMock) Remove(name string) error {
	args := fom.Called(name)
	return args.Error(0)
}

func (fom FileSystemBackendMock) RemoveAll(path string) error {
	args := fom.Called(path)
	return args.Error(0)
}

func (fom FileSystemBackendMock) Rename(oldname, newname string) error {
	args := fom.Called(oldname, newname)
	return args.Error(0)
}

func (fom FileSystemBackendMock) Stat(name string) (os.FileInfo, error) {
	args := fom.Called(name)
	return args.Get(0).(os.FileInfo), args.Error(1)
}
