package utils

import "fmt"

type CustomCopier interface {
	CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error
}

type CustomCopy struct {
}

func (fs *CustomCopy) CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error {
	// FIXME: implement and test this
	return fmt.Errorf("CopyFile not implemented yet")
}
