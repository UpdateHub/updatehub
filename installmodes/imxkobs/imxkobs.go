/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package imxkobs

import (
	"os/exec"
	"path"
	"strconv"

	"github.com/OSSystems/pkg/log"
	"github.com/updatehub/updatehub/installifdifferent"
	"github.com/updatehub/updatehub/installmodes"
	"github.com/updatehub/updatehub/metadata"
	"github.com/updatehub/updatehub/utils"
)

func init() {
	installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "imxkobs",
		CheckRequirements: checkRequirements,
		GetObject:         getObject,
	})
}

func checkRequirements() error {
	_, err := exec.LookPath("kobs-ng")

	return err
}

func getObject() interface{} {
	return &ImxKobsObject{
		CmdLineExecuter: &utils.CmdLine{},
	}
}

// ImxKobsObject encapsulates the "imxkobs" handler data and functions
type ImxKobsObject struct {
	metadata.ObjectMetadata
	utils.CmdLineExecuter
	installifdifferent.TargetGetter

	Add1KPadding    bool   `json:"1k_padding,omitempty"`
	SearchExponent  int    `json:"search_exponent,omitempty"`
	Chip0DevicePath string `json:"chip_0_device_path,omitempty"`
	Chip1DevicePath string `json:"chip_1_device_path,omitempty"`
}

// Setup implementation for the "imxkobs" handler
func (ik *ImxKobsObject) Setup() error {
	log.Debug("'imxkobs' handler Setup")
	return nil
}

// Install implementation for the "imxkobs" handler
func (ik *ImxKobsObject) Install(downloadDir string) error {
	log.Debug("'imxkobs' handler Install")

	cmdline := "kobs-ng init"

	if ik.Add1KPadding {
		cmdline += " -x"
	}

	cmdline += " " + path.Join(downloadDir, ik.Sha256sum)

	if ik.SearchExponent > 0 {
		cmdline += " --search_exponent=" + strconv.Itoa(ik.SearchExponent)
	}

	if ik.Chip0DevicePath != "" {
		cmdline += " --chip_0_device_path=" + ik.Chip0DevicePath
	}

	if ik.Chip1DevicePath != "" {
		cmdline += " --chip_1_device_path=" + ik.Chip1DevicePath
	}

	cmdline += " -v"

	_, err := ik.Execute(cmdline)

	return err
}

// Cleanup implementation for the "imxkobs" handler
func (ik *ImxKobsObject) Cleanup() error {
	log.Debug("'imxkobs' handler Cleanup")
	return nil
}

// GetTarget implementation for the "imxkobs" handler
func (ik *ImxKobsObject) GetTarget() string {
	mtdDevicePath := "/dev/mtd0"

	if ik.Chip0DevicePath != "" {
		mtdDevicePath = ik.Chip0DevicePath
	}

	return mtdDevicePath + "ro"
}
