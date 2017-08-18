/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeServerAddress(t *testing.T) {
	testCases := []struct {
		caseName        string
		inputAddress    string
		expectedAddress string
	}{
		{
			"AddressWithoutScheme",
			"127.0.0.1:8000",
			"https://127.0.0.1:8000",
		},
		{
			"AddressWithHTTPS",
			"https://127.0.0.1:8000",
			"https://127.0.0.1:8000",
		},
		{
			"AddressWithHTTP",
			"http://127.0.0.1:8000",
			"http://127.0.0.1:8000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			address, err := SanitizeServerAddress(tc.inputAddress)

			assert.Equal(t, nil, err)
			assert.Equal(t, tc.expectedAddress, address)
		})
	}
}

func TestSanitizeServerAddressWithError(t *testing.T) {
	address, err := SanitizeServerAddress("{")

	assert.EqualError(t, err, "parse https://{: invalid character \"{\" in host name")
	assert.Equal(t, "", address)
}
