/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

func TestNewServer(t *testing.T) {
	s := NewServer(nil)
	assert.NotNil(t, s)
}

func TestServerRoutes(t *testing.T) {
	testCases := []struct {
		name   string
		route  string
		method string
	}{
		{
			"Index",
			"/",
			"GET",
		},
	}

	s := NewServer(nil)
	s.ServerController = &testServerController{}

	s.CreateRouter()

	m := httptest.NewServer(s.router)

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

			body := ioutil.NopCloser(r.Body)
			res, err := ioutil.ReadAll(body)
			assert.NoError(t, err)
			assert.Equal(t, []byte(tc.route), res)
		})
	}

}

type testServerController struct {
}

func (s *testServerController) Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprintf(w, "/")
}
