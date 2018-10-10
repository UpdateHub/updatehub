/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package flash

import (
	"fmt"
	"os/exec"
	"path"

	"github.com/OSSystems/pkg/log"
	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/installifdifferent"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/mtd"
	"github.com/UpdateHub/updatehub/utils"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "flash",
		CheckRequirements: checkRequirements,
		GetObject:         getObject,
	})
}

func checkRequirements() error {
	for _, binary := range []string{"nandwrite", "flashcp", "flash_erase"} {
		_, err := exec.LookPath(binary)
		if err != nil {
			return err
		}
	}

	return nil
}

func getObject() interface{} {
	return &FlashObject{
		CmdLineExecuter:   &utils.CmdLine{},
		FileSystemBackend: afero.NewOsFs(),
		MtdUtils:          &mtd.MtdUtilsImpl{},
	}
}

// FlashObject encapsulates the "flash" handler data and functions
type FlashObject struct {
	metadata.ObjectMetadata
	utils.CmdLineExecuter
	FileSystemBackend afero.Fs
	mtd.MtdUtils
	installifdifferent.TargetProvider

	Target     string `json:"target"`
	TargetType string `json:"target-type"`

	targetDevice string // this is NOT obtained from the json but from the "Setup()"
}

// Setup implementation for the "flash" handler
func (f *FlashObject) Setup() error {
	log.Debug("'flash' handler Setup")

	switch f.TargetType {
	case "device":
		f.targetDevice = f.Target
	case "mtdname":
		td, err := f.MtdUtils.GetTargetDeviceFromMtdName(f.FileSystemBackend, f.Target)
		if err != nil {
			return err
		}

		f.targetDevice = td
	default:
		finalErr := fmt.Errorf("target-type '%s' is not supported for the 'flash' handler. Its value must be either 'device' or 'mtdname'", f.TargetType)
		log.Error(finalErr)
		return finalErr
	}

	return nil
}

// Install implementation for the "flash" handler
func (f *FlashObject) Install(downloadDir string) error {
	log.Debug("'flash' handler Install")

	isNand, err := f.MtdUtils.MtdIsNAND(f.targetDevice)
	if err != nil {
		return err
	}

	_, err = f.Execute(fmt.Sprintf("flash_erase %s 0 0", f.targetDevice))
	if err != nil {
		return err
	}

	srcPath := path.Join(downloadDir, f.Sha256sum)

	if isNand {
		_, nandErr := f.Execute(fmt.Sprintf("nandwrite -p %s %s", f.targetDevice, srcPath))
		err = nandErr
	} else {
		_, norErr := f.Execute(fmt.Sprintf("flashcp %s %s", srcPath, f.targetDevice))
		err = norErr
	}

	return err
}

// Cleanup implementation for the "flash" handler
func (f *FlashObject) Cleanup() error {
	log.Debug("'flash' handler Cleanup")
	return nil
}

// GetTarget implementation for the "flash" handler
func (f *FlashObject) GetTarget() string {
	return f.targetDevice + "ro"
}

// SetupTarget implementation for the "flash" handler
func (f *FlashObject) SetupTarget(target afero.File) {
}
