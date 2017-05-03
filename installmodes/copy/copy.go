/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package copy

import (
	"fmt"
	"path"

	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/installifdifferent"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/utils"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "copy",
		CheckRequirements: func() error { return nil },
		GetObject: func() interface{} {
			return &CopyObject{
				FileSystemHelper: &utils.FileSystem{
					CmdLineExecuter: &utils.CmdLine{},
				},
				LibArchiveBackend: &libarchive.LibArchive{},
				FileSystemBackend: afero.NewOsFs(),
				Copier:            &utils.ExtendedIO{},
				ChunkSize:         128 * 1024,
			}
		},
	})
}

// CopyObject encapsulates the "copy" handler data and functions
type CopyObject struct {
	metadata.ObjectMetadata
	metadata.CompressedObject
	utils.FileSystemHelper `json:"-"`
	LibArchiveBackend      libarchive.API `json:"-"`
	FileSystemBackend      afero.Fs
	utils.Copier           `json:"-"`
	installifdifferent.TargetGetter
	targetPath string

	Target        string `json:"target"`
	TargetType    string `json:"target-type"`
	TargetPath    string `json:"target-path"`
	FSType        string `json:"filesystem"`
	FormatOptions string `json:"format-options,omitempty"`
	MustFormat    bool   `json:"format?,omitempty"`
	MountOptions  string `json:"mount-options,omitempty"`
	ChunkSize     int    `json:"chunk-size,omitempty"`
}

// Setup implementation for the "copy" handler
func (cp *CopyObject) Setup() error {
	if cp.TargetType != "device" {
		return fmt.Errorf("target-type '%s' is not supported for the 'copy' handler. Its value must be 'device'", cp.TargetType)
	}

	return nil
}

// Install implementation for the "copy" handler
func (cp *CopyObject) Install(downloadDir string) error {
	if cp.MustFormat {
		err := cp.Format(cp.Target, cp.FSType, cp.FormatOptions)
		if err != nil {
			return err
		}
	}

	tempDirPath, err := cp.TempDir(cp.FileSystemBackend, "copy-handler")
	if err != nil {
		return err
	}
	// we can't "defer os.RemoveAll(tempDirPath)" here because it
	// could happen an "Umount" error and then the mounted dir
	// contents would be removed as well

	err = cp.Mount(cp.Target, tempDirPath, cp.FSType, cp.MountOptions)
	if err != nil {
		cp.FileSystemBackend.RemoveAll(tempDirPath)
		return err
	}

	// this line is important because "cp.targetPath" is the value
	// used on GetTarget() which is the implementation for install-if-different
	cp.targetPath = path.Join(tempDirPath, cp.TargetPath)

	errorList := []error{}

	sourcePath := path.Join(downloadDir, cp.Sha256sum)
	err = cp.CopyFile(cp.FileSystemBackend, cp.LibArchiveBackend, sourcePath, cp.targetPath, cp.ChunkSize, 0, 0, -1, true, cp.Compressed)
	if err != nil {
		errorList = append(errorList, err)
	}

	umountErr := cp.Umount(tempDirPath)
	if umountErr != nil {
		errorList = append(errorList, umountErr)
	} else {
		cp.FileSystemBackend.RemoveAll(tempDirPath)
	}

	return utils.MergeErrorList(errorList)
}

// Cleanup implementation for the "copy" handler
func (cp *CopyObject) Cleanup() error {
	return nil
}

// GetTarget implementation for the "copy" handler
func (cp *CopyObject) GetTarget() string {
	return cp.targetPath
}
