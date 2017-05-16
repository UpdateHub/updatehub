/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewAgentBackend(t *testing.T) {
	logger := logrus.New()

	ab, err := NewAgentBackend(logger)

	assert.NoError(t, err)
	assert.Equal(t, logger, ab.Logger())

	routes := ab.Routes()

	assert.Equal(t, 1, len(routes))
	assert.Equal(t, "GET", routes[0].Method)
	assert.Equal(t, "/", routes[0].Path)

	expectedFunction := reflect.ValueOf(ab.index)
	receivedFunction := reflect.ValueOf(routes[0].Handle)

	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())
}

func TestIndexRoute(t *testing.T) {
	logger := logrus.New()

	ab, err := NewAgentBackend(logger)
	assert.NoError(t, err)

	router := NewBackendRouter(ab)
	server := httptest.NewServer(router.HTTPRouter)

	r, err := http.Get(server.URL + "/")
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Agent backend index"), bodyContent)
}
