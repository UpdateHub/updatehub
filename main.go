package main

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/imdario/mergo"
	"github.com/spf13/afero"

	"code.ossystems.com.br/updatehub/agent/client"
	_ "code.ossystems.com.br/updatehub/agent/installmodes/copy"
)

var (
	// The system settings are the settings configured in the client-side and will be read-only
	systemSettingsPath = "/etc/easyfota-agent.conf"
	// The runtime settings are the settings that may can change during the execution of EasyFota
	// These settings are persisted to keep the behaviour across of device's reboot
	runtimeSettingsPath = "/var/lib/easyfota-agent.conf"
)

func main() {
	settings, err := combineSettingsFromFiles()
	if err != nil {
		log.Errorln(err)
		os.Exit(1)
	}

	fota := &EasyFota{
		state:        NewPollState(),
		pollInterval: 5,
		api:          client.NewApiClient("localhost:8080"),
		updater:      client.NewUpdateClient(),
		timeStep:     time.Minute,
		settings:     settings,
		store:        afero.NewOsFs(),
	}

	fota.Controller = fota

	fota.MainLoop()
}

func combineSettingsFromFiles() (*Settings, error) {
	files := []string{systemSettingsPath, runtimeSettingsPath}
	settings := []*Settings{}

	for _, name := range files {
		file, err := os.Open(name)
		if err != nil {
			return nil, err
		}

		s, err := LoadSettings(file)
		if err != nil {
			return nil, err
		}

		settings = append(settings, s)
	}

	err := mergo.Merge(settings[0], settings[1])
	if err != nil {
		return nil, err
	}

	return settings[0], nil
}
