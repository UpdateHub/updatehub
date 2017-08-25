/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package client

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/installmodes/imxkobs"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/stretchr/testify/assert"
)

func TestProbeUpdateWithInvalidApiRequester(t *testing.T) {
	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}

	updateMetadata, signature, extraPoll, err := uc.ProbeUpdate(nil, UpgradesEndpoint, fm)

	assert.Nil(t, updateMetadata)
	assert.Nil(t, signature)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "invalid api requester")
}

func TestProbeUpdateWithNewRequestError(t *testing.T) {
	ac := NewApiClient("http://localhost")

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}

	updateMetadata, signature, extraPoll, err := uc.ProbeUpdate(ac.Request(), "/resource%s", fm)

	assert.Nil(t, updateMetadata)
	assert.Nil(t, signature)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "failed to create probe update request: parse http://localhost/resource%s: invalid URL escape \"%s\"")
}

func TestProbeUpdateWithApiDoError(t *testing.T) {
	ac := NewApiClient("http://invalid")

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}

	updateMetadata, signature, extraPoll, err := uc.ProbeUpdate(ac.Request(), "/resource", fm)

	assert.Nil(t, updateMetadata)
	assert.Nil(t, signature)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.True(t, strings.HasPrefix(err.Error(), "probe update request failed: Post http://invalid/resource: dial tcp: lookup invalid"))
}

func TestProbeUpdateWithExtraPollHeaderError(t *testing.T) {
	expectedBody := []byte("expected body")
	address := "localhost"
	path := "/extra-poll-error"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("http://%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}

	updateMetadata, signature, extraPoll, err := uc.ProbeUpdate(ac.Request(), path, fm)

	assert.Nil(t, updateMetadata)
	assert.Nil(t, signature)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "failed to parse extra poll header: strconv.ParseInt: parsing \"@3\": invalid syntax")
}

func TestProbeUpdateWithResponseBodyReadError(t *testing.T) {
	expectedBody := []byte("partial body")
	address := "localhost"
	path := "/response-body-error"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("http://%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}

	updateMetadata, signature, extraPoll, err := uc.ProbeUpdate(ac.Request(), path, fm)

	assert.Nil(t, updateMetadata)
	assert.Nil(t, signature)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "error reading response body: unexpected EOF")
}

func TestProbeUpdateWithResponseBodyParseError(t *testing.T) {
	expectedBody := []byte("expected body")
	address := "localhost"
	path := "/resource"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("http://%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}

	updateMetadata, signature, extraPoll, err := uc.ProbeUpdate(ac.Request(), path, fm)

	assert.Nil(t, updateMetadata)
	assert.Nil(t, signature)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "failed to parse upgrade response: invalid character 'e' looking for beginning of value")
}

func TestProbeUpdateWithInvalidStatusCode(t *testing.T) {
	expectedBody := []byte("expected body")
	address := "localhost"
	path := "/error-bad-gateway"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("http://%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}

	updateMetadata, signature, extraPoll, err := uc.ProbeUpdate(ac.Request(), path, fm)

	assert.Nil(t, updateMetadata)
	assert.Nil(t, signature)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.EqualError(t, err, "invalid response received from the server. HTTP code: 502")
}

func TestProbeUpdateWithNoUpdateAvailable(t *testing.T) {
	expectedBody := []byte("expected body")
	address := "localhost"
	path := "/not-found"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("http://%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}

	updateMetadata, signature, extraPoll, err := uc.ProbeUpdate(ac.Request(), path, fm)

	assert.Nil(t, updateMetadata)
	assert.Nil(t, signature)
	assert.Equal(t, time.Duration(3), extraPoll)
	assert.NoError(t, err)
}

func TestProbeUpdateWithUpdateAvailable(t *testing.T) {
	// declaration just to register the imxkobs install mode
	_ = &imxkobs.ImxKobsObject{}

	expectedBody := `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      {
            "mode": "imxkobs",
            "sha256sum": "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
          }
	    ]
	  ],
	  "version": "1.2"
	}`
	expectedSignature := []byte("bytes")
	address := "localhost"
	path := "/resource"

	thh := &testHttpHandler{
		Path:            path,
		ResponseBody:    string(expectedBody),
		ResponseHeaders: map[string]string{"UH-Signature": base64.StdEncoding.EncodeToString(expectedSignature)},
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("http://%s:%d", address, port))

	uc := NewUpdateClient()

	fm := &metadata.FirmwareMetadata{
		ProductUID:       "productuid-value",
		DeviceIdentity:   map[string]string{"id1": "id1-value"},
		DeviceAttributes: map[string]string{"attr1": "attr1-value"},
		Hardware:         "",
		Version:          "version-value",
	}

	updateMetadata, signature, extraPoll, err := uc.ProbeUpdate(ac.Request(), path, fm)

	assert.Equal(t, expectedSignature, signature)
	assert.Equal(t, time.Duration(0), extraPoll)
	assert.NoError(t, err)

	um := updateMetadata.(*metadata.UpdateMetadata)

	assert.Equal(t, "0123456789", um.ProductUID)
	assert.Equal(t, "1.2", um.Version)
	assert.Equal(t, []byte(expectedBody), um.RawBytes)
	assert.Equal(t, nil, um.SupportedHardware)
	assert.Equal(t, "application/vnd.updatehub-v1+json", thh.LastRequestHeader.Get("Api-Content-Type"))

	// Objects
	assert.Equal(t, 1, len(um.Objects))
	assert.Equal(t, 1, len(um.Objects[0]))
	assert.Equal(t, "imxkobs", um.Objects[0][0].GetObjectMetadata().Mode)
	assert.Equal(t, "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08", um.Objects[0][0].GetObjectMetadata().Sha256sum)
}

func TestDownloadUpdateWithInvalidApiRequester(t *testing.T) {
	uc := NewUpdateClient()

	body, contentLength, err := uc.DownloadUpdate(nil, "/resource")

	assert.Nil(t, body)
	assert.Equal(t, int64(-1), contentLength)
	assert.EqualError(t, err, "invalid api requester")
}

func TestDownloadUpdateWithNewRequestError(t *testing.T) {
	ac := NewApiClient("http://localhost")

	uc := NewUpdateClient()

	body, contentLength, err := uc.DownloadUpdate(ac.Request(), "/resource%s")

	assert.Nil(t, body)
	assert.Equal(t, int64(-1), contentLength)
	assert.True(t, strings.Contains(err.Error(), "failed to create download update request"))
	assert.True(t, strings.Contains(err.Error(), "invalid URL escape"))
}

func TestDownloadUpdateWithApiDoError(t *testing.T) {
	ac := NewApiClient("http://invalid")

	uc := NewUpdateClient()

	body, contentLength, err := uc.DownloadUpdate(ac.Request(), "/resource")

	assert.Nil(t, body)
	assert.Equal(t, int64(-1), contentLength)
	assert.True(t, strings.Contains(err.Error(), "download update request failed"))
	assert.True(t, strings.Contains(err.Error(), "no such host"))
}

func TestDownloadUpdateWithHTTPError(t *testing.T) {
	expectedBody := []byte("Not found")
	address := "localhost"
	path := "/not-found"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("http://%s:%d", address, port))

	uc := NewUpdateClient()

	body, contentLength, err := uc.DownloadUpdate(ac.Request(), path)

	assert.Nil(t, body)
	assert.Equal(t, int64(-1), contentLength)
	assert.EqualError(t, err, "failed to download update. maybe the file is missing?")
}

func TestDownloadUpdateWithSuccess(t *testing.T) {
	expectedBody := []byte("expected body")
	address := "localhost"
	path := "/resource"

	thh := &testHttpHandler{
		Path:         path,
		ResponseBody: string(expectedBody),
	}

	port, _, err := StartNewTestHttpServer(address, thh)
	assert.NoError(t, err)

	ac := NewApiClient(fmt.Sprintf("http://%s:%d", address, port))

	uc := NewUpdateClient()

	body, contentLength, err := uc.DownloadUpdate(ac.Request(), path)
	defer body.Close()

	assert.Equal(t, int64(len(expectedBody)), contentLength)
	assert.NoError(t, err)

	buffer := make([]byte, contentLength)
	n, err := body.Read(buffer)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, contentLength, int64(n))

	assert.Equal(t, expectedBody, buffer)
	assert.Equal(t, "application/vnd.updatehub-v1+json", thh.LastRequestHeader.Get("Api-Content-Type"))
}

type testHttpHandler struct {
	Path              string
	ResponseBody      string
	ResponseHeaders   map[string]string
	LastRequestHeader http.Header
}

func (thh *testHttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for key, value := range thh.ResponseHeaders {
		w.Header().Add(key, value)
	}

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
		thh.LastRequestHeader = r.Header
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
