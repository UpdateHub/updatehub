package metadata

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type firmwareMetadataCmdExecuter struct {
	store afero.Fs
}

func (exe firmwareMetadataCmdExecuter) Execute(cmdline string) ([]byte, error) {
	if _, err := exe.store.Stat(cmdline); err == nil {
		data, _ := afero.ReadFile(exe.store, cmdline)
		return data, nil
	}

	return []byte(cmdline), nil
}

func TestNewFirmwareMetadata(t *testing.T) {
	expected := &FirmwareMetadata{
		ProductUID: "/product-uid",
		DeviceIdentity: map[string]string{
			"id1": "value",
			"id2": "value",
		},
		DeviceAttributes: map[string]string{
			"attr1": "value",
			"attr2": "value",
		},
		Hardware:         "/hardware",
		HardwareRevision: "/hardware-revision",
		Version:          "/version",
	}

	store := afero.NewMemMapFs()

	exe := firmwareMetadataCmdExecuter{
		store: store,
	}

	files := map[string]string{
		"/device-identity.d/key1":    "id1=value",
		"/device-identity.d/key2":    "id2=value",
		"/device-attributes.d/attr1": "attr1=value",
		"/device-attributes.d/attr2": "attr2=value",
	}

	err := store.MkdirAll("/device-identity.d/", 0755)
	assert.NoError(t, err)

	for k, v := range files {
		err := afero.WriteFile(store, k, []byte(v), 0700)
		assert.NoError(t, err)
	}

	firmwareMetadata, err := NewFirmwareMetadata("/", store, exe)

	assert.NoError(t, err)
	assert.Equal(t, expected, firmwareMetadata)
}
