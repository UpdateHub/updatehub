package main

import (
	"io"
	"io/ioutil"

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

type PollingSettings struct {
	PollingInterval      int  `ini:"Interval,omitempty"`
	PollingEnabled       bool `ini:"Enabled,omitempty"`
	LastPoll             int  `ini:"LastPoll"`
	FirstPoll            int  `ini:"FirstPoll"`
	ExtraPollingInterval int  `ini:"ExtraInterval"`
	PollingRetries       int  `ini:"Retries"`
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
	ServerAddress string `ini:"EasyFotaServerAddress"`
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
			PollingInterval:      defaultPollingInterval,
			PollingEnabled:       true,
			LastPoll:             0,
			FirstPoll:            0,
			ExtraPollingInterval: 0,
			PollingRetries:       0,
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
	cfg := ini.Empty()

	err := ini.ReflectFrom(cfg, s)
	if err != nil {
		return err
	}

	_, err = cfg.WriteTo(w)
	if err != nil {
		return err
	}

	return nil
}
