package copy

import "github.com/stretchr/testify/mock"

type FileSystemHelperMock struct {
	*mock.Mock
}

func (fsm FileSystemHelperMock) Format(targetDevice string, fsType string, formatOptions string) error {
	args := fsm.Called(targetDevice, fsType, formatOptions)
	return args.Error(0)
}

func (fsm FileSystemHelperMock) Mount(targetDevice string, mountPath string, fsType string, mountOptions string) error {
	args := fsm.Called(targetDevice, mountPath, fsType, mountOptions)
	return args.Error(0)
}

func (fsm FileSystemHelperMock) Umount(mountPath string) error {
	args := fsm.Called(mountPath)
	return args.Error(0)
}

func (fsm FileSystemHelperMock) TempDir(prefix string) (string, error) {
	args := fsm.Called(prefix)
	return args.String(0), args.Error(1)
}

type CustomCopierMock struct {
	*mock.Mock
}

func (ccm CustomCopierMock) CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error {
	args := ccm.Called(sourcePath, targetPath, chunkSize, skip, seek, count, truncate, compressed)
	return args.Error(0)
}
