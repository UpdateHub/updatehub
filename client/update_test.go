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

	"github.com/stretchr/testify/assert"
)

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
	assert.EqualError(t, err, "failed to create fetch update request")
}

func TestFetchUpdateWithApiDoError(t *testing.T) {
	ac := NewApiClient("invalid")

	uc := NewUpdateClient()

	body, contentLength, err := uc.FetchUpdate(ac.Request(), "/resource")

	assert.Nil(t, body)
	assert.Equal(t, int64(-1), contentLength)
	assert.EqualError(t, err, "fetch update request failed")
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
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found")
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
