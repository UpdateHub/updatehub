package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type firmwareMetadataCmdExecuter struct {
}

func (exe firmwareMetadataCmdExecuter) Execute(cmdline string) ([]byte, error) {
	return []byte(cmdline), nil
}

func TestNewFirmwareMetadata(t *testing.T) {
	expected := &FirmwareMetadata{
		ProductUID:       "product-uid",
		Hardware:         "hardware",
		HardwareRevision: "hardware-revision",
		Version:          "version",
	}

	exe := firmwareMetadataCmdExecuter{}

	firmwareMetadata, err := NewFirmwareMetadata(exe)

	assert.NoError(t, err)
	assert.Equal(t, expected, firmwareMetadata)
}
