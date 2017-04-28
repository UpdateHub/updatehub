/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"math/rand"
	"path"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/imdario/mergo"
	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/activeinactive"
	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/utils"
)

type UpdateHub struct {
	Controller
	utils.Copier

	settings                *Settings
	store                   afero.Fs
	firmwareMetadata        metadata.FirmwareMetadata
	state                   State
	timeStep                time.Duration
	api                     *client.ApiClient
	updater                 client.Updater
	reporter                client.Reporter
	logger                  *logrus.Logger
	lastInstalledPackageUID string
	activeInactiveBackend   activeinactive.Interface
}

type Controller interface {
	CheckUpdate(int) (*metadata.UpdateMetadata, time.Duration)
	FetchUpdate(*metadata.UpdateMetadata, <-chan bool) error
	ReportCurrentState() error
}

func (uh *UpdateHub) CheckUpdate(retries int) (*metadata.UpdateMetadata, time.Duration) {
	var data struct {
		Retries int `json:"retries"`
		metadata.FirmwareMetadata
	}

	data.FirmwareMetadata = uh.firmwareMetadata
	data.Retries = retries

	updateMetadata, extraPoll, err := uh.updater.CheckUpdate(uh.api.Request(), client.UpgradesEndpoint, data)
	if err != nil || updateMetadata == nil {
		return nil, -1
	}

	return updateMetadata.(*metadata.UpdateMetadata), extraPoll
}

func (uh *UpdateHub) FetchUpdate(updateMetadata *metadata.UpdateMetadata, cancel <-chan bool) error {
	indexToInstall, err := GetIndexOfObjectToBeInstalled(uh.activeInactiveBackend, updateMetadata)
	if err != nil {
		return err
	}

	packageUID := updateMetadata.PackageUID()

	for _, obj := range updateMetadata.Objects[indexToInstall] {
		objectUID := obj.GetObjectMetadata().Sha256sum

		uri := "/"
		uri = path.Join(uri, uh.firmwareMetadata.ProductUID)
		uri = path.Join(uri, packageUID)
		uri = path.Join(uri, objectUID)

		wr, err := uh.store.Create(path.Join(uh.settings.DownloadDir, objectUID))
		if err != nil {
			return err
		}
		defer wr.Close()

		rd, _, err := uh.updater.FetchUpdate(uh.api.Request(), uri)
		if err != nil {
			return err
		}
		defer rd.Close()

		_, err = uh.Copy(wr, rd, 30*time.Second, cancel, utils.ChunkSize, 0, -1, false)
		if err != nil {
			return err
		}
	}

	return nil
}

func (uh *UpdateHub) ReportCurrentState() error {
	if rs, ok := uh.state.(ReportableState); ok {
		packageUID := rs.UpdateMetadata().PackageUID()
		err := uh.reporter.ReportState(uh.api.Request(), packageUID, StateToString(uh.state.ID()))
		if err != nil {
			return err
		}
	}

	return nil
}

// LoadSettings loads system and runtime settings
func (uh *UpdateHub) LoadSettings() error {
	files := []string{systemSettingsPath, runtimeSettingsPath}
	settings := []*Settings{}

	for _, name := range files {
		file, err := uh.store.Open(name)
		if err != nil {
			return err
		}

		s, err := LoadSettings(file)
		if err != nil {
			return err
		}

		settings = append(settings, s)
	}

	err := mergo.Merge(settings[0], settings[1])
	if err != nil {
		return err
	}

	uh.settings = settings[0]

	return nil
}

// StartPolling starts the polling process
func (uh *UpdateHub) StartPolling() {
	now := time.Now()
	now = time.Unix(now.Unix(), 0)

	poll := NewPollState(uh)

	uh.state = poll

	timeZero := (time.Time{}).UTC()

	if uh.settings.FirstPoll == timeZero {
		// Apply an offset in first poll
		uh.settings.FirstPoll = now.Add(time.Duration(rand.Int63n(int64(uh.settings.PollingInterval))))
	} else if uh.settings.LastPoll == timeZero && now.After(uh.settings.FirstPoll) {
		// it never did a poll before
		uh.state = NewUpdateCheckState()
	} else if uh.settings.LastPoll.Add(uh.settings.PollingInterval).Before(now) {
		// pending regular interval
		uh.state = NewUpdateCheckState()
	} else {
		nextPoll := time.Unix(uh.settings.FirstPoll.Unix(), 0)
		for nextPoll.Before(now) {
			nextPoll = nextPoll.Add(uh.settings.PollingInterval)
		}

		if uh.settings.ExtraPollingInterval > 0 {
			extraPoll := uh.settings.LastPoll.Add(uh.settings.ExtraPollingInterval)

			if extraPoll.Before(nextPoll) {
				// Set polling interval to the pending extra poll interval
				poll.interval = uh.settings.ExtraPollingInterval
			}
		} else {
			poll.ticksCount = (int64(uh.settings.PollingInterval) - nextPoll.Sub(now).Nanoseconds()) / int64(uh.timeStep)
		}
	}
}
