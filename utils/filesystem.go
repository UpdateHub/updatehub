/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

/*
#include <stdlib.h>
#include <sys/mount.h>
#include <errno.h>
#include <string.h>

static int errno_wrapper() {
    return errno;
}
*/
import "C"
import (
	"fmt"
	"io/ioutil"
	"unsafe"
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
	cTargetDevice := C.CString(targetDevice)
	defer C.free(unsafe.Pointer(cTargetDevice))

	cMountPath := C.CString(mountPath)
	defer C.free(unsafe.Pointer(cMountPath))

	cFSType := C.CString(fsType)
	defer C.free(unsafe.Pointer(cFSType))

	cMountOptions := C.CString(mountOptions)
	defer C.free(unsafe.Pointer(cMountOptions))

	r := C.mount(cTargetDevice, cMountPath, cFSType, 0, unsafe.Pointer(cMountOptions))
	if r == -1 {
		return fmt.Errorf("Couldn't mount '%s': %s", targetDevice, C.GoString(C.strerror(C.errno_wrapper())))
	}

	return nil
}

func (fs *FileSystem) Umount(mountPath string) error {
	cMountPath := C.CString(mountPath)
	defer C.free(unsafe.Pointer(cMountPath))

	r := C.umount(cMountPath)
	if r == -1 {
		return fmt.Errorf("Couldn't umount '%s': %s", mountPath, C.GoString(C.strerror(C.errno_wrapper())))
	}

	return nil
}

func (fs *FileSystem) TempDir(prefix string) (string, error) {
	// FIXME: test this
	// FIXME: use afero.Fs (receive through parameter here or on struct?)
	return ioutil.TempDir("", prefix)
}
