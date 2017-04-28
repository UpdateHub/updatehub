/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package client

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/installmodes/copy"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/stretchr/testify/assert"
)

func TestCheckUpdateWithInvalidApiRequester(t *testing.T) {
	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		HardwareRevision: "",
		Version:          "version-value",
	}

	updateMetadata, extraPoll, err := uc.CheckUpdate(nil, UpgradesEndpoint, fm)

	assert.Nil(t, updateMetadata)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "invalid api requester")
}

func TestCheckUpdateWithNewRequestError(t *testing.T) {
	ac := NewApiClient("localhost")

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		HardwareRevision: "",
		Version:          "version-value",
	}

	updateMetadata, extraPoll, err := uc.CheckUpdate(ac.Request(), "/resource%s", fm)

	assert.Nil(t, updateMetadata)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "failed to create check update request")
}

func TestCheckUpdateWithApiDoError(t *testing.T) {
	ac := NewApiClient("invalid")

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		HardwareRevision: "",
		Version:          "version-value",
	}

	updateMetadata, extraPoll, err := uc.CheckUpdate(ac.Request(), "/resource", fm)

	assert.Nil(t, updateMetadata)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "check update request failed")
}

func TestCheckUpdateWithExtraPollHeaderError(t *testing.T) {
	expectedBody := []byte("expected body")
	address := "localhost"
	path := "/extra-poll-error"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		HardwareRevision: "",
		Version:          "version-value",
	}

	updateMetadata, extraPoll, err := uc.CheckUpdate(ac.Request(), path, fm)

	assert.Nil(t, updateMetadata)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "failed to parse extra poll header")
}

func TestCheckUpdateWithResponseBodyReadError(t *testing.T) {
	expectedBody := []byte("partial body")
	address := "localhost"
	path := "/response-body-error"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		HardwareRevision: "",
		Version:          "version-value",
	}

	updateMetadata, extraPoll, err := uc.CheckUpdate(ac.Request(), path, fm)

	assert.Nil(t, updateMetadata)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "error reading response body: unexpected EOF")
}

func TestCheckUpdateWithResponseBodyParseError(t *testing.T) {
	expectedBody := []byte("expected body")
	address := "localhost"
	path := "/resource"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		HardwareRevision: "",
		Version:          "version-value",
	}

	updateMetadata, extraPoll, err := uc.CheckUpdate(ac.Request(), path, fm)

	assert.Nil(t, updateMetadata)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "failed to parse upgrade response: invalid character 'e' looking for beginning of value")
}

func TestCheckUpdateWithInvalidStatusCode(t *testing.T) {
	expectedBody := []byte("expected body")
	address := "localhost"
	path := "/error-bad-gateway"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		HardwareRevision: "",
		Version:          "version-value",
	}

	updateMetadata, extraPoll, err := uc.CheckUpdate(ac.Request(), path, fm)

	assert.Nil(t, updateMetadata)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "invalid response received from the server. Status 502")
}

func TestCheckUpdateWithNoUpdateAvailable(t *testing.T) {
	expectedBody := []byte("expected body")
	address := "localhost"
	path := "/not-found"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		HardwareRevision: "",
		Version:          "version-value",
	}

	updateMetadata, extraPoll, err := uc.CheckUpdate(ac.Request(), path, fm)

	assert.Nil(t, updateMetadata)
	assert.Equal(t, time.Duration(3), extraPoll)
	assert.NoError(t, err)
}

func TestCheckUpdateWithUpdateAvailable(t *testing.T) {
	// declaration just to register the copy install mode
	_ = &copy.CopyObject{}

	expectedBody := `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      {
            "mode": "copy",
            "sha256sum": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
          }
	    ]
	  ],
	  "version": "1.2"
	}`
	address := "localhost"
	path := "/resource"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		HardwareRevision: "",
		Version:          "version-value",
	}

	updateMetadata, extraPoll, err := uc.CheckUpdate(ac.Request(), path, fm)

	assert.Equal(t, time.Duration(0), extraPoll)
	assert.NoError(t, err)

	um := updateMetadata.(*metadata.UpdateMetadata)

	assert.Equal(t, "0123456789", um.ProductUID)
	assert.Equal(t, "1.2", um.Version)
	assert.Equal(t, []byte(expectedBody), um.RawBytes)
	assert.Equal(t, 0, len(um.SupportedHardware))

	// Objects
	assert.Equal(t, 1, len(um.Objects))
	assert.Equal(t, 1, len(um.Objects[0]))
	assert.Equal(t, "copy", um.Objects[0][0].GetObjectMetadata().Mode)
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", um.Objects[0][0].GetObjectMetadata().Sha256sum)
}

func TestFetchUpdateWithInvalidApiRequester(t *testing.T) {
	uc := NewUpdateClient()

	body, contentLength, err := uc.FetchUpdate(nil, "/resource")

	assert.Nil(t, body)
	assert.Equal(t, int64(-1), contentLength)
	assert.EqualError(t, err, "invalid api requester")
}

func TestFetchUpdateWithNewRequestError(t *testing.T) {
	ac := NewApiClient("localhost")

	uc := NewUpdateClient()

	body, contentLength, err := uc.FetchUpdate(ac.Request(), "/resource%s")

	assert.Nil(t, body)
	assert.Equal(t, int64(-1), contentLength)
	assert.True(t, strings.Contains(err.Error(), "failed to create fetch update request"))
	assert.True(t, strings.Contains(err.Error(), "invalid URL escape"))
}

func TestFetchUpdateWithApiDoError(t *testing.T) {
	ac := NewApiClient("invalid")

	uc := NewUpdateClient()

	body, contentLength, err := uc.FetchUpdate(ac.Request(), "/resource")

	assert.Nil(t, body)
	assert.Equal(t, int64(-1), contentLength)
	assert.True(t, strings.Contains(err.Error(), "fetch update request failed"))
	assert.True(t, strings.Contains(err.Error(), "no such host"))
}

func TestFetchUpdateWithHTTPError(t *testing.T) {
	expectedBody := []byte("Not found")
	address := "localhost"
	path := "/not-found"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("%s:%d", address, port))

	uc := NewUpdateClient()

	body, contentLength, err := uc.FetchUpdate(ac.Request(), path)

	assert.Nil(t, body)
	assert.Equal(t, int64(-1), contentLength)
	assert.EqualError(t, err, "failed to fetch update. maybe the file is missing?")
}

func TestFetchUpdateWithSuccess(t *testing.T) {
	expectedBody := []byte("expected body")
	address := "localhost"
	path := "/resource"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("%s:%d", address, port))

	uc := NewUpdateClient()

	body, contentLength, err := uc.FetchUpdate(ac.Request(), path)
	defer body.Close()

	assert.Equal(t, int64(len(expectedBody)), contentLength)
	assert.NoError(t, err)

	buffer := make([]byte, contentLength)
	n, err := body.Read(buffer)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, contentLength, int64(n))

	assert.Equal(t, expectedBody, buffer)
}

type testHttpHandler struct {
	Path         string
	ResponseBody string
}

func (thh *testHttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/ping" {
		fmt.Fprintf(w, "pong")
	}

	if r.URL.Path == "/not-found" {
		w.Header().Add("Add-Extra-Poll", "3")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found")
	}

	if r.URL.Path == "/extra-poll-error" {
		w.Header().Add("Add-Extra-Poll", "@3")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, string(thh.ResponseBody))
	}

	if r.URL.Path == "/response-body-error" {
		w.Header().Add("Content-Length", fmt.Sprintf("%d", len(thh.ResponseBody)+1))
		fmt.Fprintf(w, string(thh.ResponseBody))
	}

	if r.URL.Path == "/error-bad-gateway" {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, string(thh.ResponseBody))
	}

	if r.URL.Path == thh.Path {
		fmt.Fprintf(w, string(thh.ResponseBody))
	}
}

var port = 8080

func StartNewTestHttpServer(address string, handler http.Handler) (int, *http.Server, error) {
	port++

	addressWithPort := fmt.Sprintf("%s:%d", address, port)

	server := &http.Server{Addr: addressWithPort, Handler: handler}

	go server.ListenAndServe()

	var body string

	// loop doing "GET"s to ensure the server already started when we
	// return
	for {
		resp, err := http.Get("http://" + addressWithPort + "/ping")

		if err == nil {
			defer resp.Body.Close()
			b, readallErr := ioutil.ReadAll(resp.Body)
			if readallErr != nil {
				return port, server, readallErr
			}

			body = string(b)

			break
		}

		if strings.Contains(err.Error(), "connection refused") {
			continue
		}

		return port, server, err
	}

	if string(body) != "pong" {
		return port, server, fmt.Errorf("body should be 'pong'")
	}

	return port, server, nil
}
