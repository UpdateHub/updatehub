/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	ValidJSONMetadata = `{
          "product-uid": "0123456789",
          "supported-hardware": [
            "hardware1-revA",
            "hardware2-revB"
          ],
          "objects": [
            [
              { "mode": "test" }
            ]
          ]
        }`

	InvalidJSONMetadata = `{
          "product-
        }`
)

func TestNewServerBackendWithNonExistantDirectoryError(t *testing.T) {
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	dirPath := path.Join(testPath, "inexistant-dir")

	sb, err := NewServerBackend(dirPath)

	assert.EqualError(t, err, fmt.Sprintf("stat %s: no such file or directory", dirPath))
	assert.Nil(t, sb)
}

func TestNewServerBackendWithNotADirectoryError(t *testing.T) {
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, "updatemetadata.json")

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(updateMetadataFilePath)

	assert.EqualError(t, err, fmt.Sprintf("%s: not a directory", updateMetadataFilePath))
	assert.Nil(t, sb)
}

func TestNewServerBackend(t *testing.T) {
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, "updatemetadata.json")

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(testPath)
	assert.NoError(t, err)

	routes := sb.Routes()

	assert.Equal(t, 3, len(routes))

	assert.Equal(t, "POST", routes[0].Method)
	assert.Equal(t, "/upgrades", routes[0].Path)
	assert.Equal(t, reflect.ValueOf(sb.getUpdateMetadata).Pointer(), reflect.ValueOf(routes[0].Handle).Pointer())

	assert.Equal(t, "POST", routes[1].Method)
	assert.Equal(t, "/report", routes[1].Path)
	assert.Equal(t, reflect.ValueOf(sb.reportStatus).Pointer(), reflect.ValueOf(routes[1].Handle).Pointer())

	assert.Equal(t, "GET", routes[2].Method)
	assert.Equal(t, "/products/:product/packages/:package/objects/:object", routes[2].Path)
	assert.Equal(t, reflect.ValueOf(sb.getObject).Pointer(), reflect.ValueOf(routes[2].Handle).Pointer())
}

func TestParseUpdateMetadataWithStatError(t *testing.T) {
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, "updatemetadata.json")

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(testPath)
	assert.NoError(t, err)

	os.Remove(updateMetadataFilePath)

	err = sb.ParseUpdateMetadata()
	assert.EqualError(t, err, fmt.Sprintf("stat %s: no such file or directory", updateMetadataFilePath))
}

func TestParseUpdateMetadataWithUnmarshalError(t *testing.T) {
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, "updatemetadata.json")

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(InvalidJSONMetadata), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(testPath)
	assert.NoError(t, err)

	err = sb.ParseUpdateMetadata()
	assert.EqualError(t, err, fmt.Sprintf("Invalid update metadata: invalid character '\\n' in string literal"))
}

func TestParseUpdateMetadata(t *testing.T) {
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, "updatemetadata.json")

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	sb, err := NewServerBackend(testPath)
	assert.NoError(t, err)

	err = sb.ParseUpdateMetadata()
	assert.NoError(t, err)
}

func TestUpgradesRoute(t *testing.T) {
	// setup filesystem
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, "updatemetadata.json")

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	// setup server
	sb, err := NewServerBackend(testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	err = sb.ParseUpdateMetadata()
	assert.NoError(t, err)

	// do the request
	r, err := http.Post(server.URL+"/upgrades", "application/json", bytes.NewBuffer([]byte("{\"content\": true}")))
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, r.StatusCode)
	assert.Equal(t, []byte(ValidJSONMetadata), bodyContent)
}

func TestUpgradesRouteWithMetadataNotFound(t *testing.T) {
	// setup filesystem
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	// setup server
	sb, err := NewServerBackend(testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	// do the request
	r, err := http.Post(server.URL+"/upgrades", "application/json", bytes.NewBuffer([]byte("{\"content\": true}")))
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, r.StatusCode)
	assert.Equal(t, []byte("404 page not found\n"), bodyContent)
}

func TestGetObjectRoute(t *testing.T) {
	productUID := "a"
	packageUID := "b"
	object := "c"

	// setup filesystem
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, "updatemetadata.json")

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	pkgdir := path.Join(testPath, productUID, packageUID)
	err = os.MkdirAll(pkgdir, 0777)
	assert.NoError(t, err)

	objdir := path.Join(pkgdir, object)
	err = ioutil.WriteFile(objdir, []byte("object_content"), 0777)
	assert.NoError(t, err)

	// setup server
	sb, err := NewServerBackend(testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	err = sb.ParseUpdateMetadata()
	assert.NoError(t, err)

	// do the request
	finalURL := fmt.Sprintf("%s/products/%s/packages/%s/objects/%s", server.URL, productUID, packageUID, object)
	r, err := http.Get(finalURL)
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, r.StatusCode)
	assert.Equal(t, []byte("object_content"), bodyContent)
}

func TestReportRoute(t *testing.T) {
	// setup filesystem
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, "updatemetadata.json")

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	// setup server
	sb, err := NewServerBackend(testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	err = sb.ParseUpdateMetadata()
	assert.NoError(t, err)

	// do the request
	reportData := []byte("{\"status\": \"downloading\", \"package-uid\": \"puid\"}")
	r, err := http.Post(server.URL+"/report", "application/json", bytes.NewBuffer(reportData))
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, r.StatusCode)
	assert.Equal(t, []byte(""), bodyContent)
}

func TestReportRouteWithInvalidReportData(t *testing.T) {
	// setup filesystem
	testPath, err := ioutil.TempDir("", "server-test")
	assert.NoError(t, err)
	defer os.RemoveAll(testPath)

	updateMetadataFilePath := path.Join(testPath, "updatemetadata.json")

	err = ioutil.WriteFile(updateMetadataFilePath, []byte(ValidJSONMetadata), 0666)
	assert.NoError(t, err)

	// setup server
	sb, err := NewServerBackend(testPath)
	assert.NoError(t, err)

	router := NewBackendRouter(sb)
	server := httptest.NewServer(router.HTTPRouter)

	err = sb.ParseUpdateMetadata()
	assert.NoError(t, err)

	// do the request
	r, err := http.Post(server.URL+"/report", "application/json", bytes.NewBuffer([]byte("{\"content\": ")))
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	assert.Equal(t, []byte("500 internal server error\n"), bodyContent)
}
