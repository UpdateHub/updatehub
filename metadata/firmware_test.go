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
	"os"
	"path"
	"syscall"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/UpdateHub/updatehub/installmodes"
	"github.com/UpdateHub/updatehub/testsmocks/cmdlinemock"
)

func TestNewFirmwareMetadataWithInexistantPath(t *testing.T) {
	metadataPath := "/tmp/inexistant/subdir"

	clm := &cmdlinemock.CmdLineExecuterMock{}
	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(""), fmt.Errorf("not found"))

	fs := afero.NewMemMapFs()

	firmwareMetadata, err := NewFirmwareMetadata(metadataPath, fs, clm)

	assert.EqualError(t, err, "not found")
	assert.Equal(t, ((*FirmwareMetadata)(nil)), firmwareMetadata)

	dirExists, err := afero.Exists(fs, metadataPath)
	assert.True(t, dirExists)
	assert.NoError(t, err)

	clm.AssertExpectations(t)
}

func TestNewFirmwareMetadataWithSuccess(t *testing.T) {
	const (
		metadataPath = "/"
	)

	testCases := []struct {
		name          string
		hardwareError error
		versionError  error
	}{
		{
			"WithNoErrorOnAllOptionalFields",
			nil,
			nil,
		},

		{
			"WithHardwareScriptNotFound",
			&os.PathError{
				Op:   "open",
				Path: path.Join(metadataPath, "hardware"),
				Err:  syscall.ENOENT,
			},
			nil,
		},

		{
			"WithVersionScriptNotFound",
			nil,
			&os.PathError{
				Op:   "open",
				Path: path.Join(metadataPath, "version"),
				Err:  syscall.ENOENT,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			productUID := "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"
			hardware := "board"
			version := "1.1"

			expected := &FirmwareMetadata{
				ProductUID: productUID,
				DeviceIdentity: map[string]string{
					"id1": "value1",
					"id2": "value2",
				},
				DeviceAttributes: map[string]string{
					"attr1": "value1",
					"attr2": "value2",
				},
				Hardware: hardware,
				Version:  version,
			}

			clm := &cmdlinemock.CmdLineExecuterMock{}

			clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(productUID), nil)
			clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte(hardware), tc.hardwareError)
			clm.On("Execute", path.Join(metadataPath, "version")).Return([]byte(version), tc.versionError)
			clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key1")).Return([]byte("id1=value1"), nil)
			clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key2")).Return([]byte("id2=value2"), nil)
			clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr1")).Return([]byte("attr1=value1"), nil)
			clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr2")).Return([]byte("attr2=value2"), nil)

			store := afero.NewMemMapFs()

			files := map[string]string{
				"/device-identity.d/key1":    "id1=value1",
				"/device-identity.d/key2":    "id2=value2",
				"/device-attributes.d/attr1": "attr1=value",
				"/device-attributes.d/attr2": "attr2=value",
			}

			for k, v := range files {
				err := afero.WriteFile(store, k, []byte(v), 0700)
				assert.NoError(t, err)
			}

			firmwareMetadata, err := NewFirmwareMetadata(metadataPath, store, clm)

			assert.NoError(t, err)
			assert.Equal(t, expected, firmwareMetadata)

			clm.AssertExpectations(t)
		})
	}
}

func TestNewFirmwareMetadataWithSuccessWithNewLineCharacters(t *testing.T) {
	const (
		metadataPath = "/"
	)

	productUID := "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"
	hardware := "board"
	version := "1.1"

	expected := &FirmwareMetadata{
		ProductUID: productUID,
		DeviceIdentity: map[string]string{
			"id1": "value1",
			"id2": "value2",
		},
		DeviceAttributes: map[string]string{
			"attr1": "value1",
			"attr2": "value2",
		},
		Hardware: hardware,
		Version:  version,
	}

	clm := &cmdlinemock.CmdLineExecuterMock{}

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(productUID+"\n"), nil)
	clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte(hardware+"\n"), nil)
	clm.On("Execute", path.Join(metadataPath, "version")).Return([]byte(version+"\n"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key1")).Return([]byte("id1=value1"+"\n"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key2")).Return([]byte("id2=value2"+"\n"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr1")).Return([]byte("attr1=value1"+"\n"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr2")).Return([]byte("attr2=value2"+"\n"), nil)

	store := afero.NewMemMapFs()

	files := map[string]string{
		"/device-identity.d/key1":    "id1=value1",
		"/device-identity.d/key2":    "id2=value2",
		"/device-attributes.d/attr1": "attr1=value",
		"/device-attributes.d/attr2": "attr2=value",
	}

	for k, v := range files {
		err := afero.WriteFile(store, k, []byte(v), 0700)
		assert.NoError(t, err)
	}

	firmwareMetadata, err := NewFirmwareMetadata(metadataPath, store, clm)

	assert.NoError(t, err)
	assert.Equal(t, expected, firmwareMetadata)

	clm.AssertExpectations(t)
}

func TestNewFirmwareMetadataWithNoDeviceIdentityScriptsFound(t *testing.T) {
	metadataPath := "/"
	productUID := "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"
	hardware := "board"
	version := "1.1"

	clm := &cmdlinemock.CmdLineExecuterMock{}

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(productUID), nil)
	clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte(hardware), nil)
	clm.On("Execute", path.Join(metadataPath, "version")).Return([]byte(version), nil)

	store := afero.NewMemMapFs()

	firmwareMetadata, err := NewFirmwareMetadata(metadataPath, store, clm)

	assert.NoError(t, err)
	assert.Equal(t, ((*FirmwareMetadata)(nil)), firmwareMetadata)

	clm.AssertExpectations(t)
}

func TestNewFirmwareMetadataWithNoDeviceAttributesScriptsFound(t *testing.T) {
	metadataPath := "/"
	productUID := "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"
	hardware := "board"
	version := "1.1"

	expected := &FirmwareMetadata{
		ProductUID: productUID,
		DeviceIdentity: map[string]string{
			"id1": "value1",
			"id2": "value2",
		},
		DeviceAttributes: map[string]string{},
		Hardware:         hardware,
		Version:          version,
	}

	clm := &cmdlinemock.CmdLineExecuterMock{}

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(productUID), nil)
	clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte(hardware), nil)
	clm.On("Execute", path.Join(metadataPath, "version")).Return([]byte(version), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key1")).Return([]byte("id1=value1"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key2")).Return([]byte("id2=value2"), nil)

	store := afero.NewMemMapFs()

	files := map[string]string{
		"/device-identity.d/key1": "id1=value1",
		"/device-identity.d/key2": "id2=value2",
	}

	for k, v := range files {
		err := afero.WriteFile(store, k, []byte(v), 0700)
		assert.NoError(t, err)
	}

	firmwareMetadata, err := NewFirmwareMetadata(metadataPath, store, clm)

	assert.NoError(t, err)
	assert.Equal(t, expected, firmwareMetadata)

	clm.AssertExpectations(t)
}

func TestNewFirmwareMetadataWithProductUIDError(t *testing.T) {
	metadataPath := "/"

	clm := &cmdlinemock.CmdLineExecuterMock{}

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(""), fmt.Errorf("productuid error"))

	store := afero.NewMemMapFs()

	firmwareMetadata, err := NewFirmwareMetadata(metadataPath, store, clm)

	assert.EqualError(t, err, "productuid error")
	assert.Equal(t, ((*FirmwareMetadata)(nil)), firmwareMetadata)

	clm.AssertExpectations(t)
}

func TestNewFirmwareMetadataWithHardwareError(t *testing.T) {
	metadataPath := "/"

	clm := &cmdlinemock.CmdLineExecuterMock{}

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte("229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"), nil)
	clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte(""), fmt.Errorf("hardware error"))

	store := afero.NewMemMapFs()

	firmwareMetadata, err := NewFirmwareMetadata(metadataPath, store, clm)

	assert.EqualError(t, err, "hardware error")
	assert.Equal(t, ((*FirmwareMetadata)(nil)), firmwareMetadata)

	clm.AssertExpectations(t)
}

func TestNewFirmwareMetadataWithVersionError(t *testing.T) {
	metadataPath := "/"

	clm := &cmdlinemock.CmdLineExecuterMock{}

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte("229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"), nil)
	clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte("board"), nil)
	clm.On("Execute", path.Join(metadataPath, "version")).Return([]byte(""), fmt.Errorf("version error"))

	store := afero.NewMemMapFs()

	firmwareMetadata, err := NewFirmwareMetadata(metadataPath, store, clm)

	assert.EqualError(t, err, "version error")
	assert.Equal(t, ((*FirmwareMetadata)(nil)), firmwareMetadata)

	clm.AssertExpectations(t)
}

func TestNewFirmwareMetadataWithDeviceIdentityError(t *testing.T) {
	metadataPath := "/"
	productUID := "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"
	hardware := "board"
	version := "1.1"

	clm := &cmdlinemock.CmdLineExecuterMock{}

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(productUID), nil)
	clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte(hardware), nil)
	clm.On("Execute", path.Join(metadataPath, "version")).Return([]byte(version), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key1")).Return([]byte(""), fmt.Errorf("device identity error"))

	store := afero.NewMemMapFs()

	files := map[string]string{
		"/device-identity.d/key1":    "id1=value1",
		"/device-identity.d/key2":    "id2=value2",
		"/device-attributes.d/attr1": "attr1=value",
		"/device-attributes.d/attr2": "attr2=value",
	}

	for k, v := range files {
		err := afero.WriteFile(store, k, []byte(v), 0700)
		assert.NoError(t, err)
	}

	firmwareMetadata, err := NewFirmwareMetadata(metadataPath, store, clm)

	assert.EqualError(t, err, "device identity error")
	assert.Equal(t, ((*FirmwareMetadata)(nil)), firmwareMetadata)

	clm.AssertExpectations(t)
}

func TestNewFirmwareMetadataWithDeviceAttributesError(t *testing.T) {
	metadataPath := "/"
	productUID := "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"
	hardware := "board"
	version := "1.1"

	clm := &cmdlinemock.CmdLineExecuterMock{}

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(productUID), nil)
	clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte(hardware), nil)
	clm.On("Execute", path.Join(metadataPath, "version")).Return([]byte(version), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key1")).Return([]byte("id1=value1"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key2")).Return([]byte("id2=value2"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr1")).Return([]byte(""), fmt.Errorf("device attributes error"))

	store := afero.NewMemMapFs()

	files := map[string]string{
		"/device-identity.d/key1":    "id1=value1",
		"/device-identity.d/key2":    "id2=value2",
		"/device-attributes.d/attr1": "attr1=value",
		"/device-attributes.d/attr2": "attr2=value",
	}

	for k, v := range files {
		err := afero.WriteFile(store, k, []byte(v), 0700)
		assert.NoError(t, err)
	}

	firmwareMetadata, err := NewFirmwareMetadata(metadataPath, store, clm)

	assert.EqualError(t, err, "device attributes error")
	assert.Equal(t, ((*FirmwareMetadata)(nil)), firmwareMetadata)

	clm.AssertExpectations(t)
}

func TestCheckSupportedHardware(t *testing.T) {
	testCases := []struct {
		name        string
		hardware    string
		expectedErr error
	}{
		{
			"WithNoHardwareOnFirmwareMetadata",
			"",
			nil,
		},

		{
			"WithMatch",
			"hardware2-revB",
			nil,
		},

		{
			"WithNoMatch",
			"hardware-value",
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

func TestCheckSupportedHardwareWithAnyString(t *testing.T) {
	fm := &FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "hardware1-revA",
		Version:          "version-value",
	}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return TestObject{} },
	})
	defer mode.Unregister()

	um, err := NewUpdateMetadata([]byte(ValidJSONMetadataWithSupportedHardwareAny))
	assert.NoError(t, err)

	err = fm.CheckSupportedHardware(um)
	assert.NoError(t, err)
}

func TestCheckSupportedHardwareWithUnknownFormat(t *testing.T) {
	fm := &FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "hardware1-revA",
		Version:          "version-value",
	}

	mode := installmodes.RegisterInstallMode(installmodes.InstallMode{
		Name:              "test",
		CheckRequirements: func() error { return nil },
		GetObject:         func() interface{} { return TestObject{} },
	})
	defer mode.Unregister()

	um, err := NewUpdateMetadata([]byte(ValidJSONMetadataWithUnknownSupportedHardwareFormat))
	assert.NoError(t, err)

	err = fm.CheckSupportedHardware(um)
	assert.EqualError(t, err, "unknown supported hardware format in the update metadata")
}
