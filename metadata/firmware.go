package metadata

type FirmwareMetadata struct {
	ProductUID       string
	DeviceIdentity   map[string]interface{}
	Version          string
	Hardware         string
	HardwareRevision string
	DeviceAttributes map[string]interface{}
}
