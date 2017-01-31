package utils

// FIXME: test this whole file

import (
	"io"
	"os"
	"time"
)

type CustomCopier interface {
	CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error
}

// FIXME: all "CustomCopy" instantiations have to be "CustomCopy{FileOperationsImpl{}}"?
type CustomCopy struct {
	FileOperations
}

type FileOperationsImpl struct {
}

type FileOperations interface {
	Open(name string) (FileInterface, error)
	Create(name string) (FileInterface, error)
}

type FileInterface interface {
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
}

func (foi FileOperationsImpl) Open(name string) (FileInterface, error) {
	return os.Open(name)
}

func (foi FileOperationsImpl) Create(name string) (FileInterface, error) {
	return os.Create(name)
}

func (cc *CustomCopy) CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error {
	source, err := cc.Open(sourcePath)

	if err != nil {
		if pathErr, ok := err.(*os.PathError); ok {
			return pathErr
		}
		return err
	}

	_, err = source.Seek(int64(skip*chunkSize), io.SeekStart)
	if err != nil {
		source.Close()
		return err
	}

	target, err := cc.Create(targetPath)
	if err != nil {
		source.Close()
		return err
	}

	_, err = target.Seek(int64(seek*chunkSize), io.SeekStart)
	if err != nil {
		source.Close()
		target.Close()
		return err
	}

	cancel := make(chan bool)
	_, err = Copy(target, source, time.Hour, cancel, chunkSize)

	source.Close()
	target.Close()

	return err
}
