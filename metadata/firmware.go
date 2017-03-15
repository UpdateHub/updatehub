package metadata

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"syscall"

	"github.com/spf13/afero"

	"code.ossystems.com.br/updatehub/agent/utils"
)

type FirmwareMetadata struct {
	ProductUID       string            `json:"product-uid"`
	DeviceIdentity   map[string]string `json:"device-identity"`
	Version          string            `json:"version"`
	Hardware         string            `json:"hardware"`
	HardwareRevision string            `json:"hardware-revision"`
	DeviceAttributes map[string]string `json:"device-attributes"`
}

func NewFirmwareMetadata(basePath string, store afero.Fs, cmd utils.CmdLineExecuter) (*FirmwareMetadata, error) {
	// FIXME: DeviceIdentity and DeviceAttributes

	productUID, err := cmd.Execute(path.Join(basePath, "product-uid"))
	if err != nil {
		return nil, err
	}

	hardware, err := cmd.Execute(path.Join(basePath, "hardware"))
	if err != nil {
		return nil, err
	}

	hardwareRevision, err := cmd.Execute(path.Join(basePath, "hardware-revision"))
	if err != nil {
		return nil, err
	}

	version, err := cmd.Execute(path.Join(basePath, "version"))
	if err != nil {
		return nil, err
	}

	deviceIdentity, err := executeHooks(path.Join(basePath, "device-identity.d"), store, cmd)
	if err != nil {
		return nil, err
	}

	deviceAttributes, err := executeHooks(path.Join(basePath, "device-attributes.d"), store, cmd)
	if err != nil {
		return nil, err
	}

	firmwareMetadata := &FirmwareMetadata{
		ProductUID:       string(productUID),
		DeviceIdentity:   deviceIdentity,
		DeviceAttributes: deviceAttributes,
		Hardware:         string(hardware),
		HardwareRevision: string(hardwareRevision),
		Version:          string(version),
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
			keyValueMap[k] = v
		}
	}

	return keyValueMap, nil
}
