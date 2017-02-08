package main

import (
	"bufio"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/spf13/afero"

	"bitbucket.org/ossystems/agent/client"
	"bitbucket.org/ossystems/agent/metadata"
	"bitbucket.org/ossystems/agent/utils"
)

type EasyFota struct {
	Controller

	settings         *Settings
	store            afero.Fs
	firmwareMetadata metadata.FirmwareMetadata
	state            State
	pollInterval     int
	timeStep         time.Duration
	api              *client.ApiClient
	updater          client.Updater
	reporter         client.Reporter
}

type Controller interface {
	CheckUpdate() (*metadata.UpdateMetadata, int)
	FetchUpdate(*metadata.UpdateMetadata, <-chan bool) error
	ReportCurrentState() error
}

func (fota *EasyFota) CheckUpdate() (*metadata.UpdateMetadata, int) {
	updateMetadata, extraPoll, err := fota.updater.CheckUpdate(fota.api.Request())
	if err != nil || updateMetadata == nil {
		return nil, 0
	}

	return updateMetadata.(*metadata.UpdateMetadata), extraPoll
}

func (fota *EasyFota) FetchUpdate(updateMetadata *metadata.UpdateMetadata, cancel <-chan bool) error {
	// For now, we installs the first object
	// FIXME: What object I should to install?
	obj := updateMetadata.Objects[0][0]

	if obj == nil {
		return errors.New("object not found")
	}

	packageUID, err := updateMetadata.Checksum()
	if err != nil {
		return err
	}

	objectUID := obj.GetObjectMetadata().Sha256sum

	uri := "/"
	uri = path.Join(uri, fota.firmwareMetadata.ProductUID)
	uri = path.Join(uri, packageUID)
	uri = path.Join(uri, objectUID)

	file, err := fota.store.Create(path.Join(fota.settings.DownloadDir, objectUID))
	if err != nil {
		return err
	}

	defer file.Close()

	rd, contentLength, err := fota.updater.FetchUpdate(fota.api.Request(), uri)
	if err != nil {
		return err
	}

	wd := bufio.NewWriter(file)

	utils.Copy(wd, rd, 30*time.Second, cancel, utils.ChunkSize, -1)

	wd.Flush()

	fmt.Println(contentLength)

	return nil
}

func (fota *EasyFota) ReportCurrentState() error {
	if rs, ok := fota.state.(ReportableState); ok {
		packageUID, _ := rs.UpdateMetadata().Checksum()
		err := fota.reporter.ReportState(fota.api.Request(), packageUID, StateToString(fota.state.Id()))
		if err != nil {
			return err
		}
	}

	return nil
}

func (fota *EasyFota) MainLoop() {
	for {
		fota.ReportCurrentState()

		fmt.Println("Handling state:", StateToString(fota.state.Id()))

		state, cancelled := fota.state.Handle(fota)

		if state.Id() == EasyFotaStateError {
			if es, ok := state.(*ErrorState); ok {
				// FIXME: log error
				fmt.Println(es.cause)
			}
		}

		if cancelled {
			fmt.Println("State cancelled")
		}

		fota.state = state
	}
}
