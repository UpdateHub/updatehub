/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package cmdlinemock

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecute(t *testing.T) {
	expectedError := fmt.Errorf("some error")

	clem := &CmdLineExecuterMock{}
	clem.On("Execute", "cmdline").Return([]byte("output"), expectedError)

	out, err := clem.Execute("cmdline")

	assert.Equal(t, []byte("output"), out)
	assert.Equal(t, expectedError, err)

	clem.AssertExpectations(t)
}
