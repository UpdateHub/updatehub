/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"io"
	"io/ioutil"
	"time"

	"github.com/go-ini/ini"
)

const (
	defaultPollingInterval = 60 * 60 // one hour (in seconds)
)

type Settings struct {
	PollingSettings  `ini:"Polling"`
	StorageSettings  `ini:"Storage"`
	UpdateSettings   `ini:"Update"`
	NetworkSettings  `ini:"Network"`
	FirmwareSettings `ini:"Firmware"`
}

type PersistentSettings struct {
	PersistentPollingSettings `ini:"Polling"`
}

type PollingSettings struct {
	PollingInterval           time.Duration `ini:"Interval,omitempty"`
	PollingEnabled            bool          `ini:"Enabled,omitempty"`
	PersistentPollingSettings `ini:"Polling"`
}

type PersistentPollingSettings struct {
	LastPoll             time.Time     `ini:"LastPoll"`
	FirstPoll            time.Time     `ini:"FirstPoll"`
	ExtraPollingInterval time.Duration `ini:"ExtraInterval"`
	PollingRetries       int           `ini:"Retries"`
}

type StorageSettings struct {
	ReadOnly bool `ini:"ReadOnly"`
}

type UpdateSettings struct {
	DownloadDir               string   `ini:"DownloadDir"`
	AutoDownloadWhenAvailable bool     `ini:"AutoDownloadWhenAvailable"`
	AutoInstallAfterDownload  bool     `ini:"AutoInstallAfterDownload"`
	AutoRebootAfterInstall    bool     `ini:"AutoRebootAfterInstall"`
	SupportedInstallModes     []string `ini:"SupportedInstallModes"`
}

type NetworkSettings struct {
	DisableHTTPS  bool   `ini:"DisableHttps"`
	ServerAddress string `ini:"UpdateHubServerAddress"`
}

type FirmwareSettings struct {
	FirmwareMetadataPath string `ini:"MetadataPath"`
}

func init() {
	ini.PrettyFormat = false
}

func LoadSettings(r io.Reader) (*Settings, error) {
	cfg, err := ini.Load(ioutil.NopCloser(r))
	if err != nil || cfg == nil {
		return nil, err
	}

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
	}

	err = cfg.MapTo(s)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func SaveSettings(s *Settings, w io.Writer) error {
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
