/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package controllermock

import (
	"time"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/stretchr/testify/mock"
)

type ControllerMock struct {
	mock.Mock
}

func (cm *ControllerMock) ProbeUpdate(apiClient *client.ApiClient, retries int) (*metadata.UpdateMetadata, []byte, time.Duration) {
	args := cm.Called(apiClient, retries)
	return args.Get(0).(*metadata.UpdateMetadata), args.Get(1).([]byte), args.Get(2).(time.Duration)
}

func (cm *ControllerMock) DownloadUpdate(apiClient *client.ApiClient, updateMetadata *metadata.UpdateMetadata, cancel <-chan bool, progressChan chan<- int) error {
	args := cm.Called(apiClient, updateMetadata, cancel, progressChan)
	return args.Error(0)
}

func (cm *ControllerMock) InstallUpdate(updateMetadata *metadata.UpdateMetadata, progressChan chan<- int) error {
	args := cm.Called(updateMetadata, progressChan)
	return args.Error(0)
}
