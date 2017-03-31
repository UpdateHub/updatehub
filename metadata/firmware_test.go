/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package metadata

import (
	"fmt"
	"testing"

	"github.com/UpdateHub/updatehub/installmodes"
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

func TestCheckSupportedHardware(t *testing.T) {
	testCases := []struct {
		name             string
		hardware         string
		hardwareRevision string
		expectedErr      error
	}{
		{
			"WithNeitherHardwareNorHardwareRevisionOnFirmwareMetadata",
			"",
			"",
			nil,
		},

		{
			"WithMatch",
			"hardware2",
			"revB",
			nil,
		},

		{
			"WithNoMatch",
			"hardware-value",
			"hardware-revision-value",
			fmt.Errorf("this hardware doesn't match the hardware supported by the update"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fm := &FirmwareMetadata{
				ProductUID:       "productuid-value",
				DeviceIdentity:   map[string]string{"id1": "id1-value"},
				DeviceAttributes: map[string]string{"attr1": "attr1-value"},
				Hardware:         tc.hardware,
				HardwareRevision: tc.hardwareRevision,
				Version:          "version-value",
			}

			mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
				Name:              "test",
				CheckRequirements: func() error { return nil },
				GetObject:         func() interface{} { return TestObject{} },
			})
			defer mode.Unregister()

			um, err := NewUpdateMetadata([]byte(ValidJSONMetadata))
			assert.NoError(t, err)

			err = fm.CheckSupportedHardware(um)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}
