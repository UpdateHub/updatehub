/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package controllermock

import (
	"fmt"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/stretchr/testify/assert"
)

func TestProbeUpdate(t *testing.T) {
	expectedMetadata := &metadata.UpdateMetadata{}
	expectedDuration := 10 * time.Second

	apiClient := client.NewApiClient("address")
	cm := &ControllerMock{}
	cm.On("ProbeUpdate", apiClient, 0).Return(expectedMetadata, expectedDuration)

	m, d := cm.ProbeUpdate(apiClient, 0)

	assert.Equal(t, expectedMetadata, m)
	assert.Equal(t, expectedDuration, d)

	cm.AssertExpectations(t)
}

func TestDownloadUpdate(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	metadata := &metadata.UpdateMetadata{}
	cancelChannel := make(<-chan bool)
	progressChannel := make(chan<- int)

	apiClient := client.NewApiClient("address")

	cm := &ControllerMock{}
	cm.On("DownloadUpdate", apiClient, metadata, cancelChannel, progressChannel).Return(expectedError)

	err := cm.DownloadUpdate(apiClient, metadata, cancelChannel, progressChannel)

	assert.Equal(t, expectedError, err)

	cm.AssertExpectations(t)
}

func TestInstallUpdate(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	metadata := &metadata.UpdateMetadata{}
	progressChannel := make(chan<- int)

	cm := &ControllerMock{}
	cm.On("InstallUpdate", metadata, progressChannel).Return(expectedError)

	err := cm.InstallUpdate(metadata, progressChannel)

	assert.Equal(t, expectedError, err)

	cm.AssertExpectations(t)
}
