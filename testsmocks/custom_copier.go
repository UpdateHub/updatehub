package testsmocks

import "github.com/stretchr/testify/mock"

type CustomCopierMock struct {
	*mock.Mock
}

func (ccm CustomCopierMock) CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error {
	args := ccm.Called(sourcePath, targetPath, chunkSize, skip, seek, count, truncate, compressed)
	return args.Error(0)
}
