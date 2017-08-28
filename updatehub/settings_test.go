/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

const (
	customSettings = `
[Polling]
Interval=2h
Enabled=false
LastPoll=2017-01-01T00:00:00Z
FirstPoll=2017-02-02T00:00:00Z
ExtraInterval=4
Retries=5

[Storage]
ReadOnly=true

[Update]
DownloadDir=/tmp/download
SupportedInstallModes=mode1,mode2

[Network]
ServerAddress=http://localhost

[Firmware]
MetadataPath=/tmp/metadata
`

	customSettingsWithInvalidPollingInterval = `
[Polling]
Interval=20s
Enabled=false
LastPoll=2017-01-01T00:00:00Z
FirstPoll=2017-02-02T00:00:00Z
ExtraInterval=4
Retries=5

[Storage]
ReadOnly=true

[Update]
DownloadDir=/tmp/download
SupportedInstallModes=mode1,mode2

[Network]
ServerAddress=http://localhost

[Firmware]
MetadataPath=/tmp/metadata
`
)

func TestToString(t *testing.T) {
	s := &Settings{
		PollingSettings: PollingSettings{
			PollingInterval: defaultPollingInterval,
			PollingEnabled:  true,
			PersistentPollingSettings: PersistentPollingSettings{
				LastPoll:             (time.Time{}).UTC(),
				FirstPoll:            (time.Time{}).UTC(),
				ExtraPollingInterval: 0,
				PollingRetries:       0,
			},
		},

		StorageSettings: StorageSettings{
			ReadOnly: false,
		},

		UpdateSettings: UpdateSettings{
			DownloadDir:           "/tmp",
			SupportedInstallModes: []string{"dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs"},
		},

		NetworkSettings: NetworkSettings{
			ServerAddress: "https://api.updatehub.io",
		},

		FirmwareSettings: FirmwareSettings{
			FirmwareMetadataPath: "",
		},
	}

	outputJSON, _ := json.MarshalIndent(s, "", "    ")
	expectedString := string(outputJSON)

	assert.Equal(t, expectedString, s.ToString())
}

func TestLoadSettingsDefaultValues(t *testing.T) {
	s, err := LoadSettings(bytes.NewReader([]byte("")))
	assert.NoError(t, err)

	assert.Equal(t, time.Hour, s.PollingSettings.PollingInterval)
	assert.Equal(t, true, s.PollingSettings.PollingEnabled)
	assert.Equal(t, (time.Time{}).UTC(), s.PollingSettings.PersistentPollingSettings.LastPoll)
	assert.Equal(t, (time.Time{}).UTC(), s.PollingSettings.PersistentPollingSettings.FirstPoll)
	assert.Equal(t, time.Duration(0), s.PollingSettings.PersistentPollingSettings.ExtraPollingInterval)
	assert.Equal(t, 0, s.PollingSettings.PersistentPollingSettings.PollingRetries)

	assert.Equal(t, false, s.StorageSettings.ReadOnly)

	assert.Equal(t, "/tmp", s.UpdateSettings.DownloadDir)
	assert.Equal(t, []string{"dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs"}, s.UpdateSettings.SupportedInstallModes)
	assert.Equal(t, -1, s.UpdateSettings.PersistentUpdateSettings.UpgradeToInstallation)

	assert.Equal(t, "https://api.updatehub.io", s.NetworkSettings.ServerAddress)

	assert.Equal(t, "/usr/share/updatehub", s.FirmwareSettings.FirmwareMetadataPath)
}

func TestLoadSettings(t *testing.T) {
	testCases := []struct {
		name             string
		data             string
		expectedSettings *Settings
	}{
		{
			"DefaultValues",
			"",
			&Settings{
				PollingSettings: PollingSettings{
					PollingInterval: defaultPollingInterval,
					PollingEnabled:  true,
					PersistentPollingSettings: PersistentPollingSettings{
						LastPoll:             (time.Time{}).UTC(),
						FirstPoll:            (time.Time{}).UTC(),
						ExtraPollingInterval: 0,
						PollingRetries:       0,
					},
				},

				StorageSettings: StorageSettings{
					ReadOnly:            false,
					RuntimeSettingsPath: "/var/lib/updatehub.conf",
				},

				UpdateSettings: UpdateSettings{
					DownloadDir:           "/tmp",
					SupportedInstallModes: []string{"dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs"},
					PersistentUpdateSettings: PersistentUpdateSettings{
						UpgradeToInstallation: -1,
					},
				},

				NetworkSettings: NetworkSettings{
					ServerAddress: "https://api.updatehub.io",
				},

				FirmwareSettings: FirmwareSettings{
					FirmwareMetadataPath: "/usr/share/updatehub",
				},
			},
		},

		{
			"CustomValues",
			customSettings,
			&Settings{
				PollingSettings: PollingSettings{
					PollingInterval: 2 * time.Hour,
					PollingEnabled:  false,
					PersistentPollingSettings: PersistentPollingSettings{
						LastPoll:             time.Date(2017, time.January, 1, 0, 0, 0, 0, time.UTC),
						FirstPoll:            time.Date(2017, time.February, 2, 0, 0, 0, 0, time.UTC),
						ExtraPollingInterval: 4,
						PollingRetries:       5,
					},
				},

				StorageSettings: StorageSettings{
					ReadOnly:            true,
					RuntimeSettingsPath: "/var/lib/updatehub.conf",
				},

				UpdateSettings: UpdateSettings{
					DownloadDir:           "/tmp/download",
					SupportedInstallModes: []string{"mode1", "mode2"},
					PersistentUpdateSettings: PersistentUpdateSettings{
						UpgradeToInstallation: -1,
					},
				},

				NetworkSettings: NetworkSettings{
					ServerAddress: "http://localhost",
				},

				FirmwareSettings: FirmwareSettings{
					FirmwareMetadataPath: "/tmp/metadata",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := LoadSettings(bytes.NewReader([]byte(tc.data)))
			assert.NoError(t, err)
			assert.NotNil(t, s)
			assert.Equal(t, tc.expectedSettings, s)
		})
	}
}

func TestLoadSettingsWithError(t *testing.T) {
	ds := DefaultSettings
	ds.PollingInterval = 20 * time.Second

	_, err := LoadSettings(bytes.NewReader([]byte(customSettingsWithInvalidPollingInterval)))
	assert.EqualError(t, err, "Settings invalid config: Polling interval can't be less than 1m0s")
}

func TestSaveSettings(t *testing.T) {
	fs := afero.NewMemMapFs()

	settings, err := LoadSettings(bytes.NewReader([]byte(customSettings)))
	assert.NoError(t, err)
	assert.NotNil(t, settings)

	err = settings.Save(fs)
	assert.NoError(t, err)

	data, err := afero.ReadFile(fs, "/var/lib/updatehub.conf")
	assert.NoError(t, err)

	expectedData := `[Polling]
LastPoll=2017-01-01T00:00:00Z
FirstPoll=2017-02-02T00:00:00Z
ExtraInterval=4
Retries=5
ProbeASAP=false

[Update]
UpgradeToInstallation=-1

`
	assert.Equal(t, expectedData, string(data))
}

func TestValidateValues(t *testing.T) {
	testCases := []struct {
		name          string
		settings      *Settings
		expectedError error
	}{
		{
			"SuccessfulValues",
			func() *Settings {
				s := DefaultSettings
				return &s
			}(),
			nil,
		},
		{
			"UnsanitizedAddress",
			func() *Settings {
				s := DefaultSettings
				s.ServerAddress = "api.updatehub.io"
				return &s
			}(),
			nil,
		},
		{
			"NegativeExtraPollingInterval",
			func() *Settings {
				s := DefaultSettings
				s.ExtraPollingInterval = -1
				return &s
			}(),
			fmt.Errorf("Extra polling interval can't be negative"),
		},
		{
			"PollingIntervalLessThanAMinute",
			func() *Settings {
				s := DefaultSettings
				s.PollingInterval = 30 * time.Second
				return &s
			}(),
			fmt.Errorf("Polling interval can't be less than 1m0s"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateValues(tc.settings)

			if tc.expectedError == nil {
				assert.Nil(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedError.Error())
			}
		})
	}
}
