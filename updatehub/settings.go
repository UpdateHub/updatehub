/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/go-ini/ini"
	"github.com/spf13/afero"
	"github.com/updatehub/updatehub/utils"
)

const (
	defaultPollingInterval = time.Hour
	defaultServerAddress   = "https://api.updatehub.io"
)

var DefaultSettings = Settings{
	PollingSettings: PollingSettings{
		PollingInterval: defaultPollingInterval,
		PollingEnabled:  true,
		PersistentPollingSettings: PersistentPollingSettings{
			LastPoll:             (time.Time{}).UTC(),
			FirstPoll:            (time.Time{}).UTC(),
			ExtraPollingInterval: 0,
			PollingRetries:       0,
			ProbeASAP:            false,
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
		ServerAddress: defaultServerAddress,
	},

	FirmwareSettings: FirmwareSettings{
		FirmwareMetadataPath: "/usr/share/updatehub",
	},
}

type Settings struct {
	PollingSettings  `ini:"Polling" json:"polling"`
	StorageSettings  `ini:"Storage" json:"storage"`
	UpdateSettings   `ini:"Update" json:"update"`
	NetworkSettings  `ini:"Network" json:"network"`
	FirmwareSettings `ini:"Firmware" json:"firmware"`
}

type PersistentSettings struct {
	PersistentPollingSettings `ini:"Polling"`
	PersistentUpdateSettings  `ini:"Update"`
}

type PollingSettings struct {
	PollingInterval           time.Duration `ini:"Interval,omitempty" json:"interval,omitempty"`
	PollingEnabled            bool          `ini:"Enabled,omitempty" json:"enabled,omitempty"`
	PersistentPollingSettings `ini:"Polling"`
}

type PersistentPollingSettings struct {
	LastPoll             time.Time     `ini:"LastPoll" json:"last-poll"`
	FirstPoll            time.Time     `ini:"FirstPoll" json:"first-poll"`
	ExtraPollingInterval time.Duration `ini:"ExtraInterval" json:"extra-interval"`
	PollingRetries       int           `ini:"Retries" json:"retries"`
	ProbeASAP            bool          `ini:"ProbeASAP" json:"probe-asap"`
}

type StorageSettings struct {
	ReadOnly            bool   `ini:"ReadOnly" json:"read-only"`
	RuntimeSettingsPath string `ini:"RuntimeSettingsPath" json:"runtime-settings-path"`
}

type UpdateSettings struct {
	DownloadDir              string   `ini:"DownloadDir" json:"download-dir"`
	SupportedInstallModes    []string `ini:"SupportedInstallModes" json:"supported-install-modes"`
	PersistentUpdateSettings `ini:"Update"`
}

type PersistentUpdateSettings struct {
	UpgradeToInstallation int `ini:"UpgradeToInstallation" json:"upgrade-to-installation"`
}

type NetworkSettings struct {
	ServerAddress string `ini:"ServerAddress" json:"server-address"`
}

type FirmwareSettings struct {
	FirmwareMetadataPath string `ini:"MetadataPath" json:"metadata-path"`
}

func init() {
	ini.PrettyFormat = false
}

func (s *Settings) ToString() string {
	outputJSON, _ := json.MarshalIndent(s, "", "    ")
	return string(outputJSON)
}

func (s *Settings) Save(fs afero.Fs) error {
	log.Debug("Saving: \n", s.ToString())

	file, err := fs.Create(s.RuntimeSettingsPath)
	if err != nil {
		return err
	}
	defer file.Close()

	ps := &PersistentSettings{
		PersistentPollingSettings: s.PollingSettings.PersistentPollingSettings,
		PersistentUpdateSettings:  s.UpdateSettings.PersistentUpdateSettings,
	}

	cfg := ini.Empty()

	err = ini.ReflectFrom(cfg, ps)
	if err != nil {
		return err
	}

	_, err = cfg.WriteTo(file)
	if err != nil {
		return err
	}

	return nil
}

func LoadSettings(r io.Reader) (*Settings, error) {
	cfg, err := ini.Load(ioutil.NopCloser(r))
	if err != nil || cfg == nil {
		return nil, err
	}

	s := DefaultSettings

	err = cfg.MapTo(&s)
	if err != nil {
		return nil, err
	}

	err = validateValues(&s)
	if err != nil {
		return nil, fmt.Errorf("Settings invalid config: %s", err)
	}

	return &s, nil
}

func validateValues(s *Settings) error {
	if s.PollingInterval < time.Minute {
		return fmt.Errorf("Polling interval can't be less than %s", time.Minute)
	}

	if s.ExtraPollingInterval < 0 {
		return fmt.Errorf("Extra polling interval can't be negative")
	}

	address, err := utils.SanitizeServerAddress(s.ServerAddress)
	if err != nil {
		return err
	}

	s.ServerAddress = address

	return nil
}
