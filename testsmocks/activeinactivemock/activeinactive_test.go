/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package activeinactivemock

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActive(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	aim := &ActiveInactiveMock{}
	aim.On("Active").Return(0, expectedError)

	a, err := aim.Active()

	assert.Equal(t, 0, a)
	assert.Equal(t, expectedError, err)

	aim.AssertExpectations(t)
}

func TestDownloadUpdate(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	aim := &ActiveInactiveMock{}
	aim.On("SetActive", 1).Return(expectedError)

	err := aim.SetActive(1)

	assert.Equal(t, expectedError, err)

	aim.AssertExpectations(t)
}

func TestValidateUpdate(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	aim := &ActiveInactiveMock{}
	aim.On("SetValidate").Return(expectedError)

	err := aim.SetValidate()

	assert.Equal(t, expectedError, err)

	aim.AssertExpectations(t)
}
