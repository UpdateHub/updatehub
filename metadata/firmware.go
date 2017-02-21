package metadata

import "bitbucket.org/ossystems/agent/utils"

type FirmwareMetadata struct {
	ProductUID       string                 `json:"product-uid"`
	DeviceIdentity   map[string]interface{} `json:"device-identity"`
	Version          string                 `json:"version"`
	Hardware         string                 `json:"hardware"`
	HardwareRevision string                 `json:"hardware-revision"`
	DeviceAttributes map[string]interface{} `json:"device-attributes"`
}

func NewFirmwareMetadata(cmd utils.CmdLineExecuter) (*FirmwareMetadata, error) {
	// FIXME: DeviceIdentity and DeviceAttributes

	productUID, err := cmd.Execute("product-uid")
	if err != nil {
		return nil, err
	}

	hardware, err := cmd.Execute("hardware")
	if err != nil {
		return nil, err
	}

	hardwareRevision, err := cmd.Execute("hardware-revision")
	if err != nil {
		return nil, err
	}

	version, err := cmd.Execute("version")
	if err != nil {
		return nil, err
	}

	firmwareMetadata := &FirmwareMetadata{
		ProductUID:       string(productUID),
		Hardware:         string(hardware),
		HardwareRevision: string(hardwareRevision),
		Version:          string(version),
	}

	return firmwareMetadata, nil
}
