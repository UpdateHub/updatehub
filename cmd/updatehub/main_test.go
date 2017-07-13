/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeServerAddress(t *testing.T) {
	testCases := []struct {
		caseName        string
		inputAddress    string
		disableHTTPS    bool
		expectedAddress string
	}{
		{
			"AddressWithoutProtocolAndHTTPSEnabled",
			"127.0.0.1:8000",
			false,
			"https://127.0.0.1:8000",
		},
		{
			"AddressWithoutProtocolAndHTTPSDisabled",
			"127.0.0.1:8000",
			true,
			"http://127.0.0.1:8000",
		},
		{
			"AddressWithHTTPProtocolAndHTTPSEnabled",
			"http://127.0.0.1:8000",
			false,
			"https://127.0.0.1:8000",
		},
		{
			"AddressWithHTTPSProtocolAndHTTPSDisabled",
			"https://127.0.0.1:8000",
			true,
			"http://127.0.0.1:8000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.caseName, func(t *testing.T) {
			address, err := sanitizeServerAddress(tc.inputAddress, tc.disableHTTPS)

			assert.Equal(t, nil, err)
			assert.Equal(t, tc.expectedAddress, address)
		})
	}
}

func TestSanitizeServerAddressWithError(t *testing.T) {
	address, err := sanitizeServerAddress("{", true)

	assert.EqualError(t, err, "parse https://{: invalid character \"{\" in host name")
	assert.Equal(t, "", address)
}
