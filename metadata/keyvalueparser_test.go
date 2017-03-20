/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package metadata

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyValueParser(t *testing.T) {
	expected := map[string]string{
		"key":         "value",
		"another_key": "value",
	}

	keyvalue, err := keyValueParser(bytes.NewReader([]byte("key=value\nanother_key=value")))

	assert.NoError(t, err)
	assert.Equal(t, expected, keyvalue)
}

func TestKeyValueParserWithInvalidKey(t *testing.T) {
	keyvalue, err := keyValueParser(bytes.NewReader([]byte("key=value\nanother_key=value\ninvalid_key")))

	assert.EqualError(t, err, "'=' expected on line 3")
	assert.Nil(t, keyvalue)
}

func TestKeyValueParserWithEmptyKey(t *testing.T) {
	expected := map[string]string{
		"key":       "value",
		"empty_key": "",
	}

	keyvalue, err := keyValueParser(bytes.NewReader([]byte("key=value\nempty_key=")))

	assert.NoError(t, err)
	assert.Equal(t, expected, keyvalue)
}
