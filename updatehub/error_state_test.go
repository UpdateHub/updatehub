/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatehub

import (
	"fmt"
	"testing"

	"github.com/UpdateHub/updatehub/client"
	"github.com/stretchr/testify/assert"
)

func TestNewErrorStateToMap(t *testing.T) {
	state := NewErrorState(client.NewApiClient("address"), nil, NewTransientError(fmt.Errorf("error message")))

	expectedMap := map[string]interface{}{}
	expectedMap["status"] = "error"
	expectedMap["error"] = "transient error: error message"

	assert.Equal(t, expectedMap, state.ToMap())
}
