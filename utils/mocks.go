package utils

import (
	"os"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/mock"
)

type FileOperationsMock struct {
	*mock.Mock
}

func (fom FileOperationsMock) Open(name string) (afero.File, error) {
	args := fom.Called(name)
	return args.Get(0).(afero.File), args.Error(1)
}

func (fom FileOperationsMock) Create(name string) (afero.File, error) {
	args := fom.Called(name)
	return args.Get(0).(afero.File), args.Error(1)
}

func (fom FileOperationsMock) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	args := fom.Called(name, flag, perm)
	return args.Get(0).(afero.File), args.Error(1)
}

func (fom FileOperationsMock) Chmod(name string, mode os.FileMode) error {
	args := fom.Called(name, mode)
	return args.Error(0)
}

func (fom FileOperationsMock) Chtimes(name string, atime time.Time, mtime time.Time) error {
	args := fom.Called(name, atime, mtime)
	return args.Error(0)
}

func (fom FileOperationsMock) Mkdir(name string, perm os.FileMode) error {
	args := fom.Called(name, perm)
	return args.Error(0)
}

func (fom FileOperationsMock) MkdirAll(path string, perm os.FileMode) error {
	args := fom.Called(path, perm)
	return args.Error(0)
}

func (fom FileOperationsMock) Name() string {
	args := fom.Called()
	return args.String(0)
}

func (fom FileOperationsMock) Remove(name string) error {
	args := fom.Called(name)
	return args.Error(0)
}

func (fom FileOperationsMock) RemoveAll(path string) error {
	args := fom.Called(path)
	return args.Error(0)
}

func (fom FileOperationsMock) Rename(oldname, newname string) error {
	args := fom.Called(oldname, newname)
	return args.Error(0)
}

func (fom FileOperationsMock) Stat(name string) (os.FileInfo, error) {
	args := fom.Called(name)
	return args.Get(0).(os.FileInfo), args.Error(1)
}

type FileMock struct {
	*mock.Mock
}

func (fm FileMock) Read(p []byte) (n int, err error) {
	args := fm.Called(p)
	return args.Int(0), args.Error(1)
}

func (fm FileMock) ReadAt(b []byte, off int64) (n int, err error) {
	args := fm.Called(b, off)
	return args.Int(0), args.Error(1)
}

func (fm FileMock) Seek(offset int64, whence int) (ret int64, err error) {
	args := fm.Called(offset, whence)
	return args.Get(0).(int64), args.Error(1)
}

func (fm FileMock) Write(b []byte) (n int, err error) {
	args := fm.Called(b)
	return args.Int(0), args.Error(1)
}

func (fm FileMock) WriteAt(b []byte, off int64) (n int, err error) {
	args := fm.Called(b, off)
	return args.Int(0), args.Error(1)
}

func (fm FileMock) Close() error {
	args := fm.Called()
	return args.Error(0)
}

func (fm FileMock) Name() string {
	args := fm.Called()
	return args.String(0)
}

func (fm FileMock) Readdir(count int) ([]os.FileInfo, error) {
	args := fm.Called(count)
	return args.Get(0).([]os.FileInfo), args.Error(1)
}

func (fm FileMock) Readdirnames(n int) ([]string, error) {
	args := fm.Called(n)
	return args.Get(0).([]string), args.Error(1)
}

func (fm FileMock) Stat() (os.FileInfo, error) {
	args := fm.Called()
	return args.Get(0).(os.FileInfo), args.Error(1)
}

func (fm FileMock) Sync() error {
	args := fm.Called()
	return args.Error(0)
}

func (fm FileMock) Truncate(size int64) error {
	args := fm.Called(size)
	return args.Error(0)
}

func (fm FileMock) WriteString(s string) (ret int, err error) {
	args := fm.Called(s)
	return args.Int(0), args.Error(1)
}
