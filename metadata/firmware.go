/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package metadata

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"strings"
	"syscall"

	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/utils"
)

type SupportedHardwareChecker interface {
	CheckSupportedHardware(um *UpdateMetadata) error
}

type FirmwareMetadata struct {
	ProductUID       string            `json:"product-uid"`
	DeviceIdentity   map[string]string `json:"device-identity"`
	Version          string            `json:"version"`
	Hardware         string            `json:"hardware"`
	HardwareRevision string            `json:"hardware-revision"`
	DeviceAttributes map[string]string `json:"device-attributes"`
}

func NewFirmwareMetadata(basePath string, store afero.Fs, cmd utils.CmdLineExecuter) (*FirmwareMetadata, error) {
	productUID, err := cmd.Execute(path.Join(basePath, "product-uid"))
	if err != nil {
		return nil, err
	}

	hardware, err := cmd.Execute(path.Join(basePath, "hardware"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	hardwareRevision, err := cmd.Execute(path.Join(basePath, "hardware-revision"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	version, err := cmd.Execute(path.Join(basePath, "version"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	deviceIdentity, err := executeHooks(path.Join(basePath, "device-identity.d"), store, cmd)
	if err != nil || len(deviceIdentity) == 0 {
		return nil, err
	}

	deviceAttributes, err := executeHooks(path.Join(basePath, "device-attributes.d"), store, cmd)
	if err != nil {
		return nil, err
	}

	firmwareMetadata := &FirmwareMetadata{
		ProductUID:       strings.TrimSpace(string(productUID)),
		DeviceIdentity:   deviceIdentity,
		DeviceAttributes: deviceAttributes,
		Hardware:         strings.TrimSpace(string(hardware)),
		HardwareRevision: strings.TrimSpace(string(hardwareRevision)),
		Version:          strings.TrimSpace(string(version)),
	}

	return firmwareMetadata, nil
}

func executeHooks(basePath string, store afero.Fs, cmd utils.CmdLineExecuter) (map[string]string, error) {
	files, err := afero.ReadDir(store, basePath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Println(err)
		return nil, err
	}

	keyValueMap := map[string]string{}

	for _, file := range files {
		if file.IsDir() || file.Mode()&syscall.S_IXUSR == 0 {
			continue
		}

		output, err := cmd.Execute(path.Join(basePath, file.Name()))
		if err != nil {
			return nil, err
		}

		keyValue, err := keyValueParser(bytes.NewReader(output))
		if err != nil {
			return nil, err
		}

		for k, v := range keyValue {
			keyValueMap[k] = strings.TrimSpace(v)
		}
	}

	return keyValueMap, nil
}

func (fm *FirmwareMetadata) CheckSupportedHardware(um *UpdateMetadata) error {
	if fm.Hardware == "" && fm.HardwareRevision == "" {
		return nil
	}

	for _, h := range um.SupportedHardware {
		if h.Hardware == fm.Hardware && h.HardwareRevision == fm.HardwareRevision {
			return nil
		}
	}

	return fmt.Errorf("this hardware doesn't match the hardware supported by the update")
}
