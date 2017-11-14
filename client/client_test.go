/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewApiClient(t *testing.T) {
	c := NewApiClient("localhost")
	assert.NotNil(t, c)
	assert.Equal(t, "localhost", c.server)
}

func TestApiClientRequest(t *testing.T) {
	c := NewApiClient("http://localhost")
	assert.NotNil(t, c)

	req := c.Request()
	assert.NotNil(t, req)

	assert.Equal(t, c, req.Client())

	responder := &struct {
		httpStatus int
		headers    http.Header
	}{
		http.StatusOK,
		http.Header{},
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responder.headers = r.Header
		w.WriteHeader(responder.httpStatus)
		w.Header().Set("Content-Type", "application/json")
	}))

	defer s.Close()

	hreq, _ := http.NewRequest(http.MethodGet, s.URL, nil)

	res, err := req.Do(hreq)
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, responder.headers)
	assert.Equal(t, responder.httpStatus, res.StatusCode)
}

func TestServerURL(t *testing.T) {
	c := NewApiClient("http://localhost")

	url := serverURL(c, "/test")

	assert.Equal(t, "http://localhost/test", url)
}

func TestCheckRedirect(t *testing.T) {
	c := NewApiClient("http://localhost")
	assert.NotNil(t, c)

	req := c.Request()
	assert.NotNil(t, req)

	assert.Equal(t, c, req.Client())

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.String() {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/redirect":
			http.Redirect(w, r, "/", http.StatusFound)
		}
	}))

	defer s.Close()

	hreq, _ := http.NewRequest(http.MethodGet, s.URL, nil)

	res, err := req.Do(hreq)
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestCheckRedirectWithHeaders(t *testing.T) {
	c := NewApiClient("http://localhost")
	assert.NotNil(t, c)

	req := c.Request()
	assert.NotNil(t, req)

	assert.Equal(t, c, req.Client())

	var headers http.Header

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.String() {
		case "/":
			headers = r.Header
			w.WriteHeader(http.StatusOK)
		case "/redirect":
			http.Redirect(w, r, "/", http.StatusFound)
		}
	}))

	defer s.Close()

	hreq, _ := http.NewRequest(http.MethodGet, s.URL, nil)
	hreq.Header.Set("Range", "bytes=0-")
	hreq.Header.Set("User-Agent", "updatehub")

	res, err := req.Do(hreq)
	assert.Nil(t, err)
	assert.NotNil(t, res)

	assert.Equal(t, hreq.Header, headers)
	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestCheckRedirectWithMaxRedirectError(t *testing.T) {
	c := NewApiClient("http://localhost")
	assert.NotNil(t, c)

	req := c.Request()
	assert.NotNil(t, req)

	assert.Equal(t, c, req.Client())

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusFound)
	}))

	defer s.Close()

	hreq, _ := http.NewRequest(http.MethodGet, s.URL, nil)

	res, err := req.Do(hreq)
	assert.Error(t, err, ErrMaxRedirect)
	assert.NotNil(t, res)
	assert.Equal(t, http.StatusFound, res.StatusCode)
}
