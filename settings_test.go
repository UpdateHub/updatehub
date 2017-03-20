/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

const customSettings = `
[Polling]
Interval=1
Enabled=false
LastPoll=2
FirstPoll=3
ExtraInterval=4
Retries=5

[Storage]
ReadOnly=true

[Update]
DownloadDir=/tmp/download
AutoDownloadWhenAvailable=false
AutoInstallAfterDownload=false
AutoRebootAfterInstall=false
SupportedInstallModes=mode1,mode2

[Network]
DisableHttps=true
UpdateHubServerAddress=localhost

[Firmware]
MetadataPath=/tmp/metadata
`

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
						LastPoll:             0,
						FirstPoll:            0,
						ExtraPollingInterval: 0,
						PollingRetries:       0,
					},
				},

				StorageSettings: StorageSettings{
					ReadOnly: false,
				},

				UpdateSettings: UpdateSettings{
					DownloadDir:               "/tmp",
					AutoDownloadWhenAvailable: true,
					AutoInstallAfterDownload:  true,
					AutoRebootAfterInstall:    true,
					SupportedInstallModes:     []string{"dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs"},
				},

				NetworkSettings: NetworkSettings{
					DisableHTTPS:  false,
					ServerAddress: "",
				},

				FirmwareSettings: FirmwareSettings{
					FirmwareMetadataPath: "",
				},
			},
		},

		{
			"CustomValues",
			customSettings,
			&Settings{
				PollingSettings: PollingSettings{
					PollingInterval: 1,
					PollingEnabled:  false,
					PersistentPollingSettings: PersistentPollingSettings{
						LastPoll:             2,
						FirstPoll:            3,
						ExtraPollingInterval: 4,
						PollingRetries:       5,
					},
				},

				StorageSettings: StorageSettings{
					ReadOnly: true,
				},

				UpdateSettings: UpdateSettings{
					DownloadDir:               "/tmp/download",
					AutoDownloadWhenAvailable: false,
					AutoInstallAfterDownload:  false,
					AutoRebootAfterInstall:    false,
					SupportedInstallModes:     []string{"mode1", "mode2"},
				},

				NetworkSettings: NetworkSettings{
					DisableHTTPS:  true,
					ServerAddress: "localhost",
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

func TestSaveSettings(t *testing.T) {
	expectedSettings, err := LoadSettings(bytes.NewReader([]byte("")))
	assert.NoError(t, err)
	assert.NotNil(t, expectedSettings)

	var buf bytes.Buffer
	err = SaveSettings(expectedSettings, &buf)
	assert.NoError(t, err)

	s, err := LoadSettings(bytes.NewReader(buf.Bytes()))
	assert.NoError(t, err)
	assert.NotNil(t, s)

	assert.Equal(t, expectedSettings, s)
}
