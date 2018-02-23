/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package ubifs

import (
	"fmt"
	"os/exec"
	"path"

	"github.com/OSSystems/pkg/log"
	"github.com/spf13/afero"

	"github.com/updatehub/updatehub/copy"
	"github.com/updatehub/updatehub/installmodes"
	"github.com/updatehub/updatehub/libarchive"
	"github.com/updatehub/updatehub/metadata"
	"github.com/updatehub/updatehub/mtd"
	"github.com/updatehub/updatehub/utils"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "ubifs",
		CheckRequirements: checkRequirements,
		GetObject:         getObject,
	})
}

func checkRequirements() error {
	for _, binary := range []string{"ubiupdatevol", "ubinfo"} {
		_, err := exec.LookPath(binary)
		if err != nil {
			return err
		}
	}

	return nil
}

func getObject() interface{} {
	cle := &utils.CmdLine{}

	return &UbifsObject{
		CmdLineExecuter:   cle,
		CopyBackend:       &copy.ExtendedIO{},
		LibArchiveBackend: &libarchive.LibArchive{},
		FileSystemBackend: afero.NewOsFs(),
		UbifsUtils: &mtd.UbifsUtilsImpl{
			CmdLineExecuter: cle,
		},
	}
}

// UbifsObject encapsulates the "ubifs" handler data and functions
type UbifsObject struct {
	metadata.ObjectMetadata
	metadata.CompressedObject
	utils.CmdLineExecuter
	mtd.UbifsUtils
	CopyBackend       copy.Interface `json:"-"`
	LibArchiveBackend libarchive.API `json:"-"`
	FileSystemBackend afero.Fs

	Target     string `json:"target"`
	TargetType string `json:"target-type"`
}

// Setup implementation for the "ubifs" handler
func (ufs *UbifsObject) Setup() error {
	log.Debug("'ubifs' handler Setup")

	if ufs.TargetType != "ubivolume" {
		finalErr := fmt.Errorf("target-type '%s' is not supported for the 'ubifs' handler. Its value must be 'ubivolume'", ufs.TargetType)
		log.Error(finalErr)
		return finalErr
	}

	return nil
}

// Install implementation for the "ubifs" handler
func (ufs *UbifsObject) Install(downloadDir string) error {
	log.Debug("'ubifs' handler Install")

	targetDevice, err := ufs.GetTargetDeviceFromUbiVolumeName(ufs.FileSystemBackend, ufs.Target)
	if err != nil {
		return err
	}

	srcPath := path.Join(downloadDir, ufs.Sha256sum)

	if ufs.Compressed {
		cmdline := fmt.Sprintf("ubiupdatevol -s %.0f %s -", ufs.UncompressedSize, targetDevice)
		copyErr := ufs.CopyBackend.CopyToProcessStdin(ufs.FileSystemBackend, ufs.LibArchiveBackend, srcPath, cmdline, ufs.Compressed)
		err = copyErr
	} else {
		_, execErr := ufs.Execute(fmt.Sprintf("ubiupdatevol %s %s", targetDevice, srcPath))
		err = execErr
	}

	return err
}

// Cleanup implementation for the "ubifs" handler
func (ufs *UbifsObject) Cleanup() error {
	log.Debug("'ubifs' handler Cleanup")
	return nil
}
