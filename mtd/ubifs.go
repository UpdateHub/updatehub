/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package mtd

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"github.com/UpdateHub/updatehub/utils"
	"github.com/spf13/afero"
)

type UbifsUtils interface {
	GetTargetDeviceFromUbiVolumeName(fsBackend afero.Fs, volume string) (string, error)
}

type UbifsUtilsImpl struct {
	utils.CmdLineExecuter
}

func (uui *UbifsUtilsImpl) GetTargetDeviceFromUbiVolumeName(fsBackend afero.Fs, volume string) (string, error) {
	files, err := afero.ReadDir(fsBackend, "/dev")
	if err != nil {
		return "", err
	}

	// foreach "/dev/ubi?" device node we check if the "volume"
	// is within this device node (we must run ubinfo on *device*
	// nodes, so "?" is to exclude *volume* nodes like "/dev/ubi0_1")
	prefix := "ubi"
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), prefix) || len(file.Name()) != len(prefix)+1 {
			continue
		}

		deviceNumber := strings.Replace(file.Name(), "ubi", "", -1)

		// we can ignore the error here since we are dealing with
		// command execution over unknown ubi device nodes. we won't
		// get any collateral damage since we have a RE match right below
		combinedOutput, _ := uui.Execute(fmt.Sprintf("ubinfo -d %s -N %s", deviceNumber, volume))

		// check if first line matches the RE below, if yes, then we found it
		scanner := bufio.NewScanner(strings.NewReader(string(combinedOutput)))
		scanner.Scan()

		r := regexp.MustCompile(`^Volume ID:   (\d) \(on ubi(\d)\)$`)
		matched := r.FindStringSubmatch(scanner.Text())

		if matched != nil && len(matched) == 3 {
			volumeID := matched[1]
			return fmt.Sprintf("/dev/ubi%s_%s", deviceNumber, volumeID), nil
		}
	}

	return "", fmt.Errorf("UBI volume '%s' wasn't found", volume)
}
