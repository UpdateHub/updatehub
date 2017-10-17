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

	"github.com/OSSystems/pkg/log"
	"github.com/spf13/afero"

	"github.com/updatehub/updatehub/utils"
)

type SupportedHardwareChecker interface {
	CheckSupportedHardware(um *UpdateMetadata) error
}

type FirmwareMetadata struct {
	ProductUID       string            `json:"product-uid"`
	DeviceIdentity   map[string]string `json:"device-identity"`
	Version          string            `json:"version"`
	Hardware         string            `json:"hardware"`
	DeviceAttributes map[string]string `json:"device-attributes"`
}

func NewFirmwareMetadata(basePath string, store afero.Fs, cmd utils.CmdLineExecuter) (*FirmwareMetadata, error) {
	log.Info("reading firmware metadata")

	store.MkdirAll(basePath, 0755)

	productUID, err := cmd.Execute(path.Join(basePath, "product-uid"))
	if err != nil {
		return nil, err
	}

	hardware, err := cmd.Execute(path.Join(basePath, "hardware"))
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
		Version:          strings.TrimSpace(string(version)),
	}

	log.Debug("    product-uid: ", firmwareMetadata.ProductUID)
	log.Debug("    hardware: ", firmwareMetadata.Hardware)
	log.Debug("    version: ", firmwareMetadata.Version)
	log.Debug("    device-identity: ", firmwareMetadata.DeviceIdentity)
	log.Debug("    device-attributes: ", firmwareMetadata.DeviceAttributes)

	return firmwareMetadata, nil
}

func executeHooks(basePath string, store afero.Fs, cmd utils.CmdLineExecuter) (map[string]string, error) {
	files, err := afero.ReadDir(store, basePath)
	if err != nil && !os.IsNotExist(err) {
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
	if fm.Hardware == "" {
		log.Warn("firmware metadata hardware is empty, so skipping supported hardware match")
		return nil
	}

	if s, ok := um.SupportedHardware.(string); ok && s == "any" {
		log.Debug("hardware is supported by the update")
		return nil
	}

	hwList, ok := um.SupportedHardware.([]interface{})
	if !ok {
		err := fmt.Errorf("unknown supported hardware format in the update metadata")
		log.Error(err)
		return err
	}

	for _, hw := range hwList {
		if hw.(string) == fm.Hardware {
			log.Debug("hardware is supported by the update")
			return nil
		}
	}

	err := fmt.Errorf("this hardware doesn't match the hardware supported by the update")
	log.Error(err)
	return err
}
