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

	"github.com/UpdateHub/updatehub/metadata"
	"github.com/stretchr/testify/assert"
)

func TestCheckUpdate(t *testing.T) {
	expectedMetadata := &metadata.UpdateMetadata{}
	expectedDuration := 10 * time.Second

	cm := &ControllerMock{}
	cm.On("CheckUpdate", 0).Return(expectedMetadata, expectedDuration)

	m, d := cm.CheckUpdate(0)

	assert.Equal(t, expectedMetadata, m)
	assert.Equal(t, expectedDuration, d)

	cm.AssertExpectations(t)
}

func TestDownloadUpdate(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	metadata := &metadata.UpdateMetadata{}
	cancelChannel := make(<-chan bool)
	progressChannel := make(chan<- int)

	cm := &ControllerMock{}
	cm.On("DownloadUpdate", metadata, cancelChannel, progressChannel).Return(expectedError)

	err := cm.DownloadUpdate(metadata, cancelChannel, progressChannel)

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
