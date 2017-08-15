/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package responsewritermock

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeader(t *testing.T) {
	rwm := &ResponseWriterMock{}

	expected := http.Header{}

	rwm.On("Header").Return(expected)
	h := rwm.Header()

	assert.Equal(t, expected, h)

	rwm.AssertExpectations(t)
}

func TestWrite(t *testing.T) {
	buffer := []byte("content")

	rwm := &ResponseWriterMock{}

	expectedError := fmt.Errorf("some error")

	rwm.On("Write", buffer).Return(0, expectedError)
	i, err := rwm.Write(buffer)

	assert.Equal(t, expectedError, err)
	assert.Equal(t, 0, i)

	rwm.AssertExpectations(t)
}

func TestWriteHeader(t *testing.T) {
	rwm := &ResponseWriterMock{}

	rwm.On("WriteHeader", 200).Return()
	rwm.WriteHeader(200)

	rwm.AssertExpectations(t)
}
