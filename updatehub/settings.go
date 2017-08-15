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
	"io"
	"io/ioutil"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/go-ini/ini"
)

const (
	defaultPollingInterval = time.Hour
	defaultServerAddress   = "api.updatehub.io"
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
		},
	},

	StorageSettings: StorageSettings{
		ReadOnly:            false,
		RuntimeSettingsPath: "/var/lib/updatehub.conf",
	},

	UpdateSettings: UpdateSettings{
		DownloadDir:           "/tmp",
		SupportedInstallModes: []string{"dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs"},
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
}

type StorageSettings struct {
	ReadOnly            bool   `ini:"ReadOnly" json:"read-only"`
	RuntimeSettingsPath string `ini:"RuntimeSettingsPath" json:"runtime-settings-path"`
}

type UpdateSettings struct {
	DownloadDir           string   `ini:"DownloadDir" json:"download-dir"`
	SupportedInstallModes []string `ini:"SupportedInstallModes" json:"supported-install-modes"`
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

	return &s, nil
}

func SaveSettings(s *Settings, w io.Writer) error {
	log.Debug("\n", s.ToString())

	ps := &PersistentSettings{
		PersistentPollingSettings: s.PollingSettings.PersistentPollingSettings,
	}

	cfg := ini.Empty()

	err := ini.ReflectFrom(cfg, ps)
	if err != nil {
		return err
	}

	_, err = cfg.WriteTo(w)
	if err != nil {
		return err
	}

	return nil
}
