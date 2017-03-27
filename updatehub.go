/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"path"
	"time"

	"github.com/spf13/afero"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/utils"
)

type UpdateHub struct {
	Controller

	settings                *Settings
	store                   afero.Fs
	firmwareMetadata        metadata.FirmwareMetadata
	state                   State
	timeStep                time.Duration
	api                     *client.ApiClient
	updater                 client.Updater
	reporter                client.Reporter
	lastInstalledPackageUID string
}

type Controller interface {
	CheckUpdate(int) (*metadata.UpdateMetadata, int)
	FetchUpdate(*metadata.UpdateMetadata, <-chan bool) error
	ReportCurrentState() error
}

func (uh *UpdateHub) CheckUpdate(retries int) (*metadata.UpdateMetadata, int) {
	var data struct {
		Retries int `json:"retries"`
		metadata.FirmwareMetadata
	}

	data.FirmwareMetadata = uh.firmwareMetadata
	data.Retries = retries

	updateMetadata, extraPoll, err := uh.updater.CheckUpdate(uh.api.Request(), data)
	if err != nil || updateMetadata == nil {
		return nil, -1
	}

	return updateMetadata.(*metadata.UpdateMetadata), extraPoll
}

func (uh *UpdateHub) FetchUpdate(updateMetadata *metadata.UpdateMetadata, cancel <-chan bool) error {
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
	uri = path.Join(uri, uh.firmwareMetadata.ProductUID)
	uri = path.Join(uri, packageUID)
	uri = path.Join(uri, objectUID)

	file, err := uh.store.Create(path.Join(uh.settings.DownloadDir, objectUID))
	if err != nil {
		return err
	}

	defer file.Close()

	rd, contentLength, err := uh.updater.FetchUpdate(uh.api.Request(), uri)
	if err != nil {
		return err
	}

	wd := bufio.NewWriter(file)

	// FIXME: maybe use the "utils.Copier" interface here. if yes, we
	// can mock it for the tests
	eio := utils.ExtendedIO{}
	eio.Copy(wd, rd, 30*time.Second, cancel, utils.ChunkSize, 0, -1, false)

	wd.Flush()

	fmt.Println(contentLength)

	return nil
}

func (uh *UpdateHub) ReportCurrentState() error {
	if rs, ok := uh.state.(ReportableState); ok {
		packageUID, _ := rs.UpdateMetadata().Checksum()
		err := uh.reporter.ReportState(uh.api.Request(), packageUID, StateToString(uh.state.ID()))
		if err != nil {
			return err
		}
	}

	return nil
}

// StartPolling starts the polling process
func (uh *UpdateHub) StartPolling() {
	now := time.Now()
	now = time.Unix(now.Unix(), 0)

	uh.state = NewPollState()

	if uh.settings.FirstPoll == 0 {
		// Apply an offset in first poll
		uh.settings.FirstPoll = int(now.Add(time.Duration(rand.Intn(uh.settings.PollingInterval))).Unix())
	} else if uh.settings.LastPoll == 0 && uh.settings.FirstPoll <= int(now.Unix()) {
		// it never did a poll before
		uh.state = NewUpdateCheckState()
	} else if time.Unix(int64(uh.settings.LastPoll), 0).Add(time.Duration(uh.settings.PollingInterval)).Before(now) {
		// pending regular interval
		uh.state = NewUpdateCheckState()
	} else {
		nextPoll := time.Unix(int64(uh.settings.FirstPoll), 0)
		for nextPoll.Before(now) {
			nextPoll = nextPoll.Add(time.Duration(uh.settings.PollingInterval))
		}

		uh.state.(*PollState).ticksCount = int((int64(uh.settings.PollingInterval) - nextPoll.Sub(now).Nanoseconds()) / int64(uh.timeStep))
	}
}

func (uh *UpdateHub) MainLoop() {
	for {
		uh.ReportCurrentState()

		fmt.Println("Handling state:", StateToString(uh.state.ID()))

		state, cancelled := uh.state.Handle(uh)

		if state.ID() == UpdateHubStateError {
			if es, ok := state.(*ErrorState); ok {
				// FIXME: log error
				fmt.Println(es.cause)
			}
		}

		if cancelled {
			fmt.Println("State cancelled")
		}

		uh.state = state
	}
}
