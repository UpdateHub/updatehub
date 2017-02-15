package metadata

type FirmwareMetadata struct {
	ProductUID       string                 `json:"product-uid"`
	DeviceIdentity   map[string]interface{} `json:"device-identity"`
	Version          string                 `json:"version"`
	Hardware         string                 `json:"hardware"`
	HardwareRevision string                 `json:"hardware-revision"`
	DeviceAttributes map[string]interface{} `json:"device-attributes"`
}
