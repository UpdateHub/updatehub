/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package rebootermock

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReboot(t *testing.T) {
	expectedErr := fmt.Errorf("custom error")

	rm := &RebooterMock{}
	rm.On("Reboot").Return(expectedErr)

	err := rm.Reboot()

	assert.Equal(t, expectedErr, err)

	rm.AssertExpectations(t)
}
