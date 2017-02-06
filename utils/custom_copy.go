package utils

import (
	"io"
	"os"
	"time"

	"github.com/spf13/afero"
)

type CustomCopier interface {
	CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error
}

type CustomCopy struct {
	FileSystemBackend afero.Fs
}

func (cc *CustomCopy) CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error {
	source, err := cc.FileSystemBackend.Open(sourcePath)

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

	flags := os.O_RDWR | os.O_CREATE
	if truncate {
		flags = flags | os.O_TRUNC
	}

	target, err := cc.FileSystemBackend.OpenFile(targetPath, flags, 0666)
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
	_, err = Copy(target, source, time.Hour, cancel, chunkSize, count)

	source.Close()
	target.Close()

	return err
}
