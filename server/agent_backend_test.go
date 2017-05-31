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
	"path"
	"reflect"
	"testing"

	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/testsmocks/cmdlinemock"
	"github.com/UpdateHub/updatehub/updatehub"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

const customSettings = `
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

func TestNewAgentBackend(t *testing.T) {
	uh := &updatehub.UpdateHub{}

	ab, err := NewAgentBackend(uh)
	assert.NoError(t, err)

	routes := ab.Routes()

	assert.Equal(t, 1, len(routes))
	assert.Equal(t, "GET", routes[0].Method)
	assert.Equal(t, "/info", routes[0].Path)

	expectedFunction := reflect.ValueOf(ab.info)
	receivedFunction := reflect.ValueOf(routes[0].Handle)

	assert.Equal(t, expectedFunction.Pointer(), receivedFunction.Pointer())
}

func TestInfoRoute(t *testing.T) {
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

	// do the GET and test
	r, err := http.Get(server.URL + "/info")
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

	clm.AssertExpectations(t)
}
