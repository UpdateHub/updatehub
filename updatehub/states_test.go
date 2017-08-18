/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"time"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
)

type testController struct {
	extraPoll               time.Duration
	pollingInterval         time.Duration
	updateAvailable         bool
	downloadUpdateError     error
	installUpdateError      error
	reportCurrentStateError error
	progressList            []int
}

const (
	validJSONMetadata = `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      {
            "mode": "test",
            "sha256sum": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
          }
	    ]
	  ]
	}`

	validJSONMetadataWithActiveInactive = `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      {
            "mode": "test",
            "sha256sum": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
            "target": "/dev/xx1",
            "target-type": "device"
          }
	    ]
        ,
	    [
	      {
            "mode": "test",
            "sha256sum": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
            "target": "/dev/xx2",
            "target-type": "device"
          }
	    ]
	  ]
	}`
)

func (c *testController) ProbeUpdate(apiClient *client.ApiClient, retries int) (*metadata.UpdateMetadata, time.Duration) {
	if c.updateAvailable {
		return &metadata.UpdateMetadata{}, c.extraPoll
	}

	return nil, c.extraPoll
}

func (c *testController) DownloadUpdate(apiClient *client.ApiClient, updateMetadata *metadata.UpdateMetadata, cancel <-chan bool, progressChan chan<- int) error {
	for _, p := range c.progressList {
		// "non-blocking" write to channel
		select {
		case progressChan <- p:
		default:
		}
	}

	return c.downloadUpdateError
}

func (c *testController) InstallUpdate(updateMetadata *metadata.UpdateMetadata, progressChan chan<- int) error {
	for _, p := range c.progressList {
		// "non-blocking" write to channel
		select {
		case progressChan <- p:
		default:
		}
	}

	return c.installUpdateError
}

func (c *testController) ReportCurrentState() error {
	return c.reportCurrentStateError
}

type testReportableState struct {
	BaseState
	ReportableState

	updateMetadata *metadata.UpdateMetadata
}

func (state *testReportableState) Handle(uh *UpdateHub) (State, bool) {
	return nil, true
}

func (state *testReportableState) UpdateMetadata() *metadata.UpdateMetadata {
	return state.updateMetadata
}

type TestObject struct {
	metadata.Object
}
