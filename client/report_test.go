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

	c := NewApiClient(url.Host)
	assert.NotNil(t, c)

	req := c.Request()
	assert.NotNil(t, req)

	assert.Equal(t, c, req.Client())

	reporter := NewReportClient()

	err = reporter.ReportState(c.Request(), "packageUID", "state")
	assert.NoError(t, err)

	var body map[string]interface{}

	err = json.Unmarshal(rawBody, &body)
	assert.NoError(t, err)

	expectedBody := make(map[string]interface{})
	expectedBody["error-message"] = ""
	expectedBody["package-uid"] = "packageUID"
	expectedBody["status"] = "state"

	assert.Equal(t, expectedBody, body)
}
