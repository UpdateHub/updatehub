/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"fmt"
	"math/rand"
	"path"
	"sync"
	"time"

	"github.com/OSSystems/pkg/log"
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
		err := fmt.Errorf("update metadata must have 1 or 2 objects. Found %d", len(um.Objects))
		log.Error(err)
		return 0, err
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
		err = fmt.Errorf("sha256sum's don't match. Expected: %s / Calculated: %s", expectedSha256sum, calculatedSha256sum)
		log.Error(err)
		return err
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
	TimeStep                time.Duration
	API                     *client.ApiClient
	Updater                 client.Updater
	Reporter                client.Reporter
	lastInstalledPackageUID string
	ActiveInactiveBackend   activeinactive.Interface
	lastReportedState       string

	InstallIfDifferentBackend installifdifferent.Interface
	Sha256Checker
	utils.Rebooter
	state      State
	stateMutex sync.Mutex
}

func NewUpdateHub(gitversion string, buildtime string, fs afero.Fs, fm metadata.FirmwareMetadata, initialState State, settings *Settings) *UpdateHub {
	uh := &UpdateHub{
		ActiveInactiveBackend:     &activeinactive.DefaultImpl{CmdLineExecuter: &utils.CmdLine{}},
		Version:                   gitversion,
		BuildTime:                 buildtime,
		state:                     initialState,
		Updater:                   client.NewUpdateClient(),
		TimeStep:                  time.Minute,
		Store:                     fs,
		FirmwareMetadata:          fm,
		Settings:                  settings,
		Reporter:                  client.NewReportClient(),
		Sha256Checker:             &Sha256CheckerImpl{},
		InstallIfDifferentBackend: &installifdifferent.DefaultImpl{FileSystemBackend: fs},
		CopyBackend:               copy.ExtendedIO{},
		Rebooter:                  &utils.RebooterImpl{},
	}

	return uh
}

func (uh *UpdateHub) Cancel(nextState State) {
	uh.state.Cancel(true, nextState)
}

func (uh *UpdateHub) GetState() State {
	return uh.state
}

func (uh *UpdateHub) SetState(state State) {
	uh.stateMutex.Lock()
	defer uh.stateMutex.Unlock()

	uh.state = state
}

func (uh *UpdateHub) ProcessCurrentState() State {
	uh.stateMutex.Lock()
	defer uh.stateMutex.Unlock()

	uh.ReportCurrentState()

	state, cancel := uh.state.Handle(uh)

	cs, ok := uh.state.(*CancellableState)
	if cancel && ok {
		uh.state = cs.NextState()
	} else {
		uh.state = state
	}

	return uh.state
}

type Controller interface {
	ProbeUpdate(int) (*metadata.UpdateMetadata, time.Duration)
	DownloadUpdate(*metadata.UpdateMetadata, <-chan bool, chan<- int) error
	InstallUpdate(*metadata.UpdateMetadata, chan<- int) error
}

func (uh *UpdateHub) ProbeUpdate(retries int) (*metadata.UpdateMetadata, time.Duration) {
	var data struct {
		Retries int `json:"retries"`
		metadata.FirmwareMetadata
	}

	data.FirmwareMetadata = uh.FirmwareMetadata
	data.Retries = retries

	updateMetadataPath := path.Join(uh.Settings.DownloadDir, metadata.UpdateMetadataFilename)

	updateMetadata, extraPoll, err := uh.Updater.ProbeUpdate(uh.API.Request(), client.UpgradesEndpoint, data)
	if err != nil {
		uh.Store.Remove(updateMetadataPath)
		return nil, -1
	}

	if updateMetadata == nil || updateMetadata.(*metadata.UpdateMetadata) == nil {
		uh.Store.Remove(updateMetadataPath)
		return nil, extraPoll
	}

	um := updateMetadata.(*metadata.UpdateMetadata)
	afero.WriteFile(uh.Store, updateMetadataPath, um.RawBytes, 0644)

	return um, extraPoll
}

// it is recommended to use a buffered channel for "progressChan" to ensure no progress event is lost
func (uh *UpdateHub) DownloadUpdate(updateMetadata *metadata.UpdateMetadata, cancel <-chan bool, progressChan chan<- int) error {
	indexToInstall, err := GetIndexOfObjectToBeInstalled(uh.ActiveInactiveBackend, updateMetadata)
	if err != nil {
		return err
	}

	packageUID := updateMetadata.PackageUID()

	log.Info(fmt.Sprintf("downloading update. (package-uid: %s)", packageUID))

	progress := 0
	for _, obj := range updateMetadata.Objects[indexToInstall] {
		objectUID := obj.GetObjectMetadata().Sha256sum
		objectPath := path.Join(uh.Settings.DownloadDir, objectUID)

		sha256sum, err := utils.FileSha256sum(uh.Store, objectPath)
		if err == nil && sha256sum == objectUID {
			log.Warn(fmt.Sprintf("objectUID '%s' already downloaded", objectUID))
			continue
		}

		log.Info("downloading object: ", objectUID)

		uri := "/products"
		uri = path.Join(uri, uh.FirmwareMetadata.ProductUID)
		uri = path.Join(uri, "packages")
		uri = path.Join(uri, packageUID)
		uri = path.Join(uri, "objects")
		uri = path.Join(uri, objectUID)

		wr, err := uh.Store.Create(objectPath)
		if err != nil {
			return err
		}
		defer wr.Close()

		log.Debug("route: ", uri)
		rd, _, err := uh.Updater.DownloadUpdate(uh.API.Request(), uri)
		if err != nil {
			return err
		}
		defer rd.Close()

		_, err = uh.CopyBackend.Copy(wr, rd, 30*time.Second, cancel, utils.ChunkSize, 0, -1, false)
		if err != nil {
			return err
		}

		log.Info("object ", objectUID, " downloaded successfully")

		step := 100 / len(updateMetadata.Objects[indexToInstall])
		progress += step

		// "non-blocking" write to channel
		select {
		case progressChan <- progress:
		default:
		}
	}

	log.Info("update downloaded successfully")

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

	log.Info(fmt.Sprintf("installing update. (package-uid: %s)", updateMetadata.PackageUID()))

	indexToInstall, err := GetIndexOfObjectToBeInstalled(uh.ActiveInactiveBackend, updateMetadata)
	if err != nil {
		return err
	}

	progress := 0

	for _, obj := range updateMetadata.Objects[indexToInstall] {
		err := uh.CheckDownloadedObjectSha256sum(uh.Store, uh.Settings.DownloadDir, obj.GetObjectMetadata().Sha256sum)
		if err != nil {
			return err
		}

		log.Info(fmt.Sprintf("installing object: %s (mode: %s)", obj.GetObjectMetadata().Sha256sum, obj.GetObjectMetadata().Mode))

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

		if install {
			log.Info("object ", obj.GetObjectMetadata().Sha256sum, " installed successfully")
		} else {
			log.Info("object ", obj.GetObjectMetadata().Sha256sum, " is already installed (satisfied the 'install-if-different' field)")
		}

		step := 100 / len(updateMetadata.Objects[indexToInstall])
		progress += step

		// "non-blocking" write to channel
		select {
		case progressChan <- progress:
		default:
		}
	}

	// 2 objects means that ActiveInactive is enabled, so we need
	// to set the new active object
	if len(updateMetadata.Objects) == 2 {
		err := uh.ActiveInactiveBackend.SetActive(indexToInstall)
		if err != nil {
			return err
		}

		log.Info("ActiveInactive activated: ", indexToInstall)
	}

	log.Info("update installed successfully")

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
	if rs, ok := uh.state.(ReportableState); ok {
		stateString := StateToString(uh.state.ID())

		if uh.lastReportedState == stateString {
			return nil
		}

		packageUID := ""
		if rs.UpdateMetadata() != nil {
			packageUID = rs.UpdateMetadata().PackageUID()
		}

		errorMessage := ""
		if es, ok := uh.state.(*ErrorState); ok {
			errorMessage = es.cause.Cause().Error()
		}

		err := uh.Reporter.ReportState(uh.API.Request(), packageUID, stateString, errorMessage, uh.FirmwareMetadata)
		if err != nil {
			return err
		}

		uh.lastReportedState = stateString
	}

	return nil
}

// StartPolling starts the polling process
func (uh *UpdateHub) StartPolling() {
	uh.stateMutex.Lock()
	defer uh.stateMutex.Unlock()

	now := time.Now()
	now = time.Unix(now.Unix(), 0)

	poll := NewPollState(uh.Settings.PollingInterval)

	uh.state = poll

	timeZero := (time.Time{}).UTC()

	if uh.Settings.FirstPoll == timeZero {
		// Apply an offset in first poll
		uh.Settings.FirstPoll = now.Add(time.Duration(rand.Int63n(int64(uh.Settings.PollingInterval))))
	} else if uh.Settings.LastPoll == timeZero && now.After(uh.Settings.FirstPoll) {
		// it never did a poll before
		uh.state = NewUpdateProbeState()
	} else if uh.Settings.LastPoll.Add(uh.Settings.PollingInterval).Before(now) {
		// pending regular interval
		uh.state = NewUpdateProbeState()
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

		log.Info("next poll is expected at: ", nextPoll)
	}
}
