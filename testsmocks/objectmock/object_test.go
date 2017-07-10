/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package objectmock

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetup(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	om := &ObjectMock{}
	om.On("Setup").Return(expectedError)

	err := om.Setup()

	assert.Equal(t, expectedError, err)

	om.AssertExpectations(t)
}

func TestInstall(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	om := &ObjectMock{}
	om.On("Install", "downloaddir").Return(expectedError)

	err := om.Install("downloaddir")

	assert.Equal(t, expectedError, err)

	om.AssertExpectations(t)
}

func TestCleanup(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	om := &ObjectMock{}
	om.On("Cleanup").Return(expectedError)

	err := om.Cleanup()

	assert.Equal(t, expectedError, err)

	om.AssertExpectations(t)
}
