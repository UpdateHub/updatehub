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

	"github.com/anacrolix/missinggo/httptoo"
	"github.com/stretchr/testify/assert"
	"github.com/updatehub/updatehub/client"
	"github.com/updatehub/updatehub/installmodes/copy"
	"github.com/updatehub/updatehub/metadata"
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

func TestProbeUpdate(t *testing.T) {
	_ = &copy.CopyObject{} // just to register the copy object

	expectedSignature := []byte("signature")
	expectedError := fmt.Errorf("some error")
	api := client.NewApiClient("localhost")

	expectedUpdateMetadata, err := metadata.NewUpdateMetadata([]byte(validUpdateMetadata))
	assert.NoError(t, err)

	um := &UpdaterMock{}
	um.On("ProbeUpdate", api.Request(), "uri", "data").Return(expectedUpdateMetadata, expectedSignature, time.Duration(5), expectedError)

	updateMetadata, signature, extraPoll, err := um.ProbeUpdate(api.Request(), "uri", "data")

	assert.Equal(t, expectedUpdateMetadata, updateMetadata)
	assert.Equal(t, expectedSignature, signature)
	assert.Equal(t, time.Duration(5), extraPoll)
	assert.Equal(t, expectedError, err)

	um.AssertExpectations(t)
}

func TestDownloadUpdate(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	api := client.NewApiClient("localhost")

	expectedBody := ioutil.NopCloser(bytes.NewBuffer([]byte("{\"content\": true}")))

	um := &UpdaterMock{}
	um.On("DownloadUpdate", api.Request(), "uri").Return(expectedBody, int64(19), expectedError)

	bodyRD, contentLength, err := um.DownloadUpdate(api.Request(), "uri", &httptoo.BytesContentRange{})

	assert.Equal(t, expectedBody, bodyRD)
	assert.Equal(t, int64(19), contentLength)
	assert.Equal(t, expectedError, err)

	um.AssertExpectations(t)
}

func TestGetUpdateContentRange(t *testing.T) {
	expectedError := fmt.Errorf("some error")
	api := client.NewApiClient("localhost")

	expectedContentRange := &httptoo.BytesContentRange{}

	um := &UpdaterMock{}
	um.On("GetUpdateContentRange", api.Request(), "uri", int64(0)).Return(expectedContentRange, expectedError)

	contentRange, err := um.GetUpdateContentRange(api.Request(), "uri", 0)

	assert.Equal(t, expectedContentRange, contentRange)
	assert.Equal(t, expectedError, err)

	um.AssertExpectations(t)
}
