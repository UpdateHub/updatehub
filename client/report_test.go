/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/updatehub/updatehub/metadata"
)

func TestReportState(t *testing.T) {
	responder := &struct {
		httpStatus int
		headers    http.Header
		body       string
	}{
		http.StatusOK,
		http.Header{},
		"",
	}

	rawBody := []byte{}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responder.headers = r.Header
		w.WriteHeader(responder.httpStatus)
		w.Header().Set("Content-Type", "application/json")

		fmt.Fprintf(w, responder.body)

		buf := new(bytes.Buffer)

		n, err := buf.ReadFrom(r.Body)
		assert.NoError(t, err)
		assert.NotZero(t, n)

		rawBody = buf.Bytes()
	}))

	defer s.Close()

	url, err := url.Parse(s.URL)
	assert.NoError(t, err)

	c := NewApiClient("http://" + url.Host)
	assert.NotNil(t, c)

	req := c.Request()
	assert.NotNil(t, req)

	assert.Equal(t, c, req.Client())

	reporter := NewReportClient()

	fm := metadata.FirmwareMetadata{
		ProductUID: "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
		DeviceIdentity: map[string]string{
			"id1": "value1",
		},
		DeviceAttributes: map[string]string{
			"attr1": "value1",
			"attr2": "value2",
		},
		Hardware: "board",
		Version:  "2.2",
	}

	err = reporter.ReportState(c.Request(), "packageUID", "previous_state", "state", "err_msg", fm)
	assert.NoError(t, err)

	var body map[string]interface{}

	err = json.Unmarshal(rawBody, &body)
	assert.NoError(t, err)

	expectedBody := make(map[string]interface{})
	expectedBody["error-message"] = "err_msg"
	expectedBody["package-uid"] = "packageUID"
	expectedBody["status"] = "state"
	expectedBody["version"] = "2.2"
	expectedBody["hardware"] = "board"
	expectedBody["product-uid"] = "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"
	expectedBody["device-identity"] = map[string]interface{}{"id1": "value1"}
	expectedBody["device-attributes"] = map[string]interface{}{"attr1": "value1", "attr2": "value2"}
	expectedBody["previous-state"] = "previous_state"

	assert.Equal(t, expectedBody, body)

	assert.Equal(t, "application/vnd.updatehub-v1+json", responder.headers.Get("Api-Content-Type"))
	assert.Equal(t, "application/json", responder.headers.Get("Content-Type"))
}
