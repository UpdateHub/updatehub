/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"crypto/rsa"
	"fmt"
	"math/rand"
	"os"
	"path"
	"sync"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/anacrolix/missinggo/httptoo"
	"github.com/pkg/errors"
	"github.com/spf13/afero"

	"github.com/updatehub/updatehub/activeinactive"
	"github.com/updatehub/updatehub/client"
	"github.com/updatehub/updatehub/copy"
	"github.com/updatehub/updatehub/installifdifferent"
	"github.com/updatehub/updatehub/metadata"
	"github.com/updatehub/updatehub/utils"
)

var ErrSha256sum = errors.New("sha256sum's don't match")

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
	CheckDownloadedObjectSha256sum(fsBackend afero.Fs, downloadDir string, expectedSha256sum string) (bool, error)
}

type Sha256CheckerImpl struct {
}

func (s *Sha256CheckerImpl) CheckDownloadedObjectSha256sum(fsBackend afero.Fs, downloadDir string, expectedSha256sum string) (bool, error) {
	calculatedSha256sum, err := utils.FileSha256sum(fsBackend, path.Join(downloadDir, expectedSha256sum))
	if err != nil {
		return false, err
	}

	return calculatedSha256sum == expectedSha256sum, nil
}

type UpdateHub struct {
	Controller
	CopyBackend copy.Interface `json:"-"`

	Version                   string
	Settings                  *Settings
	Store                     afero.Fs
	FirmwareMetadata          metadata.FirmwareMetadata
	PublicKey                 *rsa.PublicKey
	TimeStep                  time.Duration
	Updater                   client.Updater
	Reporter                  client.Reporter
	lastInstalledPackageUID   string
	ActiveInactiveBackend     activeinactive.Interface
	lastReportedState         string
	StateChangeCallbackPath   string
	ErrorCallbackPath         string
	ValidateCallbackPath      string
	RollbackCallbackPath      string
	InstallIfDifferentBackend installifdifferent.Interface
	Sha256Checker
	utils.Rebooter
	utils.CmdLineExecuter
	state            State
	previousState    State
	stateMutex       sync.Mutex
	DefaultApiClient *client.ApiClient
}

func NewUpdateHub(
	gitversion string,
	stateChangeCallbackPath string,
	errorCallbackPath string,
	validateCallbackPath string,
	rollbackCallbackPath string,
	fs afero.Fs,
	fm metadata.FirmwareMetadata,
	pubKey *rsa.PublicKey,
	initialState State,
	settings *Settings,
	DefaultApiClient *client.ApiClient) *UpdateHub {

	uh := &UpdateHub{
		ActiveInactiveBackend:     &activeinactive.DefaultImpl{CmdLineExecuter: &utils.CmdLine{}},
		Version:                   gitversion,
		state:                     initialState,
		previousState:             nil,
		Updater:                   client.NewUpdateClient(),
		TimeStep:                  time.Minute,
		Store:                     fs,
		FirmwareMetadata:          fm,
		PublicKey:                 pubKey,
		Settings:                  settings,
		Reporter:                  client.NewReportClient(),
		Sha256Checker:             &Sha256CheckerImpl{},
		InstallIfDifferentBackend: &installifdifferent.DefaultImpl{FileSystemBackend: fs},
		CopyBackend:               copy.ExtendedIO{},
		Rebooter:                  &utils.RebooterImpl{},
		CmdLineExecuter:           &utils.CmdLine{},
		StateChangeCallbackPath:   stateChangeCallbackPath,
		ErrorCallbackPath:         errorCallbackPath,
		ValidateCallbackPath:      validateCallbackPath,
		RollbackCallbackPath:      rollbackCallbackPath,
		DefaultApiClient:          DefaultApiClient,
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

func (uh *UpdateHub) stateChangeCallback(state State, action string) (State, error) {
	exists, _ := afero.Exists(uh.Store, uh.StateChangeCallbackPath)
	if !exists {
		return nil, nil
	}

	s := StateToString(state.ID())
	output, err := uh.CmdLineExecuter.Execute(fmt.Sprintf("%s %s %s", uh.StateChangeCallbackPath, action, s))
	if err != nil {
		return nil, err
	}

	flow, _ := DetermineTransitionFlow(output)

	switch flow {
	case TransitionFlowCancelled:
		return NewIdleState(), nil
	case TransitionFlowPostponed:
		log.Warn("postponed state transition not supported yet")
		return NewIdleState(), nil
	}

	return nil, nil
}

func (uh *UpdateHub) errorCallback(message string) error {
	exists, _ := afero.Exists(uh.Store, uh.ErrorCallbackPath)
	if !exists {
		return nil
	}

	_, err := uh.CmdLineExecuter.Execute(fmt.Sprintf("%s '%s'", uh.ErrorCallbackPath, message))

	return err
}

func (uh *UpdateHub) validateCallback() error {
	exists, _ := afero.Exists(uh.Store, uh.ValidateCallbackPath)
	if !exists {
		return nil
	}

	_, err := uh.CmdLineExecuter.Execute(uh.ValidateCallbackPath)

	return err
}

func (uh *UpdateHub) rollbackCallback() error {
	exists, _ := afero.Exists(uh.Store, uh.RollbackCallbackPath)
	if !exists {
		return nil
	}

	_, err := uh.CmdLineExecuter.Execute(uh.RollbackCallbackPath)

	return err
}

func (uh *UpdateHub) ProcessCurrentState() State {
	uh.stateMutex.Lock()
	defer uh.stateMutex.Unlock()

	var err error

	es, isErrorState := uh.state.(*ErrorState)
	if isErrorState {
		uh.ReportCurrentState()
		// this must be done after the report, because the report uses it
		uh.previousState = uh.state

		err = uh.errorCallback(es.cause.Error())
		if err != nil {
			log.Warn(err)
		}

		state, _ := uh.state.Handle(uh)
		uh.state = state
	} else {
		flow, err := uh.stateChangeCallback(uh.state, "enter")

		uh.ReportCurrentState()
		// this must be done after the report, because the report uses it
		uh.previousState = uh.state

		if err != nil {
			log.Error(err)
			uh.state = NewErrorState(uh.state.ApiClient(), nil, NewTransientError(err))
			return uh.state
		}

		if flow != nil {
			uh.state = flow
			return uh.state
		}

		state, cancel := uh.state.Handle(uh)

		flow, err = uh.stateChangeCallback(uh.state, "leave")
		if err != nil {
			log.Error(err)
			uh.state = NewErrorState(uh.state.ApiClient(), nil, NewTransientError(err))
			return uh.state
		}

		if flow != nil {
			uh.state = flow
			return uh.state
		}

		cs, ok := uh.state.(*CancellableState)
		if cancel && ok {
			uh.state = cs.NextState()
		} else {
			uh.state = state
		}
	}

	return uh.state
}

type Controller interface {
	ProbeUpdate(*client.ApiClient, int) (*metadata.UpdateMetadata, []byte, time.Duration, error)
	DownloadUpdate(*client.ApiClient, *metadata.UpdateMetadata, <-chan bool, chan<- int) error
	InstallUpdate(*metadata.UpdateMetadata, chan<- int) error
}

func (uh *UpdateHub) ProbeUpdate(apiClient *client.ApiClient, retries int) (*metadata.UpdateMetadata, []byte, time.Duration, error) {
	var data struct {
		Retries int `json:"retries"`
		metadata.FirmwareMetadata
		LastInstalledPackage string `json:"last-installed-package,omitempty"`
	}

	data.FirmwareMetadata = uh.FirmwareMetadata
	data.Retries = retries
	data.LastInstalledPackage = uh.lastInstalledPackageUID

	updateMetadataPath := path.Join(uh.Settings.DownloadDir, metadata.UpdateMetadataFilename)

	updateMetadata, signature, extraPoll, err := uh.Updater.ProbeUpdate(apiClient.Request(), client.UpgradesEndpoint, data)
	if err != nil {
		uh.Store.Remove(updateMetadataPath)
		return nil, nil, -1, err
	}

	if updateMetadata == nil || updateMetadata.(*metadata.UpdateMetadata) == nil {
		uh.Store.Remove(updateMetadataPath)
		return nil, signature, extraPoll, nil
	}

	um := updateMetadata.(*metadata.UpdateMetadata)
	afero.WriteFile(uh.Store, updateMetadataPath, um.RawBytes, 0644)

	return um, signature, extraPoll, nil
}

// it is recommended to use a buffered channel for "progressChan" to ensure no progress event is lost
func (uh *UpdateHub) DownloadUpdate(apiClient *client.ApiClient, updateMetadata *metadata.UpdateMetadata, cancel <-chan bool, progressChan chan<- int) error {
	indexToInstall, err := GetIndexOfObjectToBeInstalled(uh.ActiveInactiveBackend, updateMetadata)
	if err != nil {
		return err
	}

	packageUID := updateMetadata.PackageUID()

	log.Info(fmt.Sprintf("downloading update. (package-uid: %s)", packageUID))

	uh.clearDownloadDir(updateMetadata.Objects[indexToInstall])

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

		log.Debug("route: ", uri)

		wr, err := uh.Store.OpenFile(objectPath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
		if err != nil {
			return err
		}
		defer wr.Close()

		stat, _ := wr.Stat()

		var cr *httptoo.BytesContentRange

		if stat.Size() > 0 {
			cr, err = uh.Updater.GetUpdateContentRange(apiClient.Request(), uri, stat.Size())
			if err != nil {
				return err
			}

			log.Debug(fmt.Sprintf("first_bytes=%d last_bytes=%d length=%d", cr.First, cr.Last, cr.Length))
		}

		rd, _, err := uh.Updater.DownloadUpdate(apiClient.Request(), uri, cr)
		if err != nil {
			return err
		}

		if rd != nil {
			defer rd.Close()

			if cr != nil && cr.First > 0 {
				log.Info("resuming object download")
			}

			_, err = uh.CopyBackend.Copy(wr, rd, 30*time.Second, cancel, utils.ChunkSize, 0, -1, false)
			if err != nil {
				return err
			}
		}

		ok, err := uh.CheckDownloadedObjectSha256sum(uh.Store, uh.Settings.DownloadDir, obj.GetObjectMetadata().Sha256sum)
		if !ok {
			uh.Store.Remove(path.Join(uh.Settings.DownloadDir, obj.GetObjectMetadata().Sha256sum))
			return ErrSha256sum
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
		ok, err := uh.CheckDownloadedObjectSha256sum(uh.Store, uh.Settings.DownloadDir, obj.GetObjectMetadata().Sha256sum)
		if !ok {
			if err != nil {
				return err
			}

			log.Error(ErrSha256sum)

			return ErrSha256sum
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

		var previousStateString string
		if uh.previousState != nil {
			previousStateString = StateToString(uh.previousState.ID())
		}

		err := uh.Reporter.ReportState(uh.state.ApiClient().Request(), packageUID, previousStateString, stateString, errorMessage, uh.FirmwareMetadata)
		if err != nil {
			return err
		}

		uh.lastReportedState = stateString
	}

	return nil
}

// Start starts the updatehub
func (uh *UpdateHub) Start() {
	uh.stateMutex.Lock()
	defer uh.stateMutex.Unlock()

	if uh.Settings.UpgradeToInstallation >= 0 {
		// new installation just booted

		active, err := uh.ActiveInactiveBackend.Active()
		if err != nil {
			e := fmt.Errorf("couldn't get active partition, cannot detect whether the installation is successful or not. Error: %s", err)
			uh.state = NewErrorState(uh.DefaultApiClient, nil, NewTransientError(e))
			return
		}

		if uh.Settings.UpgradeToInstallation == active {
			err := uh.validateProcedure()
			if err != nil {
				// actually the code will never get here since it will
				// reboot inside the validate procedure. but just in
				// case something unexpected occurs, this will tell us
				// what happened
				uh.state = NewErrorState(uh.DefaultApiClient, nil, NewTransientError(err))
				return
			}
		} else {
			err := uh.rollbackProcedure()
			if err != nil {
				uh.state = NewErrorState(uh.DefaultApiClient, nil, NewTransientError(err))
				return
			}
		}
	}

	now := time.Now()
	now = time.Unix(now.Unix(), 0)

	poll := NewPollState(uh.Settings.PollingInterval)

	uh.state = poll

	timeZero := (time.Time{}).UTC()

	if uh.Settings.FirstPoll.After(now) && uh.Settings.LastPoll != timeZero {
		uh.Settings.FirstPoll = timeZero
	}

	if uh.Settings.FirstPoll == timeZero {
		// Apply an offset in first poll
		uh.Settings.FirstPoll = now.Add(time.Duration(rand.Int63n(int64(uh.Settings.PollingInterval))))
		uh.Settings.Save(uh.Store)
	} else if uh.Settings.LastPoll == timeZero && now.After(uh.Settings.FirstPoll) {
		// it never did a poll before
		uh.state = NewProbeState(uh.DefaultApiClient)
	} else if uh.Settings.LastPoll.Add(uh.Settings.PollingInterval).Before(now) {
		// pending regular interval
		uh.state = NewProbeState(uh.DefaultApiClient)
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

	if uh.Settings.ProbeASAP {
		uh.state = NewProbeState(uh.DefaultApiClient)
	}
}

func (uh *UpdateHub) rollbackProcedure() error {
	return uh.rollbackCallback()
}

func (uh *UpdateHub) validateProcedure() error {
	aii := uh.ActiveInactiveBackend

	err := uh.validateCallback()
	if err != nil {

		active, activeErr := aii.Active()
		if activeErr != nil {
			return activeErr
		}

		newActive := (active - 1) * -1

		// Switch the active partion
		setActiveErr := aii.SetActive(newActive)
		if setActiveErr != nil {
			return setActiveErr
		}

		// Force reboot
		uh.Rebooter.Reboot()

		return err
	}

	// We can Validate the update by calling
	// 'updatehub-active-validated', and then go
	// back to the state machine.
	err = aii.SetValidate()
	if err != nil {
		return err
	}

	return nil
}

func (uh *UpdateHub) clearDownloadDir(objects []metadata.Object) {
	mapFiles := map[string]bool{}

	dir, _ := afero.ReadDir(uh.Store, uh.Settings.DownloadDir)
	for _, file := range dir {
		mapFiles[file.Name()] = false
	}

	for _, obj := range objects {
		filename := obj.GetObjectMetadata().Sha256sum
		if _, ok := mapFiles[filename]; ok {
			mapFiles[filename] = true
		}
	}

	for filename, preserv := range mapFiles {
		if !preserv {
			uh.Store.Remove(path.Join(uh.Settings.DownloadDir, filename))
		}
	}
}

func (uh *UpdateHub) hasPendingDownload(updateMetadata *metadata.UpdateMetadata) (bool, error) {
	indexToInstall, err := GetIndexOfObjectToBeInstalled(uh.ActiveInactiveBackend, updateMetadata)
	if err != nil {
		return false, err
	}

	for _, obj := range updateMetadata.Objects[indexToInstall] {
		objectUID := obj.GetObjectMetadata().Sha256sum
		objectPath := path.Join(uh.Settings.DownloadDir, objectUID)

		sha256sum, err := utils.FileSha256sum(uh.Store, objectPath)
		if os.IsNotExist(err) {
			return true, nil
		}

		if sha256sum != objectUID {
			return true, nil
		}
	}

	return false, nil
}
