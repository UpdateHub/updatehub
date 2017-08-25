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
	"path/filepath"

	"github.com/OSSystems/pkg/log"
	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/copy"
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
				CopyBackend:       &copy.ExtendedIO{},
				Permissions:       &utils.PermissionsDefaultImpl{},
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
	CopyBackend            copy.Interface `json:"-"`
	utils.Permissions
	installifdifferent.TargetGetter
	tempDirPath string

	Target        string      `json:"target"`
	TargetType    string      `json:"target-type"`
	TargetPath    string      `json:"target-path"`
	TargetGID     interface{} `json:"target-gid"` // can be string or int
	TargetUID     interface{} `json:"target-uid"` // can be string or int
	TargetMode    string      `json:"target-mode"`
	FSType        string      `json:"filesystem"`
	FormatOptions string      `json:"format-options,omitempty"`
	MustFormat    bool        `json:"format?,omitempty"`
	MountOptions  string      `json:"mount-options,omitempty"`
	ChunkSize     int         `json:"chunk-size,omitempty"`
}

// Setup implementation for the "copy" handler
func (cp *CopyObject) Setup() error {
	log.Debug("'copy' handler Setup")

	if cp.TargetType != "device" {
		finalErr := fmt.Errorf("target-type '%s' is not supported for the 'copy' handler. Its value must be 'device'", cp.TargetType)
		log.Error(finalErr)
		return finalErr
	}

	var err error
	cp.tempDirPath, err = cp.TempDir(cp.FileSystemBackend, "copy-handler")
	if err != nil {
		return err
	}

	if cp.MustFormat {
		err = cp.Format(cp.Target, cp.FSType, cp.FormatOptions)
		if err != nil {
			return err
		}
	}

	err = cp.Mount(cp.Target, cp.tempDirPath, cp.FSType, cp.MountOptions)
	if err != nil {
		return err
	}

	return nil
}

// Install implementation for the "copy" handler
func (cp *CopyObject) Install(downloadDir string) error {
	log.Debug("'copy' handler Install")

	errorList := []error{}

	targetPath := path.Join(cp.tempDirPath, cp.TargetPath)

	err := cp.FileSystemBackend.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil {
		return err
	}

	sourcePath := path.Join(downloadDir, cp.Sha256sum)
	err = cp.CopyBackend.CopyFile(cp.FileSystemBackend, cp.LibArchiveBackend, sourcePath, targetPath, cp.ChunkSize, 0, 0, -1, true, cp.Compressed)
	if err != nil {
		errorList = append(errorList, err)
	}

	if len(errorList) == 0 {
		err = cp.Permissions.ApplyChmod(cp.FileSystemBackend, targetPath, cp.TargetMode)
		if err != nil {
			errorList = append(errorList, err)
		}

		err = cp.Permissions.ApplyChown(targetPath, cp.TargetUID, cp.TargetGID)
		if err != nil {
			errorList = append(errorList, err)
		}
	}

	return utils.MergeErrorList(errorList)
}

// Cleanup implementation for the "copy" handler
func (cp *CopyObject) Cleanup() error {
	log.Debug("'copy' handler Cleanup")

	err := cp.Umount(cp.tempDirPath)
	if err != nil {
		return err
	}

	// make sure there is NO umount error when calling
	// "os.RemoveAll(cp.tempDirPath)" here. because in this case the
	// mounted dir contents would be removed too
	cp.FileSystemBackend.RemoveAll(cp.tempDirPath)
	cp.tempDirPath = ""

	return nil
}

// GetTarget implementation for the "copy" handler
func (cp *CopyObject) GetTarget() string {
	if cp.tempDirPath == "" || cp.TargetPath == "" {
		return ""
	}

	return path.Join(cp.tempDirPath, cp.TargetPath)
}
