package testsmocks

import (
	"bitbucket.org/ossystems/agent/libarchive"

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

func (lam *LibArchiveMock) ReadNextHeader(a libarchive.Archive, e libarchive.ArchiveEntry) error {
	args := lam.Called(a, e)
	return args.Error(0)
}

func (lam *LibArchiveMock) ReadData(a libarchive.Archive, buffer []byte, length int) (int, error) {
	args := lam.Called(a, buffer, length)
	return args.Int(0), args.Error(1)
}
