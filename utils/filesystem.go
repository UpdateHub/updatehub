/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"

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
	if err := syscall.Mount(targetDevice, mountPath, fsType, 0, mountOptions); err != nil {
		return fmt.Errorf("Couldn't mount '%s': %s", targetDevice, err)
	}

	return nil
}

func (fs *FileSystem) Umount(mountPath string) error {
	if err := syscall.Unmount(mountPath, 0); err != nil {
		return fmt.Errorf("Couldn't umount '%s': %s", mountPath, err)
	}

	return nil
}

func (fs *FileSystem) TempDir(fsb afero.Fs, prefix string) (string, error) {
	return afero.TempDir(fsb, "", prefix)
}
