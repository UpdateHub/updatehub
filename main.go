/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/imdario/mergo"
	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/client"
	_ "github.com/UpdateHub/updatehub/installmodes/copy"
)

var (
	// The system settings are the settings configured in the client-side and will be read-only
	systemSettingsPath = "/etc/updatehub-agent.conf"
	// The runtime settings are the settings that may can change during the execution of UpdateHub
	// These settings are persisted to keep the behaviour across of device's reboot
	runtimeSettingsPath = "/var/lib/updatehub-agent.conf"
)

func main() {
	settings, err := combineSettingsFromFiles()
	if err != nil {
		log.Errorln(err)
		os.Exit(1)
	}

	uh := &UpdateHub{
		state:    NewPollState(),
		api:      client.NewApiClient("localhost:8080"),
		updater:  client.NewUpdateClient(),
		timeStep: time.Minute,
		settings: settings,
		store:    afero.NewOsFs(),
	}

	uh.Controller = uh

	uh.MainLoop()
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
