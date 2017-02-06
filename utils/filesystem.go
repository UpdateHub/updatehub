package utils

import (
	"fmt"
	"io/ioutil"
)

type FileSystemHelper interface {
	Format(targetDevice string, fsType string, formatOptions string) error
	Mount(targetDevice string, mountPath string, fsType string, mountOptions string) error
	TempDir(prefix string) (string, error)
	Umount(mountPath string) error
}

type FileSystem struct {
}

func (fs *FileSystem) Format(targetDevice string, fsType string, formatOptions string) error {
	// FIXME: implement and test this
	return fmt.Errorf("Format not implemented yet")
}

func (fs *FileSystem) Mount(targetDevice string, mountPath string, fsType string, mountOptions string) error {
	// FIXME: implement and test this
	return fmt.Errorf("Mount not implemented yet")
}

func (fs *FileSystem) Umount(mountPath string) error {
	// FIXME: implement and test this
	return fmt.Errorf("Umount not implemented yet")
}

func (fs *FileSystem) TempDir(prefix string) (string, error) {
	// FIXME: test this
	// FIXME: use afero.Fs (receive through parameter here or on struct?)
	return ioutil.TempDir("", prefix)
}
