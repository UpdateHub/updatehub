/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

func TestServerRoutes(t *testing.T) {
	testCases := []struct {
		name         string
		route        string
		method       string
		expectedCode int
		expectedBody []byte
	}{
		{
			"IndexGet",
			"/",
			"GET",
			http.StatusOK,
			[]byte("/"),
		},
		{
			"NotFound",
			"/not",
			"GET",
			http.StatusNotFound,
			[]byte("404 page not found\n"),
		},
		{
			"IndexPost",
			"/",
			"POST",
			http.StatusOK,
			[]byte("/"),
		},
	}

	backend := &TestBackend{}
	router := NewBackendRouter(backend)
	m := httptest.NewServer(router.HTTPRouter)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var r *http.Response
			var err error

			if tc.method == "GET" {
				r, err = http.Get(m.URL + tc.route)
				assert.NoError(t, err)
			} else if tc.method == "POST" {
				r, err = http.Post(m.URL+tc.route, "", nil)
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedCode, r.StatusCode)

			body := ioutil.NopCloser(r.Body)
			res, err := ioutil.ReadAll(body)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedBody, res)
		})
	}
}

type TestBackend struct {
}

func (tb *TestBackend) Routes() []Route {
	return []Route{
		{Method: "GET", Path: "/", Handle: tb.indexGet},
		{Method: "POST", Path: "/", Handle: tb.indexPost},
	}
}

func (tb *TestBackend) indexGet(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fmt.Fprintf(w, r.URL.Path)
}

func (tb *TestBackend) indexPost(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	fmt.Fprintf(w, r.URL.Path)
}
