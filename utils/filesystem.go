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
	"os/exec"
	"strings"
	"unsafe"

	"github.com/spf13/afero"
)

type FileSystemHelper interface {
	Format(targetDevice string, fsType string, formatOptions string) error
	Mount(targetDevice string, mountPath string, fsType string, mountOptions string) error
	TempDir(fsb afero.Fs, prefix string) (string, error)
	Umount(mountPath string) error
}

type FileSystem struct {
	CmdLineExecuter
}

func cmdlineForFormat(devicePath string, fsType string, formatOptions string) string {
	switch fsType {
	case "jffs2":
		return fmt.Sprintf("flash_erase -j %s %s 0 0", formatOptions, devicePath)
	case "ext2":
		fallthrough
	case "ext3":
		fallthrough
	case "ext4":
		return fmt.Sprintf("mkfs.%s -F %s %s", fsType, formatOptions, devicePath)
	case "ubifs":
		return fmt.Sprintf("mkfs.%s -y %s %s", fsType, formatOptions, devicePath)
	case "xfs":
		return fmt.Sprintf("mkfs.%s -f %s %s", fsType, formatOptions, devicePath)
	case "btrfs":
		fallthrough
	case "vfat":
		fallthrough
	case "f2fs":
		return fmt.Sprintf("mkfs.%s %s %s", fsType, formatOptions, devicePath)
	}

	return ""
}

func (fs *FileSystem) Format(targetDevice string, fsType string, formatOptions string) error {
	cmdline := cmdlineForFormat(targetDevice, fsType, formatOptions)

	if cmdline == "" {
		return fmt.Errorf("Couldn't format '%s': fs type '%s' is not supported", targetDevice, fsType)
	}

	// a segfault is ensured to not happen since the "if" above checks
	// for an empty string
	binary := strings.Split(cmdline, " ")[0]

	_, err := exec.LookPath(binary)
	if err != nil {
		return err
	}

	_, err = fs.Execute(cmdline)
	if err != nil {
		return fmt.Errorf("couldn't format '%s'. cmdline error: %s", targetDevice, err)
	}

	return nil
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

func (fs *FileSystem) TempDir(fsb afero.Fs, prefix string) (string, error) {
	return afero.TempDir(fsb, "", prefix)
}
