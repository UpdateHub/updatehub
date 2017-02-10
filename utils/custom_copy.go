package utils

import (
	"io"
	"os"
	"time"

	"bitbucket.org/ossystems/agent/libarchive"

	"github.com/spf13/afero"
)

type CustomCopier interface {
	CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error
}

type CustomCopy struct {
	FileSystemBackend afero.Fs
}

func (cc *CustomCopy) CopyFile(sourcePath string, targetPath string, chunkSize int, skip int, seek int, count int, truncate bool, compressed bool) error {
	var err error

	flags := os.O_RDWR | os.O_CREATE
	if truncate {
		flags = flags | os.O_TRUNC
	}

	target, err := cc.FileSystemBackend.OpenFile(targetPath, flags, 0666)
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = target.Seek(int64(seek*chunkSize), io.SeekStart)
	if err != nil {
		return err
	}

	if compressed {
		a := libarchive.LibArchive{}
		err = LACopy(a, target, sourcePath, chunkSize, skip, seek, count, truncate)
	} else {
		source, sourceErr := cc.FileSystemBackend.Open(sourcePath)
		if sourceErr != nil {
			if pathErr, ok := err.(*os.PathError); ok {
				return pathErr
			}
			return sourceErr
		}
		defer source.Close()

		_, sourceErr = source.Seek(int64(skip*chunkSize), io.SeekStart)
		if sourceErr != nil {
			return sourceErr
		}

		cancel := make(chan bool)
		_, err = Copy(target, source, time.Hour, cancel, chunkSize, count)
	}

	return err
}
