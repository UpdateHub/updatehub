/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package updatermock

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installmodes/copy"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/stretchr/testify/assert"
)

const (
	validUpdateMetadata = `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "copy", "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" }
    ]
  ]
}`
)

func TestCheckUpdate(t *testing.T) {
	_ = &copy.CopyObject{} // just to register the copy object

	expectedError := fmt.Errorf("some error")
	api := client.NewApiClient("localhost")

	expectedUpdateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	um := &UpdaterMock{}
	um.On("CheckUpdate", api.Request(), "uri", "data").Return(expectedUpdateMetadata, time.Duration(5), expectedError)

	updateMetadata, extraPoll, err := um.CheckUpdate(api.Request(), "uri", "data")

	assert.Equal(t, expectedUpdateMetadata, updateMetadata)
	assert.Equal(t, time.Duration(5), extraPoll)
	assert.Equal(t, expectedError, err)

	um.AssertExpectations(t)
}

func TestFetchUpdate(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	api := client.NewApiClient("localhost")

	expectedBody := ioutil.NopCloser(bytes.NewBuffer([]byte("{\"content\": true}")))

	um := &UpdaterMock{}
	um.On("FetchUpdate", api.Request(), "uri").Return(expectedBody, int64(19), expectedError)

	bodyRD, contentLength, err := um.FetchUpdate(api.Request(), "uri")

	assert.Equal(t, expectedBody, bodyRD)
	assert.Equal(t, int64(19), contentLength)
	assert.Equal(t, expectedError, err)

	um.AssertExpectations(t)
}
