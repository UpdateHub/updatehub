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
	"fmt"
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
	"github.com/UpdateHub/updatehub/installifdifferent"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/utils"
)

// GetIndexOfObjectToBeInstalled selects which object will be installed from the update metadata
func GetIndexOfObjectToBeInstalled(aii activeinactive.Interface, um *metadata.UpdateMetadata) (int, error) {
	if len(um.Objects) < 1 || len(um.Objects) > 2 {
		return 0, fmt.Errorf("update metadata must have 1 or 2 objects. Found %d", len(um.Objects))
	}

	// 2 objects means that ActiveInactive is enabled
	if len(um.Objects) == 2 {
		activeIndex, err := aii.Active()
		if err != nil {
			return 0, err
		}

		inactiveIndex := (activeIndex - 1) * -1

		return inactiveIndex, nil
	}

	return 0, nil
}

type Sha256Checker interface {
	CheckDownloadedObjectSha256sum(fsBackend afero.Fs, downloadDir string, expectedSha256sum string) error
}

type Sha256CheckerImpl struct {
}

func (s *Sha256CheckerImpl) CheckDownloadedObjectSha256sum(fsBackend afero.Fs, downloadDir string, expectedSha256sum string) error {
	calculatedSha256sum, err := utils.FileSha256sum(fsBackend, path.Join(downloadDir, expectedSha256sum))
	if err != nil {
		return err
	}

	if calculatedSha256sum != expectedSha256sum {
		return fmt.Errorf("sha256sum's don't match. Expected: %s / Calculated: %s", expectedSha256sum, calculatedSha256sum)
	}

	return nil
}

type UpdateHub struct {
	Controller
	CopyBackend copy.Interface `json:"-"`

	Version                 string
	BuildTime               string
	Settings                *Settings
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

	InstallIfDifferentBackend installifdifferent.Interface
	Sha256Checker
}

type Controller interface {
	CheckUpdate(int) (*metadata.UpdateMetadata, time.Duration)
	FetchUpdate(*metadata.UpdateMetadata, <-chan bool, chan<- int) error
	InstallUpdate(*metadata.UpdateMetadata, chan<- int) error
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

// it is recommended to use a buffered channel for "progressChan" to ensure no progress event is lost
func (uh *UpdateHub) FetchUpdate(updateMetadata *metadata.UpdateMetadata, cancel <-chan bool, progressChan chan<- int) error {
	indexToInstall, err := GetIndexOfObjectToBeInstalled(uh.activeInactiveBackend, updateMetadata)
	if err != nil {
		return err
	}

	packageUID := updateMetadata.PackageUID()

	progress := 0
	for _, obj := range updateMetadata.Objects[indexToInstall] {
		objectUID := obj.GetObjectMetadata().Sha256sum

		uri := "/"
		uri = path.Join(uri, uh.FirmwareMetadata.ProductUID)
		uri = path.Join(uri, packageUID)
		uri = path.Join(uri, objectUID)

		wr, err := uh.Store.Create(path.Join(uh.Settings.DownloadDir, objectUID))
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

		step := 100 / len(updateMetadata.Objects[indexToInstall])
		progress += step

		// "non-blocking" write to channel
		select {
		case progressChan <- progress:
		default:
		}
	}

	if progress < 100 {
		// "non-blocking" write to channel
		select {
		case progressChan <- 100:
		default:
		}
	}

	return nil
}

// it is recommended to use a buffered channel for "progressChan" to ensure no progress event is lost
func (uh *UpdateHub) InstallUpdate(updateMetadata *metadata.UpdateMetadata, progressChan chan<- int) error {
	err := uh.FirmwareMetadata.CheckSupportedHardware(updateMetadata)
	if err != nil {
		return err
	}

	indexToInstall, err := GetIndexOfObjectToBeInstalled(uh.activeInactiveBackend, updateMetadata)
	if err != nil {
		return err
	}

	progress := 0

	for _, obj := range updateMetadata.Objects[indexToInstall] {
		err := uh.CheckDownloadedObjectSha256sum(uh.Store, uh.Settings.DownloadDir, obj.GetObjectMetadata().Sha256sum)
		if err != nil {
			return err
		}

		err = obj.Setup()
		if err != nil {
			return err
		}

		errorList := []error{}

		install, err := uh.InstallIfDifferentBackend.Proceed(obj)
		if err != nil {
			errorList = append(errorList, err)
		}

		if install {
			err = obj.Install(uh.Settings.DownloadDir)
			if err != nil {
				errorList = append(errorList, err)
			}
		}

		err = obj.Cleanup()
		if err != nil {
			errorList = append(errorList, err)
		}

		if len(errorList) > 0 {
			return utils.MergeErrorList(errorList)
		}

		// 2 objects means that ActiveInactive is enabled, so we need
		// to set the new active object
		if len(updateMetadata.Objects) == 2 {
			err := uh.activeInactiveBackend.SetActive(indexToInstall)
			if err != nil {
				return err
			}
		}

		step := 100 / len(updateMetadata.Objects[indexToInstall])
		progress += step

		// "non-blocking" write to channel
		select {
		case progressChan <- progress:
		default:
		}
	}

	if progress < 100 {
		// "non-blocking" write to channel
		select {
		case progressChan <- 100:
		default:
		}
	}

	return nil
}

func (uh *UpdateHub) ReportCurrentState() error {
	if rs, ok := uh.State.(ReportableState); ok {
		packageUID := ""
		if rs.UpdateMetadata() != nil {
			packageUID = rs.UpdateMetadata().PackageUID()
		}
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

	uh.Settings = settings[0]

	return nil
}

// StartPolling starts the polling process
func (uh *UpdateHub) StartPolling() {
	now := time.Now()
	now = time.Unix(now.Unix(), 0)

	poll := NewPollState(uh.Settings.PollingInterval)

	uh.State = poll

	timeZero := (time.Time{}).UTC()

	if uh.Settings.FirstPoll == timeZero {
		// Apply an offset in first poll
		uh.Settings.FirstPoll = now.Add(time.Duration(rand.Int63n(int64(uh.Settings.PollingInterval))))
	} else if uh.Settings.LastPoll == timeZero && now.After(uh.Settings.FirstPoll) {
		// it never did a poll before
		uh.State = NewUpdateCheckState()
	} else if uh.Settings.LastPoll.Add(uh.Settings.PollingInterval).Before(now) {
		// pending regular interval
		uh.State = NewUpdateCheckState()
	} else {
		nextPoll := time.Unix(uh.Settings.FirstPoll.Unix(), 0)
		for nextPoll.Before(now) {
			nextPoll = nextPoll.Add(uh.Settings.PollingInterval)
		}

		if uh.Settings.ExtraPollingInterval > 0 {
			extraPoll := uh.Settings.LastPoll.Add(uh.Settings.ExtraPollingInterval)

			if extraPoll.Before(nextPoll) {
				// Set polling interval to the pending extra poll interval
				poll.interval = uh.Settings.ExtraPollingInterval
			}
		} else {
			poll.ticksCount = (int64(uh.Settings.PollingInterval) - nextPoll.Sub(now).Nanoseconds()) / int64(uh.TimeStep)
		}
	}
}
