/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package tarball

import (
	"fmt"
	"path"

	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/utils"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "tarball",
		CheckRequirements: func() error { return nil },
		GetObject: func() interface{} {
			return &TarballObject{
				FileSystemHelper:  &utils.FileSystem{},
				LibArchiveBackend: &libarchive.LibArchive{},
				FileSystemBackend: afero.NewOsFs(),
				Copier:            &utils.ExtendedIO{},
				MtdUtils:          &utils.MtdUtilsImpl{},
				UbifsUtils: &utils.UbifsUtilsImpl{
					CmdLineExecuter: &utils.CmdLine{},
				},
			}
		},
	})
}

// TarballObject encapsulates the "tarball" handler data and functions
type TarballObject struct {
	metadata.ObjectMetadata
	metadata.CompressedObject
	utils.FileSystemHelper `json:"-"`
	LibArchiveBackend      libarchive.API `json:"-"`
	FileSystemBackend      afero.Fs
	utils.Copier           `json:"-"`
	utils.MtdUtils
	utils.UbifsUtils

	Target        string `json:"target"`
	TargetType    string `json:"target-type"`
	TargetPath    string `json:"target-path"`
	FSType        string `json:"filesystem"`
	FormatOptions string `json:"format-options,omitempty"`
	MustFormat    bool   `json:"format?,omitempty"`
	MountOptions  string `json:"mount-options,omitempty"`

	targetDevice string // this is NOT obtained from the json but from the "Setup()"
}

// Setup implementation for the "tarball" handler
func (tb *TarballObject) Setup() error {
	switch tb.TargetType {
	case "device":
		tb.targetDevice = tb.Target
	case "mtdname":
		td, err := tb.MtdUtils.GetTargetDeviceFromMtdName(tb.FileSystemBackend, tb.Target)
		if err != nil {
			return err
		}

		tb.targetDevice = td
	case "ubivolume":
		td, err := tb.GetTargetDeviceFromUbiVolumeName(tb.FileSystemBackend, tb.Target)
		if err != nil {
			return err
		}

		tb.targetDevice = td
	default:
		return fmt.Errorf("target-type '%s' is not supported for the 'tarball' handler. Its value must be one of: 'device', 'ubivolume' or 'mtdname'", tb.TargetType)
	}

	return nil
}

// Install implementation for the "tarball" handler
func (tb *TarballObject) Install() error {
	if tb.MustFormat {
		err := tb.Format(tb.Target, tb.FSType, tb.FormatOptions)
		if err != nil {
			return err
		}
	}

	tempDirPath, err := tb.TempDir("tarball-handler")
	if err != nil {
		return err
	}
	// we can't "defer os.RemoveAll(tempDirPath)" here because it
	// could happen an "Umount" error and then the mounted dir
	// contents would be removed as well

	err = tb.Mount(tb.Target, tempDirPath, tb.FSType, tb.MountOptions)
	if err != nil {
		tb.FileSystemBackend.RemoveAll(tempDirPath)
		return err
	}

	targetPath := path.Join(tempDirPath, tb.TargetPath)

	errorList := []error{}

	// FIXME: on sourcePath we need to: path.Join(tb.UpdateDir, tb.Sha256sum)
	err = tb.LibArchiveBackend.Unpack(tb.Sha256sum, targetPath, false)
	if err != nil {
		errorList = append(errorList, err)
	}

	umountErr := tb.Umount(tempDirPath)
	if umountErr != nil {
		errorList = append(errorList, umountErr)
	} else {
		tb.FileSystemBackend.RemoveAll(tempDirPath)
	}

	return utils.MergeErrorList(errorList)
}

// Cleanup implementation for the "tarball" handler
func (tb *TarballObject) Cleanup() error {
	return nil
}
