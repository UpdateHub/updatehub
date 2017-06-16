/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/cmdlinemock"
	"github.com/UpdateHub/updatehub/testsmocks/controllermock"
	"github.com/UpdateHub/updatehub/testsmocks/progresstrackermock"
	"github.com/UpdateHub/updatehub/updatehub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	customSettings = `
[Polling]
Interval=1
Enabled=false
LastPoll=2017-01-01T00:00:00Z
FirstPoll=2017-02-02T00:00:00Z
ExtraInterval=4
Retries=5

[Storage]
ReadOnly=true

[Update]
DownloadDir=/tmp/download
AutoDownloadWhenAvailable=false
AutoInstallAfterDownload=false
AutoRebootAfterInstall=false
SupportedInstallModes=mode1,mode2

[Network]
DisableHttps=true
UpdateHubServerAddress=localhost

[Firmware]
MetadataPath=/tmp/metadata
`
	validUpdateMetadataWithActiveInactive = `{
  "product-uid": "123",
  "objects": [
    [
      { "mode": "test", "target": "/dev/xxa1", "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" },
      { "mode": "test", "target": "/dev/xxa2", "sha256sum": "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb" }
    ]
    ,
    [
      { "mode": "test", "target": "/dev/xxb1", "sha256sum": "d0b425e00e15a0d36b9b361f02bab63563aed6cb4665083905386c55d5b679fa" },
      { "mode": "test", "target": "/dev/xxb2", "sha256sum": "b9632efa90820ff35d6cec0946f99bb8a6317b1e2ef877e501a3e12b2d04d0ae" }
    ]
  ]
}`
)

func TestNewAgentBackend(t *testing.T) {
	uh := &updatehub.UpdateHub{}

	ab, err := NewAgentBackend(uh)
	assert.NoError(t, err)

	routes := ab.Routes()

	assert.Equal(t, 5, len(routes))

	assert.Equal(t, "GET", routes[0].Method)
	assert.Equal(t, "/info", routes[0].Path)
	expectedFunction := reflect.ValueOf(ab.info)
	receivedFunction := reflect.ValueOf(routes[0].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "GET", routes[1].Method)
	assert.Equal(t, "/status", routes[1].Path)
	expectedFunction = reflect.ValueOf(ab.status)
	receivedFunction = reflect.ValueOf(routes[1].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "POST", routes[2].Method)
	assert.Equal(t, "/update", routes[2].Path)
	expectedFunction = reflect.ValueOf(ab.update)
	receivedFunction = reflect.ValueOf(routes[2].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "GET", routes[3].Method)
	assert.Equal(t, "/update/metadata", routes[3].Path)
	expectedFunction = reflect.ValueOf(ab.updateMetadata)
	receivedFunction = reflect.ValueOf(routes[3].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())

	assert.Equal(t, "POST", routes[4].Method)
	assert.Equal(t, "/update/probe", routes[4].Path)
	expectedFunction = reflect.ValueOf(ab.updateProbe)
	receivedFunction = reflect.ValueOf(routes[4].Handle)
	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())
}

func setup(t *testing.T) (*updatehub.UpdateHub, string, *cmdlinemock.CmdLineExecuterMock) {
	const (
		metadataPath        = "/"
		systemSettingsPath  = "/system.conf"
		runtimeSettingsPath = "/runtime.conf"
	)

	// setup mem map filesystem
	fs := afero.NewMemMapFs()

	files := map[string]string{
		"/device-identity.d/key1":    "id1=value1",
		"/device-identity.d/key2":    "id2=value2",
		"/device-attributes.d/attr1": "attr1=value",
		"/device-attributes.d/attr2": "attr2=value",
	}

	for k, v := range files {
		err := afero.WriteFile(fs, k, []byte(v), 0700)
		assert.NoError(t, err)
	}

	err := afero.WriteFile(fs, systemSettingsPath, []byte(customSettings), 0644)
	assert.NoError(t, err)

	// setup mock
	productUID := "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381"
	hardware := "board"
	hardwareRevision := "revA"
	firmwareVersion := "1.1"
	agentVersion := "0.6.90-7-ga456673"
	buildTime := "2017-06-01 17:24 UTC"

	clm := &cmdlinemock.CmdLineExecuterMock{}

	clm.On("Execute", path.Join(metadataPath, "product-uid")).Return([]byte(productUID), nil)
	clm.On("Execute", path.Join(metadataPath, "hardware")).Return([]byte(hardware), nil)
	clm.On("Execute", path.Join(metadataPath, "hardware-revision")).Return([]byte(hardwareRevision), nil)
	clm.On("Execute", path.Join(metadataPath, "version")).Return([]byte(firmwareVersion), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key1")).Return([]byte("id1=value1"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-identity.d/key2")).Return([]byte("id2=value2"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr1")).Return([]byte("attr1=value1"), nil)
	clm.On("Execute", path.Join(metadataPath, "/device-attributes.d/attr2")).Return([]byte("attr2=value2"), nil)

	// create objects
	fm, err := metadata.NewFirmwareMetadata(metadataPath, fs, clm)
	assert.NoError(t, err)

	uh := &updatehub.UpdateHub{
		FirmwareMetadata:    *fm,
		SystemSettingsPath:  systemSettingsPath,
		RuntimeSettingsPath: runtimeSettingsPath,
		Store:               fs,
		Version:             agentVersion,
		BuildTime:           buildTime,
	}

	err = uh.LoadSettings()
	assert.NoError(t, err)

	ab, err := NewAgentBackend(uh)
	assert.NoError(t, err)

	router := NewBackendRouter(ab)
	server := httptest.NewServer(router.HTTPRouter)

	return uh, server.URL, clm
}

func teardown(t *testing.T) {
}

func TestInfoRoute(t *testing.T) {
	uh, url, clm := setup(t)
	defer teardown(t)

	r, err := http.Get(url + "/info")
	assert.NoError(t, err)

	jsonMap := map[string]interface{}{}

	jsonMap["version"] = uh.Version
	jsonMap["build-time"] = uh.BuildTime
	jsonMap["config"] = uh.Settings
	jsonMap["firmware"] = uh.FirmwareMetadata

	expectedJSON, err := json.MarshalIndent(jsonMap, "", "    ")
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)
	assert.Equal(t, string(expectedJSON), string(bodyContent))
	assert.Equal(t, 200, r.StatusCode)

	clm.AssertExpectations(t)
}

func TestStatusRoute(t *testing.T) {
	uh, url, clm := setup(t)
	defer teardown(t)

	ptm := &progresstrackermock.ProgressTrackerMock{}
	ptm.On("GetProgress").Return(25).Once()

	uh.State = updatehub.NewDownloadingState(&metadata.UpdateMetadata{}, ptm)

	r, err := http.Get(url + "/status")
	assert.NoError(t, err)

	jsonMap := map[string]interface{}{}

	jsonMap["status"] = "downloading"
	jsonMap["progress"] = 25

	expectedJSON, err := json.MarshalIndent(jsonMap, "", "    ")
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)
	assert.Equal(t, string(expectedJSON), string(bodyContent))
	assert.Equal(t, 200, r.StatusCode)

	clm.AssertExpectations(t)
	ptm.AssertExpectations(t)
}

func TestUpdateRoute(t *testing.T) {
	uh, url, clm := setup(t)
	defer teardown(t)

	cm := &controllermock.ControllerMock{}

	uh.Controller = cm
	cm.On("CheckUpdate", 0).Run(func(args mock.Arguments) {
		// this is on purpose, the response is supposed to be
		// asynchronous and we should get the HTTP response before the
		// sleep expires
		time.Sleep(100 * time.Second)
		os.Exit(1)
	}).Return(&metadata.UpdateMetadata{}, 10*time.Second)

	r, err := http.Post(url+"/update", "application/json", nil)
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)
	assert.Equal(t, string(`{ "message": "request accepted, update procedure fired" }`), string(bodyContent))
	assert.Equal(t, 202, r.StatusCode)

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
}

func TestUpdateMetadataRoute(t *testing.T) {
	uh, url, clm := setup(t)
	defer teardown(t)

	err := afero.WriteFile(uh.Store, "/tmp/download/updatemetadata.json", []byte(validUpdateMetadataWithActiveInactive), 0644)
	assert.NoError(t, err)

	r, err := http.Get(url + "/update/metadata")
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)
	assert.Equal(t, validUpdateMetadataWithActiveInactive, string(bodyContent))
	assert.Equal(t, 200, r.StatusCode)

	clm.AssertExpectations(t)
}

func TestUpdateMetadataRouteWithError(t *testing.T) {
	out := map[string]interface{}{}
	out["error"] = "open /tmp/download/updatemetadata.json: file does not exist"

	expectedResponse, _ := json.MarshalIndent(out, "", "    ")

	_, url, clm := setup(t)
	defer teardown(t)

	r, err := http.Get(url + "/update/metadata")
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)
	assert.Equal(t, string(expectedResponse), string(bodyContent))
	assert.Equal(t, 400, r.StatusCode)

	clm.AssertExpectations(t)
}

func TestUpdateProbeRoute(t *testing.T) {
	out := map[string]interface{}{}
	out["update-available"] = false
	out["try-again-in"] = 3600

	expectedResponse, _ := json.MarshalIndent(out, "", "    ")

	uh, url, clm := setup(t)
	defer teardown(t)

	cm := &controllermock.ControllerMock{}

	uh.Controller = cm
	cm.On("CheckUpdate", 0).Return((*metadata.UpdateMetadata)(nil), 3600*time.Second)

	r, err := http.Post(url+"/update/probe", "application/json", nil)
	assert.NoError(t, err)

	body := ioutil.NopCloser(r.Body)
	bodyContent, err := ioutil.ReadAll(body)
	assert.NoError(t, err)
	assert.Equal(t, string(expectedResponse), string(bodyContent))
	assert.Equal(t, 200, r.StatusCode)

	clm.AssertExpectations(t)
	cm.AssertExpectations(t)
}
