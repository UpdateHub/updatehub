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
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/imdario/mergo"
	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/activeinactive"
	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/copy"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/utils"
)

type UpdateHub struct {
	Controller
	CopyBackend copy.Interface `json:"-"`

	settings                *Settings
	Store                   afero.Fs
	FirmwareMetadata        metadata.FirmwareMetadata
	State                   State
	TimeStep                time.Duration
	API                     *client.ApiClient
	Updater                 client.Updater
	Reporter                client.Reporter
	lastInstalledPackageUID string
	activeInactiveBackend   activeinactive.Interface
	SystemSettingsPath      string
	RuntimeSettingsPath     string
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

	data.FirmwareMetadata = uh.FirmwareMetadata
	data.Retries = retries

	updateMetadata, extraPoll, err := uh.Updater.CheckUpdate(uh.API.Request(), client.UpgradesEndpoint, data)
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
		uri = path.Join(uri, uh.FirmwareMetadata.ProductUID)
		uri = path.Join(uri, packageUID)
		uri = path.Join(uri, objectUID)

		wr, err := uh.Store.Create(path.Join(uh.settings.DownloadDir, objectUID))
		if err != nil {
			return err
		}
		defer wr.Close()

		rd, _, err := uh.Updater.FetchUpdate(uh.API.Request(), uri)
		if err != nil {
			return err
		}
		defer rd.Close()

		_, err = uh.CopyBackend.Copy(wr, rd, 30*time.Second, cancel, utils.ChunkSize, 0, -1, false)
		if err != nil {
			return err
		}
	}

	return nil
}

func (uh *UpdateHub) ReportCurrentState() error {
	if rs, ok := uh.State.(ReportableState); ok {
		packageUID := rs.UpdateMetadata().PackageUID()
		err := uh.Reporter.ReportState(uh.API.Request(), packageUID, StateToString(uh.State.ID()))
		if err != nil {
			return err
		}
	}

	return nil
}

// LoadSettings loads system and runtime settings
func (uh *UpdateHub) LoadSettings() error {
	files := []string{uh.SystemSettingsPath, uh.RuntimeSettingsPath}
	settings := []*Settings{}

	var file io.ReadCloser
	var err error

	for _, name := range files {
		file, err = uh.Store.Open(name)
		if err != nil {
			if os.IsNotExist(err) {
				file = ioutil.NopCloser(bytes.NewReader(nil))
			} else {
				return err
			}
		}

		s, err := LoadSettings(file)
		if err != nil {
			return err
		}

		settings = append(settings, s)
	}

	err = mergo.Merge(settings[0], settings[1])
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

	uh.State = poll

	timeZero := (time.Time{}).UTC()

	if uh.settings.FirstPoll == timeZero {
		// Apply an offset in first poll
		uh.settings.FirstPoll = now.Add(time.Duration(rand.Int63n(int64(uh.settings.PollingInterval))))
	} else if uh.settings.LastPoll == timeZero && now.After(uh.settings.FirstPoll) {
		// it never did a poll before
		uh.State = NewUpdateCheckState()
	} else if uh.settings.LastPoll.Add(uh.settings.PollingInterval).Before(now) {
		// pending regular interval
		uh.State = NewUpdateCheckState()
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
			poll.ticksCount = (int64(uh.settings.PollingInterval) - nextPoll.Sub(now).Nanoseconds()) / int64(uh.TimeStep)
		}
	}
}
